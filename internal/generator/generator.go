package generator

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"aspgen/internal/templates"
)

type Manifest struct {
	Project    string    `json:"project"`
	Components []string  `json:"components"`
	Contexts   []Context `json:"contexts,omitempty"`
}

type Context struct {
	Name       string   `json:"name"`
	Aggregates []string `json:"aggregates,omitempty"`
}

type Property struct {
	Name        string
	DisplayName string
	CSharpType  string
	UIControl   string
}

type data struct {
	Project    string
	Namespace  string
	Name       string
	Properties []Property
	Context    string
	Aggregate  string
	Crud       bool
	Theme      string
	Backend    string
	Database   string
	Seed       string
	SeedCount  int
}

func Run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "new":
		return newProject(args[1:])
	case "add":
		return add(args[1:])
	case "templates":
		return templateCommand(args[1:])
	case "version":
		fmt.Println("aspgen dev")
		return nil
	default:
		return usage()
	}
}

func usage() error {
	return errors.New("usage: aspgen new NAME --app webapi|wpf|blazor|fullstack [--simple] [--backend ddd] [--database sqlite|postgres] [--seed dummy] [--theme wpfui] | aspgen add context|aggregate|value-object|domain-service|repository|event|entity|module|database|service ...")
}

func newProject(args []string) error {
	if len(args) == 0 {
		return errors.New("project name is required")
	}
	name := args[0]
	app := value(args[1:], "--app", "webapi")
	out := value(args[1:], "--output", name)
	theme, err := themeOption(args[1:])
	if err != nil {
		return err
	}
	backend, err := backendOption(args[1:])
	if err != nil {
		return err
	}
	database, err := databaseOption(args[1:])
	if err != nil {
		return err
	}
	simple := hasFlag(args, "--simple")
	seed, seedCount, err := seedOption(args[1:])
	if err != nil {
		return err
	}
	if !validProjectName(name) {
		return fmt.Errorf("invalid project name %q", name)
	}
	if app != "webapi" && app != "wpf" && app != "blazor" && app != "fullstack" {
		return fmt.Errorf("unsupported app target %q", app)
	}
	if theme != "" && app != "wpf" && app != "fullstack" {
		return errors.New("--theme requires a wpf or fullstack application")
	}
	if backend != "" && app != "webapi" && app != "fullstack" && !(app == "wpf" && backend == "ddd") {
		return errors.New("--backend ddd requires a webapi, fullstack, or local DDD wpf application")
	}
	if database != "" && app != "webapi" && app != "fullstack" && !(app == "wpf" && backend == "ddd") {
		return errors.New("--database requires a webapi, fullstack, or local DDD wpf application")
	}
	if simple && backend != "" {
		return errors.New("--simple and --backend cannot be used together")
	}
	if simple && app != "webapi" && app != "fullstack" {
		return errors.New("--simple requires a webapi or fullstack application")
	}
	if seed != "" && app != "webapi" && app != "fullstack" && !(app == "wpf" && backend == "ddd") {
		return errors.New("--seed requires a webapi, fullstack, or local DDD wpf application")
	}
	if seed != "" && !simple && backend == "" {
		return errors.New("--seed requires --simple or --backend ddd")
	}
	if database == "" {
		database = "sqlite"
	}
	if exists(filepath.Join(out, ".aspgen", "manifest.json")) {
		return fmt.Errorf("%s is already an aspgen project", out)
	}
	manifest := Manifest{Project: name}
	dryRun := hasFlag(args, "--dry-run")
	force := hasFlag(args, "--force")
	if app == "webapi" || app == "fullstack" {
		manifest.Components = append(manifest.Components, "webapi")
		if simple {
			manifest.Components = append(manifest.Components, "backend:simple")
		}
		if backend != "" {
			manifest.Components = append(manifest.Components, "backend:"+backend)
		}
		if seed != "" {
			manifest.Components = append(manifest.Components, fmt.Sprintf("seed:%s:%d", seed, seedCount))
		}
		manifest.Components = append(manifest.Components, "database:"+database)
		group := "webapi"
		if simple {
			group = "simple-webapi"
		}
		if err := renderTree(out, group, data{Project: name, Namespace: name, Backend: "simple", Database: database}, templateDir(args), dryRun, force); err != nil {
			return err
		}
		seedGroup := "seed-webapi"
		if simple {
			seedGroup = "seed-simple-webapi"
		}
		if seed != "" {
			if err := renderTree(out, seedGroup, data{Project: name, Namespace: name, Seed: seed, SeedCount: seedCount}, templateDir(args), dryRun, force); err != nil {
				return err
			}
			seedBackend := "ddd"
			if simple {
				seedBackend = "simple"
			}
			if err := updateSeedHost(out, name, seedBackend, dryRun); err != nil {
				return err
			}
		}
	}
	if app == "wpf" && backend == "ddd" {
		manifest.Components = append(manifest.Components, "backend:ddd", "database:"+database)
		if seed != "" {
			manifest.Components = append(manifest.Components, fmt.Sprintf("seed:%s:%d", seed, seedCount))
		}
		if err := renderTree(out, "wpf-ddd", data{Project: name, Namespace: name, Backend: "ddd-local", Database: database, Seed: seed, SeedCount: seedCount}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	}
	if app == "wpf" || app == "fullstack" {
		manifest.Components = append(manifest.Components, "wpf", "prism-dryioc")
		if theme != "" {
			manifest.Components = append(manifest.Components, "theme:"+theme)
		}
		if err := renderTree(out, "wpf", data{Project: name, Namespace: name, Theme: theme, Backend: backend, Seed: seed, SeedCount: seedCount}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	}
	if app == "blazor" {
		manifest.Components = append(manifest.Components, "renoir")
		if err := renderTree(out, "renoir", data{Project: name, Namespace: name}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	}
	if dryRun {
		fmt.Println("would create .aspgen/manifest.json")
		fmt.Println("would create", filepath.Join(out, name+".sln"))
		return nil
	}
	if err := saveManifest(out, manifest); err != nil {
		return err
	}
	if err := normalizeProjectFiles(out, name); err != nil {
		return err
	}
	return writeSolution(out, name, app, force, simple, backend)
}

func add(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: aspgen add context|aggregate|value-object|domain-service|repository|event|feature|entity|module|database|service NAME --project PATH [--theme wpfui]")
	}
	component, name := args[0], args[1]
	project := value(args[2:], "--project", "")
	if project == "" {
		var err error
		project, err = discoverProjectRoot(".")
		if err != nil {
			return err
		}
	}
	dryRun := hasFlag(args, "--dry-run")
	force := hasFlag(args, "--force")
	m, err := loadManifest(project)
	if err != nil {
		return err
	}
	d := data{Project: m.Project, Namespace: m.Project, Name: name, Database: componentDatabase(m.Components), Seed: componentSeed(m.Components), SeedCount: componentSeedCount(m.Components)}
	theme, err := themeOption(args[2:])
	if err != nil {
		return err
	}
	if theme == "" {
		theme = componentTheme(m.Components)
	}
	d.Theme = theme
	backend, err := backendOption(args[2:])
	if err != nil {
		return err
	}
	if backend == "" {
		backend = componentBackend(m.Components)
	}
	if requestedBackend, err := backendOption(args[2:]); err == nil && requestedBackend != "" {
		projectBackend := componentBackend(m.Components)
		if projectBackend != "" && projectBackend != requestedBackend {
			return fmt.Errorf("backend %q conflicts with project backend %q", requestedBackend, projectBackend)
		}
	}
	d.Backend = backend
	if backend == "ddd" && !isWebAPI(m) {
		d.Backend = "ddd-local"
	}
	switch component {
	case "context":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid context name %q", name)
		}
		m.Contexts = appendContext(m.Contexts, Context{Name: name})
		m.Components = appendUnique(m.Components, "context:"+name)
	case "aggregate":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid aggregate name %q", name)
		}
		contextName := value(args[2:], "--context", "")
		if contextName == "" || !validIdentifier(contextName) {
			return errors.New("aggregate requires --context ContextName")
		}
		if !contextExists(m.Contexts, contextName) {
			return fmt.Errorf("bounded context %q does not exist; add it first", contextName)
		}
		props, err := parseProperties(args[2:])
		if err != nil {
			return err
		}
		if err := rejectAggregateReservedProperties(props); err != nil {
			return err
		}
		d.Context, d.Aggregate, d.Properties, d.Crud = contextName, name, props, !hasFlag(args, "--no-crud")
		if isRenoir(m) {
			if err := renderTree(project, "renoir-aggregate", d, templateDir(args), dryRun, force); err != nil {
				return err
			}
			if d.Crud {
				if err := renderTree(project, "renoir-crud", d, templateDir(args), dryRun, force); err != nil {
					return err
				}
			}
		} else {
			return errors.New("DDD aggregate generation currently requires the blazor/Renoir profile")
		}
		m.Contexts = addAggregate(m.Contexts, contextName, name)
		m.Components = appendUnique(m.Components, "aggregate:"+contextName+":"+name)
	case "value-object":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid value object name %q", name)
		}
		contextName := value(args[2:], "--context", "")
		if contextName == "" || !contextExists(m.Contexts, contextName) {
			return errors.New("value-object requires an existing --context ContextName")
		}
		props, err := parseProperties(args[2:])
		if err != nil {
			return err
		}
		d.Context, d.Properties = contextName, props
		if !isRenoir(m) {
			return errors.New("DDD value-object generation currently requires the blazor/Renoir profile")
		}
		if err := renderTree(project, "renoir-value-object", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "value-object:"+contextName+":"+name)
	case "domain-service":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid domain service name %q", name)
		}
		contextName := value(args[2:], "--context", "")
		if contextName == "" || !contextExists(m.Contexts, contextName) {
			return errors.New("domain-service requires an existing --context ContextName")
		}
		d.Context = contextName
		if !isRenoir(m) {
			return errors.New("DDD domain-service generation currently requires the blazor/Renoir profile")
		}
		if err := renderTree(project, "renoir-domain-service", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "domain-service:"+contextName+":"+name)
	case "repository":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid repository name %q", name)
		}
		contextName := value(args[2:], "--context", "")
		aggregateName := value(args[2:], "--aggregate", "")
		if contextName == "" || !contextExists(m.Contexts, contextName) || aggregateName == "" || !validIdentifier(aggregateName) {
			return errors.New("repository requires an existing --context ContextName and --aggregate AggregateName")
		}
		d.Context, d.Aggregate = contextName, aggregateName
		if !isRenoir(m) {
			return errors.New("DDD repository generation currently requires the blazor/Renoir profile")
		}
		if err := renderTree(project, "renoir-repository", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "repository:"+contextName+":"+aggregateName)
	case "event":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid domain event name %q", name)
		}
		contextName := value(args[2:], "--context", "")
		if contextName == "" || !contextExists(m.Contexts, contextName) {
			return errors.New("event requires an existing --context ContextName")
		}
		props, err := parseProperties(args[2:])
		if err != nil {
			return err
		}
		d.Context, d.Properties = contextName, props
		if !isRenoir(m) {
			return errors.New("DDD event generation currently requires the blazor/Renoir profile")
		}
		if err := renderTree(project, "renoir-event", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "event:"+contextName+":"+name)
	case "feature":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid feature name %q", name)
		}
		if !isWebAPI(m) || hasComponent(m.Components, "backend:simple") {
			return errors.New("feature generation requires a non-simple webapi or fullstack project")
		}
		props, err := parseProperties(args[2:])
		if err != nil {
			return err
		}
		if !isWebAPI(m) {
			return errors.New("feature generation requires a webapi or fullstack project")
		}
		d.Properties = props
		if err := renderTree(project, "webapi-feature", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		if err := updateFeatureHost(project, m.Project, name, dryRun); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "feature:"+name)
	case "ui":
		framework := value(args[2:], "--framework", "wpf")
		if framework != "wpf" {
			return fmt.Errorf("unsupported UI framework %q", framework)
		}
		if !isWebAPI(m) && !hasComponent(m.Components, "wpf") {
			return errors.New("adding WPF UI requires a webapi or fullstack project")
		}
		if theme == "" {
			theme = componentTheme(m.Components)
		}
		d.Theme = theme
		if err := renderTree(project, "wpf", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "wpf")
		m.Components = appendUnique(m.Components, "prism-dryioc")
		if theme != "" {
			m.Components = appendUnique(m.Components, "theme:"+theme)
		}
		if isWebAPI(m) {
			if err := writeSolution(project, m.Project, "fullstack", true, hasComponent(m.Components, "backend:simple"), componentBackend(m.Components)); err != nil {
				return err
			}
		}
	case "entity":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid entity name %q", name)
		}
		props, err := parseProperties(args[2:])
		if err != nil {
			return err
		}
		d.Properties = props
		if (isWebAPI(m) && !hasComponent(m.Components, "backend:simple")) || (backend == "ddd" && hasComponent(m.Components, "wpf")) {
			if err := renderTree(project, "entity", d, templateDir(args), dryRun, force); err != nil {
				return err
			}
		}
		if hasComponent(m.Components, "wpf") {
			d.Namespace = m.Project + ".Desktop.Modules." + name
			if err := renderTree(project, "wpf-entity", d, templateDir(args), dryRun, force); err != nil {
				return err
			}
			if err := updateModuleCatalog(project, d.Namespace, name, dryRun); err != nil {
				return err
			}
		}
		if backend != "" {
			if !isWebAPI(m) && !(backend == "ddd" && hasComponent(m.Components, "wpf")) {
				return errors.New("a backend profile requires a webapi, fullstack, or local DDD wpf project")
			}
			if backend == "simple" {
				if err := renderTree(project, "simple-entity", data{Project: m.Project, Namespace: m.Project, Name: name, Properties: props, Backend: backend}, templateDir(args), dryRun, force); err != nil {
					return err
				}
				if err := updateSimpleDbContext(project, m.Project, name, dryRun); err != nil {
					return err
				}
			} else if backend == "ddd" {
				if isWebAPI(m) {
					if err := renderTree(project, "webapi-ddd-entity", data{Project: m.Project, Namespace: m.Project, Name: name, Properties: props, Backend: backend}, templateDir(args), dryRun, force); err != nil {
						return err
					}
					if err := updateEntityDependencyInjection(project, m.Project, name, dryRun); err != nil {
						return err
					}
					if err := updateEntityDbContext(project, m.Project, name, dryRun); err != nil {
						return err
					}
				} else if err := updateEntityDbContextLocal(project, m.Project, name, props, dryRun); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unsupported backend %q", backend)
			}
			if isWebAPI(m) {
				if err := updateFeatureHost(project, m.Project, name, dryRun); err != nil {
					return err
				}
			}
			m.Components = appendUnique(m.Components, "backend:"+backend)
		}
		if componentSeed(m.Components) == "dummy" {
			if backend == "" {
				return errors.New("dummy seeding requires a simple or DDD backend")
			}
			seedBackend := backend
			if backend == "ddd" && !isWebAPI(m) {
				seedBackend = "ddd-local"
			}
			if err := updateSeedFile(project, m.Project, seedBackend, name, props, componentSeedCount(m.Components), dryRun); err != nil {
				return err
			}
		}
		m.Components = appendUnique(m.Components, "entity:"+name)
	case "module":
		if !validIdentifier(name) {
			return fmt.Errorf("invalid module name %q", name)
		}
		if !hasComponent(m.Components, "wpf") {
			return errors.New("module generation requires a WPF UI; add ui first")
		}
		d.Namespace = m.Project + ".Modules." + name
		if err := renderTree(project, "module", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		if err := updateModuleCatalog(project, d.Namespace, name, dryRun); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "module:"+name)
	case "database":
		if !isWebAPI(m) {
			return errors.New("database generation requires a webapi or fullstack project")
		}
		if isWebAPI(m) && (exists(filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")) || exists(filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs"))) {
			m.Components = appendUnique(m.Components, "database:"+name)
			break
		}
		if err := renderTree(project, "database", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "database:"+name)
	case "service":
		if !isWebAPI(m) || hasComponent(m.Components, "backend:simple") {
			return errors.New("service generation requires a non-simple webapi or fullstack project")
		}
		if err := renderTree(project, "service", d, templateDir(args), dryRun, force); err != nil {
			return err
		}
		m.Components = appendUnique(m.Components, "service:"+name)
	default:
		return fmt.Errorf("unsupported component %q", component)
	}
	if err := registerGeneratedProjectFiles(project, dryRun); err != nil {
		return err
	}
	if dryRun {
		fmt.Println("would update .aspgen/manifest.json")
		return nil
	}
	return saveManifest(project, m)
}

