package generator

// attachContextBlazorUI wires `-ui blazor` onto a cqrs/es-tier
// --context/--arch project: renders the AppBlazor host (a fresh Blazor
// Server project that talks to the WebApi host over HttpClient, unlike
// legacy Renoir's in-process AppBlazor) plus a per-aggregate CRUD/details
// page for every aggregate already recorded in the manifest.
func attachContextBlazorUI(project string, m *Manifest, override string, dryRun, force bool) error {
	hostData := data{Project: m.Project, Namespace: m.Project}
	if err := renderTree(project, "blazor-context", hostData, override, dryRun, force); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		if err := renderContextBlazorCrud(project, m.Project, entity.Name, entity.Context, entity.Properties, nil, override, dryRun, force); err != nil {
			return err
		}
	}
	return nil
}

// renderContextBlazorCrud renders a single aggregate's CRUD/details Razor
// pages. Shared by attachContextBlazorUI (existing aggregates) and
// renderAggregateCrud's cqrs/es cases (aggregates added after -ui blazor is
// already attached).
func renderContextBlazorCrud(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, override string, dryRun, force bool) error {
	pageData := data{
		Project:    projectName,
		Namespace:  projectName,
		Context:    contextName,
		Aggregate:  aggregate,
		Properties: properties,
		Relations:  relations,
	}
	return renderTree(project, "blazor-context-crud", pageData, override, dryRun, force)
}

// renderContextBlazorCrudIfAttached renders the aggregate just handled by
// renderAggregateCrud's cqrs/es cases as a Razor CRUD page, but only if -ui
// blazor has already been attached to this project. No-op otherwise.
func renderContextBlazorCrudIfAttached(r addRequest, m *Manifest, d data) error {
	if m.UI != "blazor" {
		return nil
	}
	return renderContextBlazorCrud(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, templateDir(r.Args), r.DryRun, r.Force)
}
