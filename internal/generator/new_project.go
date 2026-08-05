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
	contextName, err := contextOption(args[1:])
	if err != nil {
		return err
	}
	if contextName == "" {
		return errors.New("new requires --context CTX --arch ar|dm|cqrs|es; run 'aspgen new --help' for usage")
	}
	return newContextProject(name, args[1:])
}

// newContextProject implements `aspgen new NAME --context CTX --arch TIER
// [-ui UI] [--database DB] [flags]`, the entry point for the --context/--arch
// engine (the only project bootstrap path). ar/dm/cqrs/es arch tiers are all
// implemented. dm is headless (no WebApi host); cqrs and es each get their
// own headless-until-endpoints WebApi Minimal API host since vertical-slice
// features need somewhere to be mounted.
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
	if ui != "" && ui != "spa" && ui != "wpf" && ui != "blazor" && ui != "mvc" {
		return fmt.Errorf("-ui %q is not implemented yet; use spa, wpf, blazor, or mvc", ui)
	}
	if ui == "mvc" && arch != "dm" {
		return fmt.Errorf("-ui mvc currently only supports --arch dm (in-process CrudService calls, no WebApi host); use wpf/blazor/spa for cqrs/es")
	}
	if ui == "wpf" && arch != "cqrs" && arch != "es" && arch != "dm" {
		return fmt.Errorf("-ui wpf requires --arch cqrs, es, or dm; %q is not supported", arch)
	}
	if (ui == "spa" || ui == "blazor") && arch != "cqrs" && arch != "es" {
		return fmt.Errorf("-ui %s requires --arch cqrs or es (needs a WebApi host); %q is headless", ui, arch)
	}
	theme, err := themeOption(args)
	if err != nil {
		return err
	}
	if theme != "" && ui != "wpf" {
		return errors.New("--theme requires -ui wpf")
	}
	themeMode, err := themeModeOption(args)
	if err != nil {
		return err
	}
	themeMode, themeModeValue := resolveThemeMode(theme, themeMode)
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
	if ui == "wpf" {
		if err := attachContextWpfUI(out, &manifest, theme, themeModeValue, templateDir(args), dryRun, force); err != nil {
			return err
		}
		manifest.Components = append(manifest.Components, "ui:wpf")
		if theme != "" {
			manifest.Components = append(manifest.Components, "theme:"+theme, "theme-mode:"+themeMode)
		}
	}
	if ui == "blazor" {
		if err := attachContextBlazorUI(out, &manifest, templateDir(args), dryRun, force); err != nil {
			return err
		}
		manifest.Components = append(manifest.Components, "ui:blazor")
	}
	if ui == "mvc" {
		if err := attachContextMvcUI(out, &manifest, templateDir(args), dryRun, force); err != nil {
			return err
		}
		manifest.Components = append(manifest.Components, "ui:mvc")
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
	return writeSolution(out, name, solutionApp, force, simple, "", withTests, ui)
}
