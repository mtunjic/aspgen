package generator

import (
	"errors"
	"fmt"
	"path/filepath"
)

// isLegacyDDDTarget reports whether app/backend name a legacy profile target
// that supports --backend ddd/--database/--seed: webapi, fullstack, or a
// local DDD wpf app (wpf + --backend ddd).
func isLegacyDDDTarget(app, backend string) bool {
	return app == "webapi" || app == "fullstack" || (app == "wpf" && backend == "ddd")
}

func newProject(args []string) error {
	if len(args) == 0 {
		return errors.New("project name is required; run 'aspgen new --help' for usage")
	}
	if isHelp(args[0]) {
		fmt.Print(newHelp)
		return nil
	}
	name := args[0]
	// --context alone is also used by the legacy blazor/Renoir --script import
	// flow; only --arch (unique to the new engine) triggers newContextProject.
	if v, _, _ := matchOption(args[1:], "--context"); v != "" {
		if archVal, _, _ := matchOption(args[1:], "--arch"); archVal != "" {
			return newContextProject(name, args[1:])
		}
	}
	// Deprecated: --app/--backend/--simple/--theme are the legacy profile
	// selection path, kept working but no longer advertised in --help.
	// Prefer --context/--arch (newContextProject above) for new projects.
	app := value(args[1:], "--app", "webapi")
	out := value(args[1:], "--output", name)
	theme, err := themeOption(args[1:])
	if err != nil {
		return err
	}
	themeMode, err := themeModeOption(args[1:])
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
	dbImport, dbImportRequested, err := parseDBImportFlags(args[1:])
	if err != nil {
		return err
	}
	if !validProjectName(name) {
		return fmt.Errorf("invalid project name %q", name)
	}
	if app != "webapi" && app != "wpf" && app != "blazor" && app != "fullstack" {
		return fmt.Errorf("unsupported app target %q; use webapi, wpf, blazor, or fullstack", app)
	}
	if theme != "" && app != "wpf" && app != "fullstack" {
		return errors.New("--theme requires a wpf or fullstack application")
	}
	if themeMode != "" && theme != "wpfui" {
		return errors.New("--theme-mode requires --theme wpfui")
	}
	if backend != "" && !isLegacyDDDTarget(app, backend) {
		return errors.New("--backend ddd requires a webapi, fullstack, or local DDD wpf application")
	}
	if database != "" && !isLegacyDDDTarget(app, backend) {
		return errors.New("--database requires a webapi, fullstack, or local DDD wpf application")
	}
	if simple && backend != "" {
		return errors.New("--simple and --backend cannot be used together")
	}
	if simple && app != "webapi" && app != "fullstack" {
		return errors.New("--simple requires a webapi or fullstack application")
	}
	if seed != "" && !isLegacyDDDTarget(app, backend) {
		return errors.New("--seed requires a webapi, fullstack, or local DDD wpf application")
	}
	if seed != "" && !simple && backend == "" {
		return errors.New("--seed requires --simple or --backend ddd")
	}
	if dbImportRequested {
		if app != "blazor" && !isLegacyDDDTarget(app, backend) {
			return errors.New("--script requires a webapi, fullstack, blazor, or local DDD wpf application")
		}
		if app == "blazor" {
			if dbImport.Context == "" {
				return errors.New("--script on the blazor/Renoir profile requires --context ContextName")
			}
		} else if !simple && backend == "" {
			return errors.New("--script requires --simple or --backend ddd")
		}
	}
	if database == "" {
		database = "sqlite"
	}
	themeMode, themeModeValue := resolveThemeMode(theme, themeMode)
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
			manifest.Components = append(manifest.Components, "theme:"+theme, "theme-mode:"+themeMode)
		}
		if err := renderTree(out, "wpf", data{Project: name, Namespace: name, Theme: theme, ThemeMode: themeModeValue, Backend: backend, Seed: seed, SeedCount: seedCount}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	}
	if app == "blazor" {
		manifest.Components = append(manifest.Components, "renoir")
		if err := renderTree(out, "renoir", data{Project: name, Namespace: name}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	}
	if dbImportRequested {
		resolvedBackend := backend
		if simple {
			resolvedBackend = "simple"
		}
		if err := runDBImport(out, &manifest, resolvedBackend, dbImport, dryRun, force); err != nil {
			return err
		}
	}
	if !dryRun {
		if err := registerGeneratedProjectFiles(out, dryRun); err != nil {
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
	return writeSolution(out, name, app, force, simple, backend, false)
}

// newContextProject implements `aspgen new NAME --context CTX --arch TIER
// [-ui UI] [--database DB] [flags]`, the entry point for the --context/--arch
// engine. Presence of --context on `new` bypasses all legacy --app/--backend
// validation and flags; the `ar`, `dm`, `cqrs`, and `es` arch tiers are all
// implemented now. ar/dm are headless (-ui is not implemented yet, Phase 5);
// cqrs and es each get their own headless-until-endpoints WebApi Minimal API
// host since vertical-slice features need somewhere to be mounted.
func newContextProject(name string, args []string) error {
	if !validProjectName(name) {
		return fmt.Errorf("invalid project name %q", name)
	}
	contextName, err := contextOption(args)
	if err != nil {
		return err
	}
	if !validIdentifier(contextName) {
		return fmt.Errorf("invalid context name %q", contextName)
	}
	arch, err := archOption(args)
	if err != nil {
		return err
	}
	if arch == "" {
		return errors.New("--context requires --arch ar|dm|cqrs|es")
	}
	if arch != "ar" && arch != "dm" && arch != "cqrs" && arch != "es" {
		return fmt.Errorf("unrecognized arch tier %q; use ar, dm, cqrs, or es", arch)
	}
	ui, err := uiOption(args)
	if err != nil {
		return err
	}
	if ui != "" && ui != "spa" {
		return fmt.Errorf("-ui %q is not implemented yet; only spa is currently supported", ui)
	}
	if ui == "spa" && arch != "cqrs" && arch != "es" {
		return fmt.Errorf("-ui spa requires --arch cqrs or es (needs a WebApi host); %q is headless", arch)
	}
	database, err := databaseOption(args)
	if err != nil {
		return err
	}
	if database == "" {
		database = "sqlite"
	}
	out := value(args, "--output", name)
	if exists(filepath.Join(out, ".aspgen", "manifest.json")) {
		return fmt.Errorf("%s is already an aspgen project", out)
	}
	dryRun := hasFlag(args, "--dry-run")
	force := hasFlag(args, "--force")
	withTests := !hasFlag(args, "--no-tests")

	manifest := Manifest{
		Project:     name,
		UI:          ui,
		Persistence: database,
		Contexts:    []Context{{Name: contextName, Arch: arch}},
		Components:  []string{"context-engine", "context:" + contextName, "database:" + database},
	}
	solutionApp, simple := "webapi", true
	switch arch {
	case "ar":
		manifest.Components = append(manifest.Components, "webapi", "backend:simple")
		if err := renderTree(out, "simple-webapi", data{Project: name, Namespace: name, Backend: "simple", Database: database, Arch: arch}, templateDir(args), dryRun, force); err != nil {
			return err
		}
	case "cqrs":
		manifest.Components = append(manifest.Components, "backend:cqrs")
		if err := renderTree(out, "cqrs", data{Project: name, Namespace: name, Database: database}, templateDir(args), dryRun, force); err != nil {
			return err
		}
		solutionApp, simple = "cqrs", false
	case "es":
		manifest.Components = append(manifest.Components, "backend:es")
		if err := renderTree(out, "es", data{Project: name, Namespace: name, Database: database}, templateDir(args), dryRun, force); err != nil {
			return err
		}
		solutionApp, simple = "es", false
	default:
		manifest.Components = append(manifest.Components, "backend:dm")
		if err := renderTree(out, "dm", data{Project: name, Namespace: name, Database: database}, templateDir(args), dryRun, force); err != nil {
			return err
		}
		solutionApp, simple = "dm", false
	}
	if withTests {
		if err := renderTree(out, "tests-unit", data{Project: name, Namespace: name, Arch: arch, Database: database}, templateDir(args), dryRun, force); err != nil {
			return err
		}
		manifest.Components = append(manifest.Components, "tests:unit")
		if arch != "dm" {
			if err := renderTree(out, "tests-integration", data{Project: name, Namespace: name, Arch: arch, Database: database}, templateDir(args), dryRun, force); err != nil {
				return err
			}
			manifest.Components = append(manifest.Components, "tests:integration")
		}
	}
	// scripts/ci.ps1: generated for every --context/--arch project regardless
	// of --no-tests, since it's a useful restore/build(/test/publish) driver
	// even without generated tests.
	if err := renderTree(out, "ci", data{Project: name, Namespace: name, Arch: arch, Database: database}, templateDir(args), dryRun, force); err != nil {
		return err
	}
	manifest.Components = append(manifest.Components, "ci:script")
	if !dryRun {
		if err := registerGeneratedProjectFiles(out, dryRun); err != nil {
			return err
		}
	}
	if ui == "spa" {
		if err := attachSpaHost(out, name, dryRun); err != nil {
			return err
		}
		manifest.Components = append(manifest.Components, "ui:spa")
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
	return writeSolution(out, name, solutionApp, force, simple, "", withTests)
}