func renderTree(root, group string, d data, override string, dryRun, force bool) error {
	var source fs.FS = templates.FS
	prefix := filepath.ToSlash(filepath.Join("files", group))
	if override != "" {
		candidate := filepath.Join(override, group)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			source = os.DirFS(override)
			prefix = filepath.ToSlash(group)
		}
	}
	return fs.WalkDir(source, prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(path, prefix+"/"), ".tmpl")
		rel, err := renderString(filepath.ToSlash(rel), d)
		if err != nil {
			return fmt.Errorf("render path %s: %w", path, err)
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if exists(target) && !force {
			if group == "entity" && filepath.Base(target) == "AuditableEntity.cs" {
				return nil
			}
			return fmt.Errorf("refusing to overwrite %s", target)
		}
		if dryRun {
			fmt.Println("would create", target)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		content, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		body, err := renderString(string(content), d)
		if err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		return os.WriteFile(target, []byte(body), 0o644)
	})
}

func updateModuleCatalog(project, namespace, module string, dryRun bool) error {
	path := filepath.Join(project, "src", "Desktop", "App.xaml.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read module catalog host %s: %w", path, err)
	}
	textContent := string(content)
	registration := "        moduleCatalog.AddModule<" + namespace + "." + module + "Module>();"
	if strings.Contains(textContent, registration) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:modules") {
		return errors.New("App.xaml.cs is missing the // aspgen:modules marker")
	}
	using := "using " + namespace + ";\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	textContent = strings.Replace(textContent, "        // aspgen:modules", "        // aspgen:modules\n"+registration, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateFeatureHost(project, namespace, feature string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Program.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read feature host %s: %w", path, err)
	}
	textContent := string(content)
	using := "using " + namespace + ".WebApi.Features." + feature + ";\n"
	call := "app.Map" + feature + "Endpoints();"
	if strings.Contains(textContent, call) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:features") {
		return errors.New("Program.cs is missing the // aspgen:features marker")
	}
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	textContent = strings.Replace(textContent, "// aspgen:features", "// aspgen:features\n"+call, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateSeedHost(project, namespace, backend string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Program.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read seed host %s: %w", path, err)
	}
	textContent := string(content)
	call := "await DatabaseSeeder.SeedAsync(app.Services);"
	if strings.Contains(textContent, call) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:seed") {
		return errors.New("Program.cs is missing the // aspgen:seed marker")
	}
	if backend == "ddd" {
		using := "using " + namespace + ".WebApi.Seeding;\n"
		if !strings.Contains(textContent, using) {
			textContent = using + textContent
		}
	}
	textContent = strings.Replace(textContent, "// aspgen:seed", "// aspgen:seed\n"+call, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateSeedFile(project, namespace, backend, entity string, properties []Property, count int, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Data", "DatabaseSeeder.cs")
	if backend == "ddd" {
		path = filepath.Join(project, "src", "WebApi", "Seeding", "DatabaseSeeder.cs")
	} else if backend == "ddd-local" {
		path = filepath.Join(project, "src", "Infrastructure", "Seeding", "DatabaseSeeder.cs")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read seed file %s: %w", path, err)
	}
	textContent := string(content)
	using := "using " + namespace + ".WebApi.Models;\n"
	if backend == "ddd" {
		using = "using " + namespace + ".Domain.Entities;\n"
	} else if backend == "ddd-local" {
		using = "using " + namespace + ".Domain.Entities;\n"
	}
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	if strings.Contains(textContent, "db."+entity+"s.AnyAsync") {
		return nil
	}
	block := renderSeedBlock(backend, entity, properties, count)
	if !strings.Contains(textContent, "    // aspgen:seed") {
		return errors.New("DatabaseSeeder.cs is missing the // aspgen:seed marker")
	}
	textContent = strings.Replace(textContent, "    // aspgen:seed", "    // aspgen:seed\n"+block, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func renderSeedBlock(backend, entity string, properties []Property, count int) string {
	var b strings.Builder
	if backend == "ddd-local" {
		fmt.Fprintf(&b, "        if (!db.%ss.Any())\n        {\n", entity)
		fmt.Fprintf(&b, "            db.%ss.AddRange(\n", entity)
		for row := 0; row < count; row++ {
			fmt.Fprintf(&b, "                new %s(", entity)
			for i, property := range properties {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(seedLiteral(property.CSharpType, property.Name, row))
			}
			b.WriteString(")")
			if row < count-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("            );\n        }\n")
		return b.String()
	}
	fmt.Fprintf(&b, "        if (!await db.%ss.AnyAsync(cancellationToken))\n        {\n", entity)
	fmt.Fprintf(&b, "            db.%ss.AddRange(\n", entity)
	for row := 0; row < count; row++ {
		fmt.Fprintf(&b, "                new %s", entity)
		if backend == "ddd" {
			b.WriteString("(")
		} else {
			b.WriteString(" {")
		}
		for i, property := range properties {
			if i > 0 {
				b.WriteString(",")
			}
			value := seedLiteral(property.CSharpType, property.Name, row)
			if backend == "ddd" {
				b.WriteString(" ")
				b.WriteString(value)
			} else {
				fmt.Fprintf(&b, " %s = %s", property.Name, value)
			}
		}
		if backend == "ddd" {
			b.WriteString(")")
		} else {
			b.WriteString(" }")
		}
		if row < count-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("            );\n        }\n")
	return b.String()
}

func seedLiteral(csharpType, property string, row int) string {
	baseType := strings.TrimSuffix(csharpType, "?")
	switch baseType {
	case "string":
		return fmt.Sprintf("\"%s sample %d\"", property, row+1)
	case "int":
		return fmt.Sprintf("%d", 20+row)
	case "long":
		return fmt.Sprintf("%dL", 1001+row)
	case "decimal":
		return fmt.Sprintf("%d.50m", 100+row)
	case "float":
		return fmt.Sprintf("%d.5f", 4+row)
	case "bool":
		return strconv.FormatBool(row%2 == 0)
	case "DateOnly":
		return fmt.Sprintf("new DateOnly(%d, %d, %d)", 2000+row/336, row%12+1, row%28+1)
	case "DateTime":
		return fmt.Sprintf("new DateTime(%d, %d, %d, 10, 30, 0, DateTimeKind.Utc)", 2000+row/336, row%12+1, row%28+1)
	case "Guid":
		return fmt.Sprintf("Guid.Parse(\"00000000-0000-0000-0000-%012d\")", row+1)
	default:
		return "default!"
	}
}

func updateEntityDbContext(project, namespace, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read database context %s: %w", path, err)
	}
	textContent := string(content)
	using := "using " + namespace + ".Domain.Entities;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:entities") {
		return errors.New("AppDbContext.cs is missing the // aspgen:entities marker")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateEntityDbContextLocal(project, namespace, entity string, _ []Property, dryRun bool) error {
	path := filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read local database context %s: %w", path, err)
	}
	textContent := string(content)
	using := "using " + namespace + ".Domain.Entities;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:entities") {
		return errors.New("local AppDbContext.cs is missing the // aspgen:entities marker")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateSimpleDbContext(project, namespace, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read simple database context %s: %w", path, err)
	}
	textContent := string(content)
	using := "using " + namespace + ".WebApi.Models;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:entities") {
		return errors.New("simple AppDbContext.cs is missing the // aspgen:entities marker")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

func updateEntityDependencyInjection(project, namespace, entity string, dryRun bool) error {
	registrations := []struct {
		path, marker, line string
	}{
		{filepath.Join(project, "src", "Infrastructure", "DependencyInjection.cs"), "        // aspgen:repositories", "        services.AddScoped<Domain.Repositories.I" + entity + "Repository, Persistence." + entity + "Repository>();"},
	}
	for _, registration := range registrations {
		content, err := os.ReadFile(registration.path)
		if err != nil {
			return fmt.Errorf("read dependency injection file %s: %w", registration.path, err)
		}
		textContent := string(content)
		if strings.Contains(textContent, registration.line) {
			continue
		}
		if !strings.Contains(textContent, registration.marker) {
			return fmt.Errorf("%s is missing the %s marker", registration.path, strings.TrimSpace(registration.marker))
		}
		textContent = strings.Replace(textContent, registration.marker, registration.marker+"\n"+registration.line, 1)
		if dryRun {
			fmt.Println("would update", registration.path)
			continue
		}
		if err := os.WriteFile(registration.path, []byte(textContent), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderString(raw string, d data) (string, error) {
	t, err := template.New("file").Funcs(template.FuncMap{
		"pascal": pascal, "camel": camel, "kebab": kebab, "trimSuffix": strings.TrimSuffix,
	}).Parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}

func parseProperties(args []string) ([]Property, error) {
	var result []Property
	seen := map[string]bool{}
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if arg == "--project" || arg == "--templates" || arg == "--framework" || arg == "--context" || arg == "--aggregate" {
				skipNext = true
			}
			continue
		}
		parts := strings.Split(arg, ":")
		if len(parts) < 2 {
			continue
		}
		name, typ := parts[0], parts[1]
		if !validIdentifier(name) || seen[name] {
			return nil, fmt.Errorf("invalid or duplicate property %q", name)
		}
		mapped, ok := mapType(typ)
		if !ok {
			return nil, fmt.Errorf("unsupported property type %q", typ)
		}
		seen[name] = true
		propertyName := pascal(name)
		result = append(result, Property{Name: propertyName, DisplayName: humanize(propertyName), CSharpType: mapped, UIControl: controlForType(mapped)})
	}
	if len(result) == 0 {
		return nil, errors.New("at least one property is required, e.g. name:string")
	}
	return result, nil
}

func mapType(t string) (string, bool) {
	nullable := strings.HasSuffix(t, "?")
	t = strings.TrimSuffix(t, "?")
	m := map[string]string{"string": "string", "int": "int", "long": "long", "decimal": "decimal", "float": "float", "bool": "bool", "date": "DateOnly", "datetime": "DateTime", "guid": "Guid", "uuid": "Guid"}
	v, ok := m[t]
	if nullable && ok && v != "string" {
		v += "?"
	}
	if nullable && v == "string" {
		v += "?"
	}
	return v, ok
}

func controlForType(t string) string {
	switch strings.TrimSuffix(t, "?") {
	case "bool":
		return "InputCheckbox"
	case "int", "long", "decimal", "float":
		return "InputNumber"
	case "DateOnly", "DateTime":
		return "InputDate"
	default:
		return "InputText"
	}
}

func humanize(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	runes := []rune(value)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			previous := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && unicode.IsLower(next)) {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	words := strings.Fields(b.String())
	for i, word := range words {
		if word == "" {
			continue
		}
		wordRunes := []rune(strings.ToLower(word))
		wordRunes[0] = unicode.ToUpper(wordRunes[0])
		words[i] = string(wordRunes)
	}
	return strings.Join(words, " ")
}

func themeOption(args []string) (string, error) {
	for i, arg := range args {
		for _, key := range []string{"--theme", "-theme"} {
			if arg == key {
				if i+1 >= len(args) {
					return "", errors.New("theme value is required")
				}
				return validateTheme(args[i+1])
			}
			if strings.HasPrefix(arg, key+":") {
				return validateTheme(strings.TrimPrefix(arg, key+":"))
			}
		}
	}
	return "", nil
}

func backendOption(args []string) (string, error) {
	for i, arg := range args {
		for _, key := range []string{"--backend", "-backend"} {
			if arg == key {
				if i+1 >= len(args) {
					return "", errors.New("backend value is required")
				}
				return validateBackend(args[i+1])
			}
			if strings.HasPrefix(arg, key+":") {
				return validateBackend(strings.TrimPrefix(arg, key+":"))
			}
		}
	}
	return "", nil
}

func seedOption(args []string) (string, int, error) {
	for i, arg := range args {
		for _, key := range []string{"--seed", "-seed"} {
			if arg == key {
				if i+1 >= len(args) {
					return "", 0, errors.New("seed value is required")
				}
				count := 3
				if i+2 < len(args) && !strings.HasPrefix(args[i+2], "-") {
					parsed, err := strconv.Atoi(args[i+2])
					if err != nil {
						return "", 0, fmt.Errorf("invalid seed count %q", args[i+2])
					}
					count = parsed
				}
				seed, err := validateSeed(args[i+1])
				return seed, count, validateSeedCount(seed, count, err)
			}
			if strings.HasPrefix(arg, key+":") {
				value := strings.TrimPrefix(arg, key+":")
				parts := strings.Split(value, ":")
				count := 3
				if len(parts) > 1 {
					parsed, err := strconv.Atoi(parts[1])
					if err != nil {
						return "", 0, fmt.Errorf("invalid seed count %q", parts[1])
					}
					count = parsed
				}
				seed, err := validateSeed(parts[0])
				return seed, count, validateSeedCount(seed, count, err)
			}
		}
	}
	return "", 0, nil
}

func validateSeedCount(seed string, count int, seedErr error) error {
	if seedErr != nil {
		return seedErr
	}
	if seed == "" || seed == "none" {
		return nil
	}
	if count < 1 || count > 10000 {
		return fmt.Errorf("seed count must be between 1 and 10000, got %d", count)
	}
	return nil
}

func validateSeed(seed string) (string, error) {
	if seed == "" || seed == "none" {
		return "", nil
	}
	if seed != "dummy" {
		return "", fmt.Errorf("unsupported seed profile %q", seed)
	}
	return seed, nil
}

func databaseOption(args []string) (string, error) {
	for i, arg := range args {
		for _, key := range []string{"--database", "-database"} {
			if arg == key {
				if i+1 >= len(args) {
					return "", errors.New("database value is required")
				}
				return validateDatabase(args[i+1])
			}
			if strings.HasPrefix(arg, key+":") {
				return validateDatabase(strings.TrimPrefix(arg, key+":"))
			}
		}
	}
	return "", nil
}

func validateDatabase(database string) (string, error) {
	if database == "" || database == "none" {
		return "", nil
	}
	if database != "sqlite" && database != "postgres" {
		return "", fmt.Errorf("unsupported database %q; use sqlite or postgres", database)
	}
	return database, nil
}

func validateBackend(backend string) (string, error) {
	if backend == "" || backend == "none" {
		return "", nil
	}
	if backend != "ddd" {
		return "", fmt.Errorf("unsupported backend %q", backend)
	}
	return backend, nil
}

func validateTheme(theme string) (string, error) {
	if theme == "" || theme == "none" {
		return "", nil
	}
	if theme != "wpfui" {
		return "", fmt.Errorf("unsupported WPF theme %q", theme)
	}
	return theme, nil
}

func componentTheme(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "theme:") {
			return strings.TrimPrefix(component, "theme:")
		}
	}
	return ""
}

func componentBackend(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "backend:") {
			return strings.TrimPrefix(component, "backend:")
		}
	}
	return ""
}

func componentDatabase(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "database:") {
			return strings.TrimPrefix(component, "database:")
		}
	}
	return "sqlite"
}

