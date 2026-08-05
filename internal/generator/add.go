package generator

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// addRequest carries the context shared by every `add` subcommand handler.
type addRequest struct {
	Args      []string // flags following the component and name (original args[2:])
	Name      string
	Project   string
	DryRun    bool
	Force     bool
	Theme     string
	ThemeMode string
	Backend   string
}

var addHandlers = map[string]func(addRequest, *Manifest, *data) error{
	"context":        addContextCmd,
	"aggregate":      addAggregateCmd,
	"value-object":   addValueObjectCmd,
	"domain-service": addDomainServiceCmd,
	"repository":     addRepositoryCmd,
	"event":          addEventCmd,
	"ui":             addUICmd,
	"entity":         addEntityCmd,
	"entity-field":   addEntityFieldCmd,
}

func add(args []string) error {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Print(addHelp)
		return nil
	}
	if len(args) < 2 {
		return errors.New("usage: aspgen add KIND NAME --project PATH [flags]; run 'aspgen add --help' for details")
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
	d := data{Project: m.Project, Namespace: m.Project, Name: name, Database: componentDatabase(m.Components)}
	theme, err := themeOption(args[2:])
	if err != nil {
		return err
	}
	if theme == "" {
		theme = componentTheme(m.Components)
	}
	d.Theme = theme
	themeMode, err := themeModeOption(args[2:])
	if err != nil {
		return err
	}
	if themeMode == "" {
		themeMode = componentThemeMode(m.Components)
	}
	themeMode, themeModeValue := resolveThemeMode(theme, themeMode)
	d.ThemeMode = themeModeValue
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

	handler, ok := addHandlers[component]
	if !ok {
		kinds := make([]string, 0, len(addHandlers))
		for k := range addHandlers {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		return fmt.Errorf("unsupported component %q; use one of: %s", component, strings.Join(kinds, ", "))
	}
	r := addRequest{Args: args[2:], Name: name, Project: project, DryRun: dryRun, Force: force, Theme: theme, ThemeMode: themeMode, Backend: backend}
	if err := handler(r, &m, &d); err != nil {
		return err
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
