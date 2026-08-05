package generator

// attachContextWpfUI wires `-ui wpf` onto a cqrs/es-tier --context/--arch
// project: renders the Prism/DryIoc Desktop shell (the "wpf" template group
// as-is - it has no dependency on the legacy --app flow) plus a per-aggregate
// wpf-entity module for every aggregate already recorded in the manifest.
// cqrs/es tiers always have a WebApi host, so every module gets the existing
// HTTP-backed Store variant (see {{ .Name }}Store.cs.tmpl's Backend branch).
// Relations are not retrofitted for aggregates added before -ui wpf was
// attached, since EntityMeta only persists Properties, not Relations;
// aggregates added afterwards get full relation support via
// renderAggregateCrud's live *data (see add_ddd.go).
func attachContextWpfUI(project string, m *Manifest, theme, themeMode, override string, dryRun, force bool) error {
	backend := componentBackend(m.Components)
	shellData := data{Project: m.Project, Namespace: m.Project, Theme: theme, ThemeMode: themeMode, Backend: backend, Database: m.Persistence}
	if err := renderTree(project, "wpf", shellData, override, dryRun, force); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		if err := renderContextWpfModule(project, m.Project, entity.Name, entity.Context, entity.Properties, nil, backend, theme, themeMode, override, dryRun, force); err != nil {
			return err
		}
	}
	return nil
}

// renderContextWpfModule renders a single aggregate's wpf-entity Desktop
// module (list/edit/details views + HTTP-backed Store) and registers it with
// the Desktop shell's module catalog. Shared by attachContextWpfUI (existing
// aggregates) and renderAggregateCrud's cqrs/es cases (aggregates added after
// -ui wpf is already attached).
func renderContextWpfModule(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, backend, theme, themeMode, override string, dryRun, force bool) error {
	namespace := projectName + ".Desktop.Modules." + aggregate
	moduleData := data{
		Project:    projectName,
		Namespace:  namespace,
		Name:       aggregate,
		Context:    contextName,
		Properties: properties,
		Relations:  relations,
		Backend:    backend,
		Theme:      theme,
		ThemeMode:  themeMode,
	}
	if err := renderTree(project, "wpf-entity", moduleData, override, dryRun, force); err != nil {
		return err
	}
	return updateModuleCatalog(project, namespace, aggregate, dryRun)
}

// renderContextWpfModuleIfAttached renders the aggregate just handled by
// renderAggregateCrud's cqrs/es cases as a Desktop module, but only if -ui
// wpf has already been attached to this project (add_ddd.go, addAggregateCmd
// -> renderAggregateCrud). No-op otherwise.
func renderContextWpfModuleIfAttached(r addRequest, m *Manifest, d data) error {
	if m.UI != "wpf" {
		return nil
	}
	return renderContextWpfModule(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, d.Backend, d.Theme, d.ThemeMode, templateDir(r.Args), r.DryRun, r.Force)
}