func componentSeed(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "seed:") {
			value := strings.TrimPrefix(component, "seed:")
			return strings.Split(value, ":")[0]
		}
	}
	return ""
}

func componentSeedCount(components []string) int {
	for _, component := range components {
		if strings.HasPrefix(component, "seed:") {
			parts := strings.Split(component, ":")
			if len(parts) > 2 {
				if count, err := strconv.Atoi(parts[2]); err == nil && count > 0 {
					return count
				}
			}
			return 3
		}
	}
	return 0
}

func rejectAggregateReservedProperties(properties []Property) error {
	for _, property := range properties {
		if property.Name == "Id" {
			return errors.New("aggregate property \"id\" is reserved for the aggregate identity")
		}
	}
	return nil
}

func isRenoir(m Manifest) bool { return hasComponent(m.Components, "renoir") }
func isWebAPI(m Manifest) bool { return hasComponent(m.Components, "webapi") }
func hasComponent(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func contextExists(contexts []Context, name string) bool {
	for _, context := range contexts {
		if context.Name == name {
			return true
		}
	}
	return false
}
func appendContext(contexts []Context, value Context) []Context {
	if contextExists(contexts, value.Name) {
		return contexts
	}
	return append(contexts, value)
}
func addAggregate(contexts []Context, contextName, aggregateName string) []Context {
	for i := range contexts {
		if contexts[i].Name == contextName {
			for _, name := range contexts[i].Aggregates {
				if name == aggregateName {
					return contexts
				}
			}
			contexts[i].Aggregates = append(contexts[i].Aggregates, aggregateName)
		}
	}
	return contexts
}

func templateCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: aspgen templates export PATH | list")
	}
	switch args[0] {
	case "list":
		return fs.WalkDir(templates.FS, "files", func(path string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				fmt.Println(strings.TrimPrefix(path, "files/"))
			}
			return err
		})
	case "export":
		if len(args) < 2 {
			return errors.New("template export path is required")
		}
		return fs.WalkDir(templates.FS, "files", func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			target := filepath.Join(args[1], strings.TrimPrefix(path, "files/"))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			b, err := fs.ReadFile(templates.FS, path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, 0o644)
		})
	case "validate":
		if len(args) < 2 {
			return errors.New("template validation path is required")
		}
		return validateTemplateTree(os.DirFS(args[1]), ".")
	default:
		return fmt.Errorf("unknown templates command %q", args[0])
	}
}

