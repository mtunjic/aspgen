package generator

// attachContextWpfUI wires `-ui wpf` onto a cqrs/es-tier --context/--arch
// project: renders the Prism/DryIoc Desktop shell (the "wpf" template group
// as-is - it has no dependency on the legacy --app flow) plus a per-aggregate
// wpf-entity module for every aggregate already recorded in the manifest.
// cqrs/es tiers always have a WebApi host, so every module gets the existing
// HTTP-backed Store variant (see {{ .Name }}Store.cs.tmpl's Backend branch).
// Relations for aggregates added before -ui wpf was attached are rebuilt via
// reconstructRelations from each EntityMeta's persisted Properties (the only
// thing the manifest keeps); aggregates added afterwards get relations
// straight from renderAggregateCrud's live *data (see add_ddd.go).
func attachContextWpfUI(project string, m *Manifest, theme, themeMode, override string, dryRun, force bool) error {
	backend := componentBackend(m.Components)
	shellData := data{Project: m.Project, Namespace: m.Project, Theme: theme, ThemeMode: themeMode, Backend: backend, Database: m.Persistence}
	if projectHasDmContext(*m) {
		// Desktop.csproj only needs an Application project reference (for
		// in-process CrudService calls) when at least one dm-tier context
		// exists ANYWHERE in the project - componentBackend above only
		// reflects whichever context was created first via `new`, which
		// may be cqrs/es in a mixed-tier project even though a dm context
		// (and its Desktop module) exists too.
		shellData.Backend = "dm"
	}
	if err := renderTree(project, "wpf", shellData, override, dryRun, force); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		// Each entity's own context may be at a different arch tier than
		// whichever context was created first (which is all `backend`
		// above reflects) - resolve its Store's HTTP-vs-in-process branch
		// from ITS OWN context, not the project-wide default.
		entityBackend := backend
		if ctx, ok := findContext(m.Contexts, entity.Context); ok && ctx.Arch != "" {
			entityBackend = archBackend(ctx.Arch)
		}
		reverseRelations := computeReverseRelations(m.Entities, entity.Name, entity.Context)
		if err := renderContextWpfModule(project, m.Project, entity.Name, entity.Context, entity.Properties, reconstructRelations(entity.Properties), reverseRelations, entityBackend, theme, themeMode, override, dryRun, force); err != nil {
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
func renderContextWpfModule(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, reverseRelations []ReverseRelation, backend, theme, themeMode, override string, dryRun, force bool) error {
	namespace := projectName + ".Desktop.Modules." + aggregate
	moduleData := data{
		Project:          projectName,
		Namespace:        namespace,
		Name:             aggregate,
		Context:          contextName,
		Properties:       properties,
		Relations:        relations,
		ReverseRelations: reverseRelations,
		Backend:          backend,
		Theme:            theme,
		ThemeMode:        themeMode,
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
	reverseRelations := computeReverseRelations(m.Entities, d.Aggregate, d.Context)
	return renderContextWpfModule(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, reverseRelations, d.Backend, d.Theme, d.ThemeMode, templateDir(r.Args), r.DryRun, r.Force)
}
