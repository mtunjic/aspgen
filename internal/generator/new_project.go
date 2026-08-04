package generator

import (
	"errors"
	"fmt"
	"path/filepath"
)

func newProject(args []string) error {
	if len(args) == 0 {
		return errors.New("project name is required; run 'aspgen new --help' for usage")
	}
	if isHelp(args[0]) {
		fmt.Print(newHelp)
		return nil
	}
	name := args[0]
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
	if dbImportRequested {
		if app != "webapi" && app != "fullstack" && app != "blazor" && !(app == "wpf" && backend == "ddd") {
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
	return writeSolution(out, name, app, force, simple, backend)
}