func loadManifest(root string) (Manifest, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("project directory %s does not exist; create it first with aspgen new", root)
		}
		return Manifest{}, fmt.Errorf("cannot access project directory %s: %w", root, err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".aspgen", "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("not an aspgen project: missing .aspgen/manifest.json in %s; run from a generated project or pass --project PATH", root)
	}
	var m Manifest
	return m, json.Unmarshal(b, &m)
}

func discoverProjectRoot(start string) (string, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if exists(filepath.Join(root, ".aspgen", "manifest.json")) {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("not an aspgen project: missing .aspgen/manifest.json; run from a generated project or pass --project PATH")
		}
		root = parent
	}
}
func saveManifest(root string, m Manifest) error {
	sort.Strings(m.Components)
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.MkdirAll(filepath.Join(root, ".aspgen"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".aspgen", "manifest.json"), append(b, '\n'), 0o644)
}

// registerGeneratedProjectFiles makes incremental files explicit in their
// owning project without duplicating SDK-style default items.
func registerGeneratedProjectFiles(root string, dryRun bool) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	projects := make(map[string][]string)
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		normalized := filepath.ToSlash(path)
		if entry.IsDir() || strings.Contains(normalized, "/bin/") || strings.Contains(normalized, "/obj/") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".cs" && ext != ".xaml" {
			return nil
		}
		projectFile, err := owningProjectFile(filepath.Dir(path))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(projectFile), path)
		if err != nil {
			return err
		}
		projects[projectFile] = append(projects[projectFile], filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}
	for projectFile, files := range projects {
		sort.Strings(files)
		if err := updateProjectFileItems(projectFile, files, dryRun); err != nil {
			return err
		}
	}
	return nil
}

