package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

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
	if err := updateReadmeRunSection(project, m.Project, "blazor", componentBackend(m.Components), dryRun); err != nil {
		return err
	}
	for _, entity := range m.Entities {
		if err := renderContextBlazorCrud(project, m.Project, entity.Name, entity.Context, entity.Properties, reconstructRelations(entity.Properties), reconstructManyToMany(entity, m.Entities), buildBlazorQuickAdds(reconstructRelations(entity.Properties), m.Entities), override, dryRun, force); err != nil {
			return err
		}
		if err := updateBlazorNav(project, m.Project, entity.Name, entity.Context, dryRun); err != nil {
			return err
		}
	}
	return nil
}

// renderContextBlazorCrud renders a single aggregate's CRUD/details Razor
// pages. Shared by attachContextBlazorUI (existing aggregates) and
// renderAggregateCrud's cqrs/es cases (aggregates added after -ui blazor is
// already attached).
func renderContextBlazorCrud(project, projectName, aggregate, contextName string, properties []Property, relations []Relation, manyToMany []ManyToManyRelation, blazorQuickAdds map[string]string, override string, dryRun, force bool) error {
	pageData := data{
		Project:                projectName,
		Namespace:              projectName,
		Context:                contextName,
		Aggregate:              aggregate,
		Properties:             properties,
		Relations:              relations,
		ManyToMany:             manyToMany,
		RelationBlazorQuickAdds: blazorQuickAdds,
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
	if err := renderContextBlazorCrud(r.Project, m.Project, d.Aggregate, d.Context, d.Properties, d.Relations, d.ManyToMany, buildBlazorQuickAdds(d.Relations, m.Entities), templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	return updateBlazorNav(r.Project, m.Project, d.Aggregate, d.Context, r.DryRun)
}

// updateBlazorNav adds a navbar link for aggregate to the Blazor shell's
// MainLayout navigation, guarded by the `<!-- aspgen:nav -->` marker so every
// aggregate added to a cqrs/es -ui blazor project becomes reachable from the
// shell.
func updateBlazorNav(project, projectName, aggregate, contextName string, dryRun bool) error {
	path := filepath.Join(project, "src", projectName+".AppBlazor", "Components", "Layout", "MainLayout.razor")
	content, err := readMarkerFile(path, "MainLayout.razor")
	if err != nil {
		return err
	}
	content = normalizeCRLF(content)
	href := blazorNavHref(contextName, aggregate)
	li := "                <li class=\"nav-item\"><NavLink class=\"nav-link\" href=\"" + href + "\">" + aggregate + "s</NavLink></li>"
	if strings.Contains(content, li) {
		return nil
	}
	if !strings.Contains(content, "<!-- aspgen:nav -->") {
		return missingMarkerErr("MainLayout.razor", "<!-- aspgen:nav -->")
	}
	content = strings.Replace(content, "<!-- aspgen:nav -->", "<!-- aspgen:nav -->\n"+li, 1)
	return writeMarkerFile(path, content, dryRun)
}

// ensureHref helper keeps the aggregate href construction testable.
func blazorNavHref(contextName, aggregate string) string {
	return fmt.Sprintf("/%s/%ss", kebab(contextName), kebab(aggregate))
}
