package generator

// attachContextMvcUI wires `-ui mvc` onto a dm-tier --context/--arch
// project: dm has no WebApi host, so unlike wpf/blazor (HTTP, cqrs/es only)
// the MVC host calls the aggregate's CrudService directly, in-process,
// mirroring legacy Renoir's in-process AppBlazor wiring. Renders the
// {Project}.WebMvc host once plus a Controller/Views set for every aggregate
// already recorded in the manifest.
func attachContextMvcUI(project string, m *Manifest, override string, dryRun, force bool) error {
	hostData := data{Project: m.Project, Namespace: m.Project, Database: m.Persistence}
	if err := renderTree(project, "mvc-context", hostData, override, dryRun, force); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		if err := renderContextMvcCrud(project, m.Project, entity.Name, entity.Context, entity.Properties, reconstructRelations(entity.Properties), override, dryRun, force); err != nil {
			return err
		}
		if err := registerMvcCrudService(project, m.Project, entity.Name, dryRun); err != nil {
			return err
		}
	}
	return nil
}

// renderContextMvcCrud renders a single aggregate's Controller/Views. Shared
// by attachContextMvcUI (existing aggregates) and renderAggregateCrud's dm
// case (aggregates added after -ui mvc is already attached).
func renderContextMvcCrud(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, override string, dryRun, force bool) error {
	pageData := data{
		Project:    projectName,
		Namespace:  projectName,
		Context:    contextName,
		Aggregate:  aggregate,
		Properties: properties,
		Relations:  relations,
	}
	return renderTree(project, "mvc-context-crud", pageData, override, dryRun, force)
}

// registerMvcCrudService registers an aggregate's CrudService with the
// WebMvc host's DI container. Reuses updateBlazorServiceHost - despite the
// name it's just a generic "src/{dir}/Program.cs" marker patcher.
func registerMvcCrudService(project, projectName, aggregate string, dryRun bool) error {
	using := "using " + projectName + ".Application;\n"
	registration := "        builder.Services.AddScoped<" + aggregate + "CrudService>();"
	return updateBlazorServiceHost(project, projectName+".WebMvc", []string{using}, registration, dryRun)
}

// renderContextMvcCrudIfAttached renders the aggregate just handled by
// renderAggregateCrud's dm case as a Controller/Views set, but only if -ui
// mvc has already been attached to this project. No-op otherwise.
func renderContextMvcCrudIfAttached(r addRequest, m *Manifest, d data) error {
	if m.UI != "mvc" {
		return nil
	}
	if err := renderContextMvcCrud(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	return registerMvcCrudService(r.Project, m.Project, d.Aggregate, r.DryRun)
}