func owningProjectFile(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".csproj") {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no owning .csproj found for generated file below %s", start)
		}
		dir = parent
	}
}

func updateProjectFileItems(projectFile string, files []string, dryRun bool) error {
	content, err := os.ReadFile(projectFile)
	if err != nil {
		return err
	}
	textContent := string(content)
	const marker = "  <!-- aspgen:files -->"
	if !strings.Contains(textContent, marker) {
		textContent = strings.Replace(textContent, "</Project>", "  <ItemGroup>\n"+marker+"\n  </ItemGroup>\n</Project>", 1)
	}
	var additions []string
	for _, file := range files {
		item := "Compile"
		if strings.HasSuffix(strings.ToLower(file), ".xaml") {
			item = "Page"
			if strings.EqualFold(filepath.Base(file), "App.xaml") {
				item = "ApplicationDefinition"
			}
		}
		line := fmt.Sprintf("    <%s Update=\"%s\" />", item, file)
		if !strings.Contains(textContent, line) {
			additions = append(additions, line)
		}
	}
	if len(additions) == 0 {
		return nil
	}
	textContent = strings.Replace(textContent, marker, marker+"\n"+strings.Join(additions, "\n"), 1)
	if dryRun {
		fmt.Println("would update", projectFile)
		return nil
	}
	return os.WriteFile(projectFile, []byte(textContent), 0o644)
}

