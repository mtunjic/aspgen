package generator

import (
	"errors"
	"fmt"
	"path/filepath"
)

func addFeatureCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid feature name %q", r.Name)
	}
	if !isNonSimpleWebAPI(*m) {
		return errors.New("feature generation requires a non-simple webapi or fullstack project")
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	if !isWebAPI(*m) {
		return errors.New("feature generation requires a webapi or fullstack project")
	}
	d.Properties = props
	if err := renderTree(r.Project, "webapi-feature", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if err := updateFeatureHost(r.Project, m.Project, r.Name, r.DryRun); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "feature:"+r.Name)
	return nil
}

func addUICmd(r addRequest, m *Manifest, d *data) error {
	if isContextEngine(*m) {
		return addContextUICmd(r, m)
	}
	framework := value(r.Args, "--framework", "wpf")
	if framework != "wpf" {
		return fmt.Errorf("unsupported UI framework %q", framework)
	}
	if !isWebAPI(*m) && !isWPFProject(*m) {
		return errors.New("adding WPF UI requires a webapi or fullstack project")
	}
	theme := r.Theme
	if theme == "" {
		theme = componentTheme(m.Components)
	}
	d.Theme = theme
	if err := renderTree(r.Project, "wpf", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "wpf")
	m.Components = appendUnique(m.Components, "prism-dryioc")
	if theme != "" {
		m.Components = appendUnique(m.Components, "theme:"+theme)
		m.Components = appendUnique(m.Components, "theme-mode:"+r.ThemeMode)
	}
	if isWebAPI(*m) {
		if err := writeSolution(r.Project, m.Project, "fullstack", true, hasComponent(m.Components, "backend:simple"), componentBackend(m.Components), false); err != nil {
			return err
		}
	}
	return nil
}

// addContextUICmd implements `add ui --framework wpf|blazor|spa` for
// --context/--arch engine projects (as opposed to the legacy --app/--backend
// flow above). Only spa is implemented so far (Phase 5, in progress);
// wpf/blazor require their own host-project scaffolding, not done yet.
func addContextUICmd(r addRequest, m *Manifest) error {
	framework := value(r.Args, "--framework", "")
	if framework == "" {
		return errors.New("add ui requires --framework wpf|blazor|spa")
	}
	if framework != "spa" {
		return fmt.Errorf("-ui %q is not implemented yet; only spa is currently supported for --context/--arch projects", framework)
	}
	if m.UI != "" && m.UI != framework && !r.Force {
		return fmt.Errorf("project already has a %q UI attached; use --force to replace", m.UI)
	}
	if !hasWebApiHost(r.Project) {
		return errors.New("-ui spa requires a cqrs or es arch-tier context (needs a WebApi host); no WebApi host was found")
	}
	if err := attachSpaHost(r.Project, m.Project, r.DryRun); err != nil {
		return err
	}
	m.UI = framework
	m.Components = appendUnique(m.Components, "ui:spa")
	return nil
}

func addDatabaseCmd(r addRequest, m *Manifest, d *data) error {
	if !isWebAPI(*m) {
		return errors.New("database generation requires a webapi or fullstack project")
	}
	if isWebAPI(*m) && (exists(filepath.Join(r.Project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")) || exists(filepath.Join(r.Project, "src", "WebApi", "Data", "AppDbContext.cs"))) {
		m.Components = appendUnique(m.Components, "database:"+r.Name)
		return nil
	}
	if err := renderTree(r.Project, "database", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "database:"+r.Name)
	return nil
}

func addServiceCmd(r addRequest, m *Manifest, d *data) error {
	if !isNonSimpleWebAPI(*m) {
		return errors.New("service generation requires a non-simple webapi or fullstack project")
	}
	if err := renderTree(r.Project, "service", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "service:"+r.Name)
	return nil
}
