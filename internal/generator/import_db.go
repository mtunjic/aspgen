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
	// Context is the bounded context every imported table becomes an
	// ar-tier entity in.
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
// each selected table, synthesizes property args and calls addEntityCmd —
// the same call `add entity` itself makes — to create an ar-tier entity in
// req.Context. Foreign-key columns become `nav:Target` relations (tables are
// imported referenced-first so relations resolve). It also writes a
// `schema.sql` backup snapshot at the project root.
func runDBImport(project string, m *Manifest, backend string, req dbImportRequest, dryRun, force bool) error {
	if backend == "" {
		return errors.New("DB-driven entity import requires an ar-tier context project")
	}
	if req.Context == "" {
		return errors.New("DB-driven entity import requires --context ContextName")
	}
	tables, err := discoverTables(req)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return errors.New("no tables selected for import")
	}
	// Import referenced tables before the tables that reference them, so
	// foreign-key relations resolve (the relation target must already exist).
	tables = orderTablesByDependency(tables)
	inSet := map[string]bool{}
	for _, t := range tables {
		inSet[strings.ToLower(t.Name)] = true
	}
	for _, table := range tables {
		propArgs, skipped := synthesizeProps(table, req.Provider, backend, inSet)
		for _, name := range skipped {
			fmt.Fprintf(os.Stderr, "import-db: skipping column %q in table %q (unsupported type or reserved name)\n", name, table.Name)
		}
		if len(propArgs) == 0 {
			fmt.Fprintf(os.Stderr, "import-db: skipping table %q (no usable columns)\n", table.Name)
			continue
		}
		entityName := singularize(collapsePascal(table.Name))
		r := addRequest{Name: entityName, Args: append(append([]string{}, propArgs...), "--context", req.Context), Project: project, DryRun: dryRun, Force: force, Backend: backend}
		d := &data{
			Project:   m.Project,
			Namespace: m.Project,
			Name:      entityName,
			Database:  componentDatabase(m.Components),
			Backend:   backend,
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
//
// Foreign-key columns become `nav:Target` relation args instead of scalar
// columns — but only when the referenced table is part of the import set
// (otherwise the relation cannot resolve and the column falls back to a
// scalar). The FK column itself is dropped; the generator synthesizes the
// `{Nav}Id` property.
func synthesizeProps(table dbschema.Table, provider, backend string, inSet map[string]bool) (args, skipped []string) {
	for _, col := range table.Columns {
		if col.ForeignKey != "" && inSet[strings.ToLower(col.ForeignKey)] {
			nav := relationNavName(col.Name)
			target := singularize(collapsePascal(col.ForeignKey))
			if col.Nullable {
				target += "?"
			}
			args = append(args, nav+":"+target)
			continue
		}
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

// relationNavName derives the navigation-property name from an FK column
// name: `customer_id` and `customerId` both become `customer` (the generator
// appends Id to form the FK property `CustomerId`).
func relationNavName(col string) string {
	name := col
	if strings.HasSuffix(strings.ToLower(name), "_id") {
		name = name[:len(name)-3]
	} else if strings.HasSuffix(name, "Id") && len(name) > 2 {
		name = name[:len(name)-2]
	}
	parts := strings.Split(name, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// orderTablesByDependency returns tables with referenced tables placed before
// their referencing tables, so entities can be created in FK order. Cyclic
// or self-referencing FK graphs keep a stable (declaration) relative order.
func orderTablesByDependency(tables []dbschema.Table) []dbschema.Table {
	byName := map[string]dbschema.Table{}
	for _, t := range tables {
		byName[strings.ToLower(t.Name)] = t
	}
	var result []dbschema.Table
	added := map[string]bool{}
	var visit func(name string, stack map[string]bool)
	visit = func(name string, stack map[string]bool) {
		key := strings.ToLower(name)
		t, ok := byName[key]
		if !ok || added[key] {
			return
		}
		if stack[key] {
			return // cycle guard
		}
		stack[key] = true
		for _, c := range t.Columns {
			if c.ForeignKey != "" {
				visit(c.ForeignKey, stack)
			}
		}
		delete(stack, key)
		added[key] = true
		result = append(result, t)
	}
	for _, t := range tables {
		visit(t.Name, map[string]bool{})
	}
	return result
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