func projectFileName(project, suffix string) string {
	if strings.Contains(project, ".") {
		return project + "." + suffix + ".csproj"
	}
	return suffix + ".csproj"
}

func normalizeProjectFiles(root, project string) error {
	if !strings.Contains(project, ".") {
		return nil
	}
	for _, suffix := range []string{"Domain", "Application", "Infrastructure", "WebApi"} {
		oldPath := filepath.Join(root, "src", suffix, suffix+".csproj")
		newPath := filepath.Join(root, "src", suffix, projectFileName(project, suffix))
		if exists(oldPath) && oldPath != newPath {
			if exists(newPath) {
				return fmt.Errorf("cannot rename %s because %s already exists", oldPath, newPath)
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}
	return rewriteProjectReferences(root, project)
}

func rewriteProjectReferences(root, project string) error {
	replacements := map[string]string{}
	for _, suffix := range []string{"Domain", "Application", "Infrastructure", "WebApi"} {
		replacements[suffix+".csproj"] = projectFileName(project, suffix)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (filepath.Ext(path) != ".csproj" && filepath.Ext(path) != ".sln") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		textContent := string(content)
		for oldName, newName := range replacements {
			textContent = strings.ReplaceAll(textContent, oldName, newName)
		}
		return os.WriteFile(path, []byte(textContent), 0o644)
	})
}

func writeSolution(root, name, app string, force, simple bool, backend string) error {
	target := filepath.Join(root, name+".sln")
	if exists(target) && !force {
		return fmt.Errorf("refusing to overwrite %s", target)
	}
	projects := make([]string, 0, 8)
	targets := make([]string, 0, 8)
	if app == "webapi" || app == "fullstack" || (app == "wpf" && backend == "ddd") {
		projectsToAdd := []struct{ target, display, path string }{
			{"domain", name + ".Domain", "src\\Domain\\" + projectFileName(name, "Domain")},
			{"application", name + ".Application", "src\\Application\\" + projectFileName(name, "Application")},
			{"infrastructure", name + ".Infrastructure", "src\\Infrastructure\\" + projectFileName(name, "Infrastructure")},
			{"webapi", name + ".WebApi", "src\\WebApi\\" + projectFileName(name, "WebApi")},
		}
		if simple {
			projectsToAdd = []struct{ target, display, path string }{{"webapi", name + ".WebApi", "src\\WebApi\\" + projectFileName(name, "WebApi")}}
		}
		if app == "wpf" && backend == "ddd" {
			projectsToAdd = []struct{ target, display, path string }{
				{"domain", name + ".Domain", "src\\Domain\\" + projectFileName(name, "Domain")},
				{"application", name + ".Application", "src\\Application\\" + projectFileName(name, "Application")},
				{"infrastructure", name + ".Infrastructure", "src\\Infrastructure\\" + projectFileName(name, "Infrastructure")},
			}
		}
		for _, project := range projectsToAdd {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	if app == "wpf" || app == "fullstack" {
		projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s.Desktop\", \"src\\Desktop\\%s.Desktop.csproj\", \"{%s}\"", name, name, projectGUID(name, "wpf")))
		targets = append(targets, "wpf")
	}
	if app == "blazor" {
		for _, project := range []struct{ target, display, path string }{
			{"domain", name + ".DomainModel", "src\\" + name + ".DomainModel\\" + name + ".DomainModel.csproj"},
			{"application", name + ".Application", "src\\" + name + ".Application\\" + name + ".Application.csproj"},
			{"infrastructure", name + ".Infrastructure", "src\\" + name + ".Infrastructure\\" + name + ".Infrastructure.csproj"},
			{"persistence", name + ".Persistence", "src\\" + name + ".Persistence\\" + name + ".Persistence.csproj"},
			{"resources", name + ".Resources", "src\\" + name + ".Resources\\" + name + ".Resources.csproj"},
			{"app", name + ".AppBlazor", "src\\" + name + ".AppBlazor\\" + name + ".AppBlazor.csproj"},
		} {
			projects = append(projects, fmt.Sprintf("Project(\"{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}\") = \"%s\", \"%s\", \"{%s}\"", project.display, project.path, projectGUID(name, project.target)))
			targets = append(targets, project.target)
		}
	}
	projectText := make([]string, 0, len(projects))
	for _, project := range projects {
		projectText = append(projectText, project+"\nEndProject")
	}
	config := make([]string, 0, len(projects)*4)
	for _, target := range targets {
		guid := projectGUID(name, target)
		config = append(config,
			fmt.Sprintf("\t\t{%s}.Debug|Any CPU.ActiveCfg = Debug|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Debug|Any CPU.Build.0 = Debug|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Release|Any CPU.ActiveCfg = Release|Any CPU", guid),
			fmt.Sprintf("\t\t{%s}.Release|Any CPU.Build.0 = Release|Any CPU", guid),
		)
	}
	content := "Microsoft Visual Studio Solution File, Format Version 12.00\n# Visual Studio Version 17\nVisualStudioVersion = 17.0.31903.59\nMinimumVisualStudioVersion = 10.0.40219.1\n" + strings.Join(projectText, "\n") + "\nGlobal\n\tGlobalSection(SolutionConfigurationPlatforms) = preSolution\n\t\tDebug|Any CPU = Debug|Any CPU\n\t\tRelease|Any CPU = Release|Any CPU\n\tEndGlobalSection\n\tGlobalSection(ProjectConfigurationPlatforms) = postSolution\n" + strings.Join(config, "\n") + "\n\tEndGlobalSection\nEndGlobal\n"
	return os.WriteFile(target, []byte(content), 0o644)
}

func projectGUID(project, target string) string {
	sum := sha1.Sum([]byte(project + ":" + target))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func value(args []string, key, fallback string) string {
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return fallback
}
func templateDir(args []string) string { return value(args, "--templates", "") }
func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func validateTemplateTree(source fs.FS, root string) error {
	return fs.WalkDir(source, root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return walkErr
		}
		content, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		_, err = template.New(filepath.Base(path)).Funcs(template.FuncMap{
			"pascal": pascal, "camel": camel, "kebab": kebab, "trimSuffix": strings.TrimSuffix,
		}).Parse(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
}

func appendUnique(xs []string, value string) []string {
	for _, x := range xs {
		if x == value {
			return xs
		}
	}
	return append(xs, value)
}
func validIdentifier(s string) bool {
	if s == "" || (!unicode.IsLetter(rune(s[0])) && s[0] != '_') {
		return false
	}
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func validProjectName(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return false
	}
	for _, part := range parts {
		if !validIdentifier(part) {
			return false
		}
	}
	return true
}
func pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func camel(s string) string { p := pascal(s); return strings.ToLower(p[:1]) + p[1:] }
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
