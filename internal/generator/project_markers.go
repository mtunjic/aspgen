package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readMarkerFile reads path, wrapping any error with description so callers
// get a consistent "read <description> <path>: <cause>" message.
func readMarkerFile(path, description string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s %s: %w", description, path, err)
	}
	return string(content), nil
}

// writeMarkerFile writes textContent back to path, or just announces the
// change when dryRun is set.
func writeMarkerFile(path, textContent string, dryRun bool) error {
	if dryRun {
		fmt.Println("would update", path)
		return nil
	}
	return os.WriteFile(path, []byte(textContent), 0o644)
}

// missingMarkerErr reports that fileDesc does not contain marker.
func missingMarkerErr(fileDesc, marker string) error {
	return fmt.Errorf("%s is missing the %s marker", fileDesc, strings.TrimSpace(marker))
}

func updateModuleCatalog(project, namespace, module string, dryRun bool) error {
	path := filepath.Join(project, "src", "Desktop", "App.xaml.cs")
	textContent, err := readMarkerFile(path, "module catalog host")
	if err != nil {
		return err
	}
	registration := "        moduleCatalog.AddModule<" + namespace + "." + module + "Module>();"
	if strings.Contains(textContent, registration) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:modules") {
		return missingMarkerErr("App.xaml.cs", "// aspgen:modules")
	}
	using := "using " + namespace + ";\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	textContent = strings.Replace(textContent, "        // aspgen:modules", "        // aspgen:modules\n"+registration, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

