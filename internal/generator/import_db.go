package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aspgen/internal/dbschema"
)

// dbImportRequest carries the flags shared by `new --script` and the
// `import-db` verb.
type dbImportRequest struct {
	Provider string
	Script   string
	Tables   []string
	// Context is the bounded context every imported table is added to as
	// an aggregate; only used (and required) when the target project is
	// the blazor/Renoir profile.
	Context string
}

// parseDBImportFlags reads the DB-import flags out of args. ok is false when
// --script is not present (DB import not requested).
func parseDBImportFlags(args []string) (req dbImportRequest, ok bool, err error) {
	script, err := scriptOption(args)
	if err != nil {
		return dbImportRequest{}, false, err
	}
	if script == "" {
		return dbImportRequest{}, false, nil
	}
	provider, err := providerOption(args)
	if err != nil {
		return dbImportRequest{}, false, err
	}
	if provider == "" {
		return dbImportRequest{}, false, errors.New("--provider is required with --script")
	}
	tables, err := tablesOption(args)
	if err != nil {
		return dbImportRequest{}, false, err
	}
	context := value(args, "--context", "")
	return dbImportRequest{Provider: provider, Script: script, Tables: tables, Context: context}, true, nil
}

// runDBImport discovers table/column schema from the SQL script and, for
// each selected table, synthesizes `col:type` property args and
// calls addEntityCmd — the same call `add entity` itself makes — so every
// backend profile addEntityCmd already supports works unmodified. It also
// writes a `schema.sql` backup snapshot at the project root.
func runDBImport(project string, m *Manifest, backend string, req dbImportRequest, dryRun, force bool) error {
	if isRenoir(*m) {
		return runDBImportRenoir(project, m, req, dryRun, force)
	}
	if backend == "" {
		return errors.New("DB-driven entity import requires --simple or --backend ddd on the target project")
	}
	tables, err := discoverTables(req)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return errors.New("no tables selected for import")
	}
	for _, table := range tables {
		propArgs, skipped := synthesizeProps(table, req.Provider, backend)
		for _, name := range skipped {
			fmt.Fprintf(os.Stderr, "import-db: skipping column %q in table %q (unsupported type or reserved name)\n", name, table.Name)
		}
		if len(propArgs) == 0 {
			fmt.Fprintf(os.Stderr, "import-db: skipping table %q (no usable columns)\n", table.Name)
			continue
		}
		entityName := singularize(collapsePascal(table.Name))
		r := addRequest{Name: entityName, Args: propArgs, Project: project, DryRun: dryRun, Force: force, Backend: backend}
		d := &data{
			Project:   m.Project,
			Namespace: m.Project,
			Name:      entityName,
			Database:  componentDatabase(m.Components),
			Seed:      componentSeed(m.Components),
			SeedCount: componentSeedCount(m.Components),
			Backend:   backend,
		}
		if backend == "ddd" && !isWebAPI(*m) {
			d.Backend = "ddd-local"
		}
		if err := addEntityCmd(r, m, d); err != nil {
			return fmt.Errorf("table %q: %w", table.Name, err)
		}
	}
	if !dryRun {
		if err := os.WriteFile(filepath.Join(project, "schema.sql"), []byte(dbschema.RenderSchemaSQL(tables)), 0o644); err != nil {
			return err
		}
	}
	m.Components = appendUnique(m.Components, "db-import:"+req.Provider)
	return nil
}

