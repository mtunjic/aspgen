package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

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
	if err := updateReadmeRunSection(project, m.Project, "mvc", "dm", dryRun); err != nil {
		return err
	}
	// The WebMvc is a real ASP.NET Core host, so dm-tier MVC projects get an
	// IntegrationTests project (WebApplicationFactory) alongside the usual
	// UnitTests. Its relation test files are added per-aggregate at `add`
	// time; this renders the project shell itself.
	if err := renderTree(project, "tests-mvc", hostData, override, dryRun, force); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		if err := renderContextMvcCrud(project, m.Project, entity.Name, entity.Context, entity.Properties, reconstructRelations(entity.Properties), reconstructManyToMany(entity, m.Entities), override, dryRun, force); err != nil {
			return err
		}
		if err := registerMvcCrudService(project, m.Project, entity.Name, dryRun); err != nil {
			return err
		}
		if err := updateMvcNav(project, m.Project, entity.Name, dryRun); err != nil {
			return err
		}
		if err := updateHomeRedirect(project, m.Project, entity.Name, dryRun); err != nil {
			return err
		}
	}
	return nil
}

// renderContextMvcCrud renders a single aggregate's Controller/Views. Shared
// by attachContextMvcUI (existing aggregates) and renderAggregateCrud's dm
// case (aggregates added after -ui mvc is already attached).
func renderContextMvcCrud(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, manyToMany []ManyToManyRelation, override string, dryRun, force bool) error {
	pageData := data{
		Project:    projectName,
		Namespace:  projectName,
		Context:    contextName,
		Aggregate:  aggregate,
		Properties: properties,
		Relations:  relations,
		ManyToMany: manyToMany,
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
	if err := renderContextMvcCrud(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, d.ManyToMany, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if err := registerMvcCrudService(r.Project, m.Project, d.Aggregate, r.DryRun); err != nil {
		return err
	}
	if err := updateMvcNav(r.Project, m.Project, d.Aggregate, r.DryRun); err != nil {
		return err
	}
	return updateHomeRedirect(r.Project, m.Project, d.Aggregate, r.DryRun)
}

// updateMvcNav adds a navbar link for aggregate to the WebMvc layout's
// navigation list, guarded by the `<!-- aspgen:nav -->` marker so every
// aggregate added to a dm-tier MVC project becomes reachable from the shell.
func updateMvcNav(project, projectName, aggregate string, dryRun bool) error {
	path := filepath.Join(project, "src", projectName+".WebMvc", "Views", "Shared", "_Layout.cshtml")
	content, err := readMarkerFile(path, "_Layout.cshtml")
	if err != nil {
		return err
	}
	content = normalizeCRLF(content)
	li := "                    <li class=\"nav-item\"><a class=\"nav-link\" asp-controller=\"" + aggregate + "\" asp-action=\"Index\">" + aggregate + "</a></li>"
	if strings.Contains(content, li) {
		return nil
	}
	if !strings.Contains(content, "<!-- aspgen:nav -->") {
		return missingMarkerErr("_Layout.cshtml", "<!-- aspgen:nav -->")
	}
	content = strings.Replace(content, "<!-- aspgen:nav -->", "<!-- aspgen:nav -->\n"+li, 1)
	return writeMarkerFile(path, content, dryRun)
}

// updateHomeRedirect makes the WebMvc landing page land on an aggregate's
// searchable Index (the first aggregate added wins) by replacing the
// `// aspgen:redirect` marker in HomeController with a RedirectToAction call.
func updateHomeRedirect(project, projectName, aggregate string, dryRun bool) error {
	path := filepath.Join(project, "src", projectName+".WebMvc", "Controllers", "HomeController.cs")
	content, err := readMarkerFile(path, "HomeController.cs")
	if err != nil {
		return err
	}
	content = normalizeCRLF(content)
	marker := "        // aspgen:redirect\n        return View();"
	redirect := fmt.Sprintf("        return RedirectToAction(\"Index\", %q);", aggregate)
	if !strings.Contains(content, marker) {
		return nil // already redirected (the first aggregate handled it)
	}
	content = strings.Replace(content, marker, redirect, 1)
	return writeMarkerFile(path, content, dryRun)
}

// normalizeCRLF converts CRLF line endings to LF so marker matching works
// regardless of the working-tree/checkout line endings (templates and
// generated files may legitimately be either LF or CRLF on Windows).
func normalizeCRLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
