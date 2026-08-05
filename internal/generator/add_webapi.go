package generator

import (
	"errors"
	"fmt"
)

func addUICmd(r addRequest, m *Manifest, d *data) error {
	if !isContextEngine(*m) {
		return errors.New("add ui requires a --context/--arch engine project; run 'aspgen add ui --help'")
	}
	return addContextUICmd(r, m)
}

// addContextUICmd implements `add ui --framework wpf|blazor|spa|mvc` for
// --context/--arch engine projects. spa and blazor need a WebApi host
// (cqrs/es tiers only); wpf works on cqrs/es (HTTP, calling the WebApi) or
// dm (in-process, calling the CrudService directly - dm has no WebApi
// host); mvc is dm-only, in-process.
// TODO: blazor could also target dm in-process (mirrors the wpf/mvc
// in-process pattern) and mvc could target cqrs/es over HTTP (mirrors wpf's
// HTTP Store) - neither is implemented; not requested yet.
func addContextUICmd(r addRequest, m *Manifest) error {
	framework := value(r.Args, "--framework", "")
	if framework == "" {
		return errors.New("add ui requires --framework wpf|blazor|spa|mvc")
	}
	if framework != "spa" && framework != "wpf" && framework != "blazor" && framework != "mvc" {
		return fmt.Errorf("-ui %q is not implemented yet; use spa, wpf, blazor, or mvc for --context/--arch projects", framework)
	}
	if m.UI != "" && m.UI != framework && !r.Force {
		return fmt.Errorf("project already has a %q UI attached; use --force to replace", m.UI)
	}
	if framework == "mvc" {
		if componentBackend(m.Components) != "dm" {
			return fmt.Errorf("-ui mvc currently only supports a dm-tier context; this project is %q", componentBackend(m.Components))
		}
		if err := attachContextMvcUI(r.Project, m, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		withTests := hasComponent(m.Components, "tests:unit")
		if err := writeSolution(r.Project, m.Project, componentBackend(m.Components), true, false, "", withTests, "mvc"); err != nil {
			return err
		}
		m.UI = framework
		m.Components = appendUnique(m.Components, "ui:mvc")
		return nil
	}
	if framework == "wpf" {
		backend := componentBackend(m.Components)
		if backend != "dm" && !hasWebApiHost(r.Project) {
			return errors.New("-ui wpf requires a cqrs or es arch-tier context (needs a WebApi host) or a dm arch-tier context (in-process); no WebApi host was found")
		}
		theme := r.Theme
		themeMode := r.ThemeMode
		if err := attachContextWpfUI(r.Project, m, theme, themeMode, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		withTests := hasComponent(m.Components, "tests:unit")
		if err := writeSolution(r.Project, m.Project, backend, true, false, "", withTests, "wpf"); err != nil {
			return err
		}
		m.UI = framework
		m.Components = appendUnique(m.Components, "ui:wpf")
		if theme != "" {
			m.Components = appendUnique(m.Components, "theme:"+theme)
			m.Components = appendUnique(m.Components, "theme-mode:"+themeMode)
		}
		return nil
	}
	if !hasWebApiHost(r.Project) {
		return fmt.Errorf("-ui %s requires a cqrs or es arch-tier context (needs a WebApi host); no WebApi host was found", framework)
	}
	if framework == "blazor" {
		if err := attachContextBlazorUI(r.Project, m, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		withTests := hasComponent(m.Components, "tests:unit")
		if err := writeSolution(r.Project, m.Project, componentBackend(m.Components), true, false, "", withTests, "blazor"); err != nil {
			return err
		}
		m.UI = framework
		m.Components = appendUnique(m.Components, "ui:blazor")
		return nil
	}
	if err := attachSpaHost(r.Project, m.Project, r.DryRun); err != nil {
		return err
	}
	m.UI = framework
	m.Components = appendUnique(m.Components, "ui:spa")
	return nil
}