// runDBImportRenoir mirrors runDBImport for the blazor/Renoir profile: every
// selected table becomes a CRUD-default aggregate (via addAggregateCmd, the
// same call `add aggregate` itself makes) in req.Context, which must already
// exist in the manifest.
func runDBImportRenoir(project string, m *Manifest, req dbImportRequest, dryRun, force bool) error {
	if req.Context == "" {
		return errors.New("DB-driven aggregate import on the blazor/Renoir profile requires --context ContextName")
	}
	if !validIdentifier(req.Context) {
		return fmt.Errorf("invalid context name %q", req.Context)
	}
	if !contextExists(m.Contexts, req.Context) {
		m.Contexts = appendContext(m.Contexts, Context{Name: req.Context})
		m.Components = appendUnique(m.Components, "context:"+req.Context)
	}
	tables, err := discoverTables(req)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return errors.New("no tables selected for import")
	}
	for _, table := range tables {
		propArgs, skipped := synthesizeProps(table, req.Provider, "ddd")
		for _, name := range skipped {
			fmt.Fprintf(os.Stderr, "import-db: skipping column %q in table %q (unsupported type or reserved name)\n", name, table.Name)
		}
		if len(propArgs) == 0 {
			fmt.Fprintf(os.Stderr, "import-db: skipping table %q (no usable columns)\n", table.Name)
			continue
		}
		aggregateName := singularize(collapsePascal(table.Name))
		r := addRequest{Name: aggregateName, Args: append(append([]string{}, propArgs...), "--context", req.Context), Project: project, DryRun: dryRun, Force: force}
		d := &data{Project: m.Project, Namespace: m.Project}
		if err := addAggregateCmd(r, m, d); err != nil {
			return fmt.Errorf("table %q: %w", table.Name, err)
		}
	}
	if !dryRun {
		if err := os.WriteFile(filepath.Join(project, "schema.sql"), []byte(dbschema.RenderSchemaSQL(tables)), 0o644); err != nil {
			return err
		}
	}
	m.Components = appendUnique(m.Components, "db-import:"+req.Provider)
	return nil
}

func discoverTables(req dbImportRequest) ([]dbschema.Table, error) {
	script, err := os.ReadFile(req.Script)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", req.Script, err)
	}
	tables, err := dbschema.ParseScript(req.Provider, string(script))
	if err != nil {
		return nil, err
	}
	return filterParsedTables(tables, req.Tables)
}

// filterParsedTables applies a --tables allow-list to script-parsed tables.
func filterParsedTables(tables []dbschema.Table, wanted []string) ([]dbschema.Table, error) {
	if len(wanted) == 0 {
		return tables, nil
	}
	byName := map[string]dbschema.Table{}
	for _, t := range tables {
		byName[t.Name] = t
	}
	result := make([]dbschema.Table, 0, len(wanted))
	for _, name := range wanted {
		t, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("table %q not found in script", name)
		}
		result = append(result, t)
	}
	return result, nil
}

// synthesizeProps builds `col:type` property args for addEntityCmd, skipping
// primary-key columns (every entity template already emits Id), conventional
// audit columns on the ddd backend (AuditableEntity already provides
// CreatedOn/UpdatedOn), and columns whose raw type has no canonical mapping.
func synthesizeProps(table dbschema.Table, provider, backend string) (args, skipped []string) {
	for _, col := range table.Columns {
		if col.IsPrimaryKey || isReservedColumn(col.Name, backend) {
			continue
		}
		alias, ok := dbschema.MapColumnType(provider, col.RawType)
		if !ok {
			skipped = append(skipped, col.Name)
			continue
		}
		if col.Nullable {
			alias += "?"
		}
		args = append(args, col.Name+":"+alias)
	}
	return args, skipped
}

func isReservedColumn(name, backend string) bool {
	lower := strings.ToLower(name)
	if lower == "id" {
		return true
	}
	if backend != "ddd" {
		return false
	}
	switch lower {
	case "created_on", "createdon", "created_at", "createdat",
		"updated_on", "updatedon", "updated_at", "updatedat":
		return true
	}
	return false
}

// collapsePascal turns a table name (Customers, customer_orders, TBL_Orders)
// into a PascalCase identifier, reusing humanize's word-boundary splitting.
func collapsePascal(name string) string {
	return strings.ReplaceAll(humanize(name), " ", "")
}

// singularize is a best-effort table-name-to-entity-name heuristic; it will
// mis-singularize words that already end in "s" but aren't plural (e.g.
// "Status", "Series") — a documented v1 limitation with no per-table
// override yet.
func singularize(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, "ies"):
		return name[:len(name)-3] + "y"
	case strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"),
		strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "ses"):
		return name[:len(name)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss"):
		return name[:len(name)-1]
	default:
		return name
	}
}