func updateFeatureHost(project, namespace, feature string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Program.cs")
	textContent, err := readMarkerFile(path, "feature host")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".WebApi.Features." + feature + ";\n"
	call := "app.Map" + feature + "Endpoints();"
	if strings.Contains(textContent, call) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:features") {
		return missingMarkerErr("Program.cs", "// aspgen:features")
	}
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	textContent = strings.Replace(textContent, "// aspgen:features", "// aspgen:features\n"+call, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateBlazorServiceHost registers a Renoir application service or repository
// implementation with DryIoc-free ASP.NET Core DI in the AppBlazor host.
func updateBlazorServiceHost(project, appBlazorDir string, usings []string, registration string, dryRun bool) error {
	path := filepath.Join(project, "src", appBlazorDir, "Program.cs")
	textContent, err := readMarkerFile(path, "Blazor service host")
	if err != nil {
		return err
	}
	if strings.Contains(textContent, registration) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:services") {
		return missingMarkerErr("Program.cs", "// aspgen:services")
	}
	for _, using := range usings {
		if !strings.Contains(textContent, using) {
			textContent = using + textContent
		}
	}
	textContent = strings.Replace(textContent, "// aspgen:services", "// aspgen:services\n"+registration, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateApplicationServiceHost registers a cqrs-tier aggregate's CrudService
// with the headless WebApi host's own DI container, in
// {{.Project}}.Application's DependencyInjection.cs. Unlike dm tier (which
// has no host to wire into, Phase 5) and legacy Renoir (which wires into
// AppBlazor's Program.cs via updateBlazorServiceHost), cqrs tier's vertical
// slices call through the CrudService, so it must be registered here.
func updateApplicationServiceHost(project, applicationDir, registration string, dryRun bool) error {
	path := filepath.Join(project, "src", applicationDir, "DependencyInjection.cs")
	textContent, err := readMarkerFile(path, "Application DI host")
	if err != nil {
		return err
	}
	if strings.Contains(textContent, registration) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:services") {
		return missingMarkerErr("DependencyInjection.cs", "// aspgen:services")
	}
	textContent = strings.Replace(textContent, "// aspgen:services", "// aspgen:services\n"+registration, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateInfrastructureRepositoryHost registers a cqrs-tier repository
// implementation with cqrs-tier DI in {{.Project}}.Infrastructure's
// DependencyInjection.cs. Only cqrs-tier contexts have a host to wire a
// repository into; dm-tier stays headless until Phase 5 (`add repository`
// still renders the repository files there, just without this wiring step).
func updateInfrastructureRepositoryHost(project, infrastructureDir string, usings []string, registration string, dryRun bool) error {
	path := filepath.Join(project, "src", infrastructureDir, "DependencyInjection.cs")
	textContent, err := readMarkerFile(path, "Infrastructure DI host")
	if err != nil {
		return err
	}
	if strings.Contains(textContent, registration) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:repositories") {
		return missingMarkerErr("DependencyInjection.cs", "// aspgen:repositories")
	}
	for _, using := range usings {
		if !strings.Contains(textContent, using) {
			textContent = using + textContent
		}
	}
	textContent = strings.Replace(textContent, "// aspgen:repositories", "// aspgen:repositories\n"+registration, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

func updateEntityDbContext(project, namespace, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")
	textContent, err := readMarkerFile(path, "database context")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".Domain.Entities;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:entities") {
		return missingMarkerErr("AppDbContext.cs", "// aspgen:entities")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

func updateEntityDbContextLocal(project, namespace, entity string, _ []Property, dryRun bool) error {
	path := filepath.Join(project, "src", "Infrastructure", "Persistence", "AppDbContext.cs")
	textContent, err := readMarkerFile(path, "local database context")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".Domain.Entities;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:entities") {
		return missingMarkerErr("local AppDbContext.cs", "    // aspgen:entities")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

func updateSimpleDbContext(project, namespace, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs")
	textContent, err := readMarkerFile(path, "simple database context")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".WebApi.Models;\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:entities") {
		return missingMarkerErr("simple AppDbContext.cs", "    // aspgen:entities")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateSimpleDbContextRelations inserts one HasOne/WithMany/HasForeignKey
// fluent configuration line per many-to-one relation declared on entity into
// the simple AppDbContext's OnModelCreating override.
func updateSimpleDbContextRelations(project, entity string, relations []Relation, dryRun bool) error {
	if len(relations) == 0 {
		return nil
	}
	path := filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs")
	textContent, err := readMarkerFile(path, "simple database context")
	if err != nil {
		return err
	}
	changed := false
	for _, rel := range relations {
		line := "        modelBuilder.Entity<" + entity + ">().HasOne(x => x." + rel.Name + ").WithMany().HasForeignKey(x => x." + rel.FKProperty + ");"
		if strings.Contains(textContent, line) {
			continue
		}
		if !strings.Contains(textContent, "        // aspgen:relations") {
			return missingMarkerErr("simple AppDbContext.cs", "// aspgen:relations")
		}
		textContent = strings.Replace(textContent, "        // aspgen:relations", "        // aspgen:relations\n"+line, 1)
		changed = true
	}
	if !changed {
		return nil
	}
	return writeMarkerFile(path, textContent, dryRun)
}

// updateContextDbContext registers a context-nested ar-tier entity's DbSet
// with the AppDbContext, mirroring updateSimpleDbContext but importing the
// entity's context-scoped Models namespace instead of the flat one.
func updateContextDbContext(project, namespace, contextName, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Data", "AppDbContext.cs")
	textContent, err := readMarkerFile(path, "context database context")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".WebApi.Models." + contextName + ";\n"
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	property := "    public DbSet<" + entity + "> " + entity + "s => Set<" + entity + ">();"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:entities") {
		return missingMarkerErr("context AppDbContext.cs", "// aspgen:entities")
	}
	textContent = strings.Replace(textContent, "    // aspgen:entities", "    // aspgen:entities\n"+property, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateContextFeatureHost registers a context-nested ar-tier entity's
// Minimal API endpoints with Program.cs, mirroring updateFeatureHost but
// importing the entity's context-scoped Features namespace.
func updateContextFeatureHost(project, namespace, contextName, entity string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Program.cs")
	textContent, err := readMarkerFile(path, "feature host")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".WebApi.Features." + contextName + "." + entity + ";\n"
	call := "app.Map" + entity + "Endpoints();"
	if strings.Contains(textContent, call) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:features") {
		return missingMarkerErr("Program.cs", "// aspgen:features")
	}
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	textContent = strings.Replace(textContent, "// aspgen:features", "// aspgen:features\n"+call, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateInverseNavigation adds a read-only inverse collection navigation
// property for childEntity onto the parent's already-generated model,
// domain entity, or aggregate class at path, guarded by the
// // aspgen:navigation marker.
func updateInverseNavigation(path, description, childEntity string, dryRun bool) error {
	textContent, err := readMarkerFile(path, description)
	if err != nil {
		return err
	}
	property := "    public ICollection<" + childEntity + "> " + childEntity + "s { get; set; } = [];"
	if strings.Contains(textContent, property) {
		return nil
	}
	if !strings.Contains(textContent, "    // aspgen:navigation") {
		return missingMarkerErr(description, "// aspgen:navigation")
	}
	textContent = strings.Replace(textContent, "    // aspgen:navigation", "    // aspgen:navigation\n"+property, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateWpfDetailsChildren adds a read-only reverse-relation child grid to
// an already-rendered {targetEntity}DetailsView.xaml/{targetEntity}
// DetailsViewModel.cs, for a NEW relation just declared on sourceEntity
// pointing back at targetEntity. The target's Details page is always
// rendered before this relation exists (a relation target must already be
// in the manifest before anything can reference it), so this always runs as
// a retrofit patch, never at the target's own initial render. The source
// entity's store is resolved via Prism's ContainerLocator instead of
// constructor injection, since the target ViewModel's constructor has
// already been rendered - no signature patching required.
func updateWpfDetailsChildren(project, projectName, targetEntity, sourceEntity, fkProperty string, dryRun bool) error {
	displayName := sourceEntity + "s"
	viewModelPath := filepath.Join(project, "src", "Desktop", "Modules", targetEntity, "ViewModels", targetEntity+"DetailsViewModel.cs")
	content, err := readPatchFile(viewModelPath)
	if err != nil {
		return err
	}
	collectionProperty := "    public ObservableCollection<" + sourceEntity + "Row> " + displayName + " { get; } = [];"
	if strings.Contains(content, collectionProperty) {
		return nil
	}
	usingServices := "using " + projectName + ".Desktop.Modules." + sourceEntity + ".Services;\n"
	usingModels := "using " + projectName + ".Desktop.Modules." + sourceEntity + ".Models;\n"
	if !strings.Contains(content, usingServices) {
		content = usingServices + content
	}
	if !strings.Contains(content, usingModels) {
		content = usingModels + content
	}
	var ok bool
	content, ok = insertBeforeMarker(content, "    // aspgen:childrenCollections", collectionProperty)
	if !ok {
		return patchErr(targetEntity+"DetailsViewModel.cs", "// aspgen:childrenCollections")
	}
	loadLines := "        " + displayName + ".Clear();\n" +
		"        foreach (var child in Prism.Ioc.ContainerLocator.Current.Resolve<I" + sourceEntity + "Store>().GetAll().Where(x => x." + fkProperty + " == id)) " + displayName + ".Add(child);"
	content, ok = insertBeforeMarker(content, "        // aspgen:childrenLoads", loadLines)
	if !ok {
		return patchErr(targetEntity+"DetailsViewModel.cs", "// aspgen:childrenLoads")
	}
	if err := writePatchFile(viewModelPath, content, dryRun); err != nil {
		return err
	}

	viewPath := filepath.Join(project, "src", "Desktop", "Modules", targetEntity, "Views", targetEntity+"DetailsView.xaml")
	viewContent, err := readPatchFile(viewPath)
	if err != nil {
		return err
	}
	childGrid := "                <StackPanel Margin=\"0,16,0,0\">\n" +
		"                    <TextBlock FontWeight=\"SemiBold\" Text=\"" + displayName + "\" />\n" +
		"                    <DataGrid Margin=\"0,4,0,0\" MaxHeight=\"220\" AutoGenerateColumns=\"True\" IsReadOnly=\"True\" ItemsSource=\"{Binding " + displayName + "}\" />\n" +
		"                </StackPanel>"
	viewContent, ok = insertBeforeMarker(viewContent, "                <!-- aspgen:children -->", childGrid)
	if !ok {
		return patchErr(targetEntity+"DetailsView.xaml", "<!-- aspgen:children -->")
	}
	return writePatchFile(viewPath, viewContent, dryRun)
}

// updateMvcDetailsChildren adds a read-only reverse-relation child table to
// an already-rendered {targetEntity}Controller.cs/Views/{targetEntity}/
// Details.cshtml, for a NEW relation just declared on sourceEntity pointing
// back at targetEntity. Unlike the WPF case, the Controller's primary
// constructor is patched directly (via the /* aspgen:childrenParams */
// marker) since ASP.NET Core resolves it fresh per request - there's no
// already-constructed instance to work around.
func updateMvcDetailsChildren(project, projectName, targetEntity, sourceEntity, fkProperty string, sourceProperties []Property, dryRun bool) error {
	displayName := sourceEntity + "s"
	controllerPath := filepath.Join(project, "src", projectName+".WebMvc", "Controllers", targetEntity+"Controller.cs")
	content, err := readPatchFile(controllerPath)
	if err != nil {
		return err
	}
	loadLine := "        ViewBag." + displayName + " = (await " + camel(sourceEntity) + "Service.GetAllAsync(cancellationToken)).Where(x => x." + fkProperty + " == id).ToList();"
	if strings.Contains(content, loadLine) {
		return nil
	}
	if !strings.Contains(content, "/* aspgen:childrenParams */") {
		return patchErr(targetEntity+"Controller.cs", "/* aspgen:childrenParams */")
	}
	ctorParam := ", " + sourceEntity + "CrudService " + camel(sourceEntity) + "Service"
	content = strings.Replace(content, "/* aspgen:childrenParams */", ctorParam+"/* aspgen:childrenParams */", 1)
	content, ok := insertBeforeMarker(content, "        // aspgen:childrenLoads", loadLine)
	if !ok {
		return patchErr(targetEntity+"Controller.cs", "// aspgen:childrenLoads")
	}
	if err := writePatchFile(controllerPath, content, dryRun); err != nil {
		return err
	}

	viewPath := filepath.Join(project, "src", projectName+".WebMvc", "Views", targetEntity, "Details.cshtml")
	viewContent, err := readPatchFile(viewPath)
	if err != nil {
		return err
	}
	var header, cells strings.Builder
	for _, p := range sourceProperties {
		if p.RelationTarget != "" {
			continue
		}
		header.WriteString("            <th>" + p.DisplayName + "</th>\n")
		cells.WriteString("            <td>@child." + p.Name + "</td>\n")
	}
	table := "<h3>" + displayName + "</h3>\n" +
		"<table class=\"table\">\n" +
		"    <thead>\n        <tr>\n" + header.String() + "        </tr>\n    </thead>\n" +
		"    <tbody>\n    @foreach (var child in (List<" + sourceEntity + "View>)ViewBag." + displayName + ")\n    {\n" +
		"        <tr>\n" + cells.String() + "        </tr>\n    }\n    </tbody>\n</table>"
	viewContent, ok = insertBeforeMarker(viewContent, "<!-- aspgen:children -->", table)
	if !ok {
		return patchErr(targetEntity+"/Details.cshtml", "<!-- aspgen:children -->")
	}
	return writePatchFile(viewPath, viewContent, dryRun)
}

// updateBlazorDetailsChildren adds a read-only reverse-relation child table
// to an already-rendered {targetEntity}Details.razor, for a NEW relation
// just declared on sourceEntity (in the same bounded context, relations are
// same-context only) pointing back at targetEntity. Reuses the page's
// existing @inject HttpClient - Blazor Details pages already call the
// related entity's own REST endpoint directly for forward relations, so the
// reverse direction needs no new @inject at all, just new @code lines.
func updateBlazorDetailsChildren(project, projectName, contextName, targetEntity, sourceEntity, fkProperty string, dryRun bool) error {
	displayName := sourceEntity + "s"
	path := filepath.Join(project, "src", projectName+".AppBlazor", "Components", "Pages", contextName, targetEntity+"Details.razor")
	content, err := readPatchFile(path)
	if err != nil {
		return err
	}
	field := "    private List<" + sourceEntity + "View> " + displayName + " = [];"
	if strings.Contains(content, field) {
		return nil
	}
	content, ok := insertBeforeMarker(content, "    // aspgen:childrenFields", field)
	if !ok {
		return patchErr(targetEntity+"Details.razor", "// aspgen:childrenFields")
	}
	loadLine := "        " + displayName + " = (await Http.GetFromJsonAsync<List<" + sourceEntity + "View>>(\"/api/" + kebab(contextName) + "/" + kebab(sourceEntity) + "\") ?? []).Where(x => x." + fkProperty + " == Id).ToList();"
	content, ok = insertBeforeMarker(content, "        // aspgen:childrenLoads", loadLine)
	if !ok {
		return patchErr(targetEntity+"Details.razor", "// aspgen:childrenLoads")
	}
	markup := "    <h3>" + displayName + "</h3>\n" +
		"    <table class=\"table\">\n        <tbody>\n        @foreach (var child in " + displayName + ")\n        {\n            <tr>@child.ToString()</tr>\n        }\n        </tbody>\n    </table>"
	content, ok = insertBeforeMarker(content, "    @* aspgen:children *@", markup)
	if !ok {
		return patchErr(targetEntity+"Details.razor", "@* aspgen:children *@")
	}
	return writePatchFile(path, content, dryRun)
}

// hasWebApiHost reports whether project has a --context/--arch engine WebApi
// host (cqrs/es tiers only; ar/dm stay headless class libraries).
func hasWebApiHost(project string) bool {
	return exists(filepath.Join(project, "src", "WebApi", "Program.cs"))
}

// hasBlazorHost reports whether project has a --context/--arch engine
// AppBlazor host attached via -ui blazor.
func hasBlazorHost(project, projectName string) bool {
	return exists(filepath.Join(project, "src", projectName+".AppBlazor", "Program.cs"))
}

// attachSpaHost wires `-ui spa` onto an existing cqrs/es-tier WebApi host:
// OpenAPI/Scalar discovery plus a permissive local-dev CORS policy, so a
// separately-hosted SPA can call the API. It does not scaffold any actual
// frontend project. Idempotent: safe to call again (e.g. from both `new
// ... -ui spa` and a later `add ui --framework spa`).
func attachSpaHost(project, projectName string, dryRun bool) error {
	programPath := filepath.Join(project, "src", "WebApi", "Program.cs")
	csprojPath := filepath.Join(project, "src", "WebApi", "WebApi.csproj")
	if dryRun {
		fmt.Println("would update", programPath)
		fmt.Println("would update", csprojPath)
		return nil
	}
	if err := attachSpaProgram(programPath, projectName); err != nil {
		return err
	}
	return attachSpaCsproj(csprojPath)
}

func attachSpaProgram(path, projectName string) error {
	textContent, err := readMarkerFile(path, "WebApi host")
	if err != nil {
		return err
	}
	const corsPolicy = `builder.Services.AddCors(options => options.AddPolicy("Spa", policy => policy.WithOrigins("http://localhost:5173").AllowAnyHeader().AllowAnyMethod()));`
	if strings.Contains(textContent, corsPolicy) {
		return nil
	}
	newline := "\n"
	if strings.Contains(textContent, "\r\n") {
		newline = "\r\n"
	}
	if !strings.Contains(textContent, "using Scalar.AspNetCore;") {
		textContent = "using Scalar.AspNetCore;" + newline + textContent
	}
	const buildAnchor = "builder.Services.AddApplication();"
	if !strings.Contains(textContent, buildAnchor) {
		return missingMarkerErr("Program.cs", buildAnchor)
	}
	textContent = strings.Replace(textContent, buildAnchor,
		"builder.Services.AddOpenApi();"+newline+corsPolicy+newline+buildAnchor, 1)
	const appAnchor = `app.MapHealthChecks("/health");`
	if !strings.Contains(textContent, appAnchor) {
		return missingMarkerErr("Program.cs", appAnchor)
	}
	textContent = strings.Replace(textContent, appAnchor,
		"app.MapOpenApi();"+newline+
			fmt.Sprintf("app.MapScalarApiReference(options => options.WithTitle(%q));", projectName+" API")+newline+
			`app.UseCors("Spa");`+newline+appAnchor, 1)
	return writeMarkerFile(path, textContent, false)
}

func attachSpaCsproj(path string) error {
	textContent, err := readMarkerFile(path, "WebApi host project file")
	if err != nil {
		return err
	}
	if strings.Contains(textContent, `PackageReference Include="Scalar.AspNetCore"`) {
		return nil
	}
	if !strings.Contains(textContent, "</Project>") {
		return missingMarkerErr("WebApi.csproj", "</Project>")
	}
	newline := "\n"
	if strings.Contains(textContent, "\r\n") {
		newline = "\r\n"
	}
	insertion := "  <ItemGroup>" + newline +
		"    <PackageReference Include=\"Microsoft.AspNetCore.OpenApi\" Version=\"10.0.10\" />" + newline +
		"    <PackageReference Include=\"Microsoft.OpenApi\" Version=\"2.7.5\" />" + newline +
		"    <PackageReference Include=\"Scalar.AspNetCore\" Version=\"2.16.16\" />" + newline +
		"  </ItemGroup>" + newline + "</Project>"
	textContent = strings.Replace(textContent, "</Project>", insertion, 1)
	return writeMarkerFile(path, textContent, false)
}
