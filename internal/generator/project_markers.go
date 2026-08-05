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

func updateSeedHost(project, namespace, backend string, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Program.cs")
	textContent, err := readMarkerFile(path, "seed host")
	if err != nil {
		return err
	}
	call := "await DatabaseSeeder.SeedAsync(app.Services);"
	if strings.Contains(textContent, call) {
		return nil
	}
	if !strings.Contains(textContent, "// aspgen:seed") {
		return missingMarkerErr("Program.cs", "// aspgen:seed")
	}
	if backend == "ddd" {
		using := "using " + namespace + ".WebApi.Seeding;\n"
		if !strings.Contains(textContent, using) {
			textContent = using + textContent
		}
	}
	textContent = strings.Replace(textContent, "// aspgen:seed", "// aspgen:seed\n"+call, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

func updateSeedFile(project, namespace, backend, entity string, properties []Property, count int, dryRun bool) error {
	path := filepath.Join(project, "src", "WebApi", "Data", "DatabaseSeeder.cs")
	if backend == "ddd" {
		path = filepath.Join(project, "src", "WebApi", "Seeding", "DatabaseSeeder.cs")
	} else if backend == "ddd-local" {
		path = filepath.Join(project, "src", "Infrastructure", "Seeding", "DatabaseSeeder.cs")
	}
	textContent, err := readMarkerFile(path, "seed file")
	if err != nil {
		return err
	}
	using := "using " + namespace + ".WebApi.Models;\n"
	if backend == "ddd" {
		using = "using " + namespace + ".Domain.Entities;\n"
	} else if backend == "ddd-local" {
		using = "using " + namespace + ".Domain.Entities;\n"
	}
	if !strings.Contains(textContent, using) {
		textContent = using + textContent
	}
	if strings.Contains(textContent, "db."+entity+"s.AnyAsync") {
		return nil
	}
	block := renderSeedBlock(backend, entity, properties, count)
	if !strings.Contains(textContent, "    // aspgen:seed") {
		return missingMarkerErr("DatabaseSeeder.cs", "    // aspgen:seed")
	}
	textContent = strings.Replace(textContent, "    // aspgen:seed", "    // aspgen:seed\n"+block, 1)
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

func updateEntityDependencyInjection(project, namespace, entity string, dryRun bool) error {
	registrations := []struct {
		path, marker, line string
	}{
		{filepath.Join(project, "src", "Infrastructure", "DependencyInjection.cs"), "        // aspgen:repositories", "        services.AddScoped<Domain.Repositories.I" + entity + "Repository, Persistence." + entity + "Repository>();"},
	}
	for _, registration := range registrations {
		textContent, err := readMarkerFile(registration.path, "dependency injection file")
		if err != nil {
			return err
		}
		if strings.Contains(textContent, registration.line) {
			continue
		}
		if !strings.Contains(textContent, registration.marker) {
			return missingMarkerErr(registration.path, registration.marker)
		}
		textContent = strings.Replace(textContent, registration.marker, registration.marker+"\n"+registration.line, 1)
		if err := writeMarkerFile(registration.path, textContent, dryRun); err != nil {
			return err
		}
	}
	return nil
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

// updateRelatedGrid adds a read-only DataGrid displaying childEntity records
// related to parentEntity onto the parent's already-generated wpf-entity
// view, guarded by the <!-- aspgen:related --> marker.
func updateRelatedGrid(project, parentEntity, childEntity, theme string, dryRun bool) error {
	path := filepath.Join(project, "src", "Desktop", "Modules", parentEntity, "Views", parentEntity+"View.xaml")
	textContent, err := readMarkerFile(path, parentEntity+"View.xaml")
	if err != nil {
		return err
	}
	if strings.Contains(textContent, "Header=\""+childEntity+"s\"") {
		return nil
	}
	marker := "        <!-- aspgen:related -->"
	if !strings.Contains(textContent, marker) {
		return missingMarkerErr(parentEntity+"View.xaml", marker)
	}
	gridTag := "DataGrid"
	if theme == "wpfui" {
		gridTag = "ui:DataGrid"
	}
	block := "        <GroupBox Grid.Row=\"3\" Margin=\"0,12,0,0\" Header=\"" + childEntity + "s\">\n" +
		"            <" + gridTag + " ItemsSource=\"{Binding " + childEntity + "s}\" AutoGenerateColumns=\"True\" CanUserAddRows=\"False\" IsReadOnly=\"True\" />\n" +
		"        </GroupBox>"
	textContent = strings.Replace(textContent, marker, block+"\n"+marker, 1)
	return writeMarkerFile(path, textContent, dryRun)
}

// updateRelatedStore wires an injected childEntity store, a read-only
// display collection, and its population into the parent's already-generated
// wpf-entity view model, guarded by the aspgen:related* markers.
func updateRelatedStore(project, namespace, parentEntity, childEntity string, dryRun bool) error {
	path := filepath.Join(project, "src", "Desktop", "Modules", parentEntity, "ViewModels", parentEntity+"ViewModel.cs")
	textContent, err := readMarkerFile(path, parentEntity+"ViewModel.cs")
	if err != nil {
		return err
	}
	childCamel := camel(childEntity)
	field := "    private readonly I" + childEntity + "Store " + childCamel + "Store;"
	if strings.Contains(textContent, field) {
		return nil
	}
	usingMarker := "// aspgen:relatedUsings"
	if !strings.Contains(textContent, usingMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", usingMarker)
	}
	usings := "using " + namespace + ".Desktop.Modules." + childEntity + ".Services;\n" +
		"using " + namespace + ".Desktop.Modules." + childEntity + ".Models;\n" + usingMarker
	textContent = strings.Replace(textContent, usingMarker, usings, 1)

	storeMarker := "    // aspgen:relatedStores"
	if !strings.Contains(textContent, storeMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", storeMarker)
	}
	textContent = strings.Replace(textContent, storeMarker, field+"\n"+storeMarker, 1)

	paramMarker := "/* aspgen:relatedParams */"
	if !strings.Contains(textContent, paramMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", paramMarker)
	}
	textContent = strings.Replace(textContent, paramMarker, ", I"+childEntity+"Store "+childCamel+"Store"+paramMarker, 1)

	assignMarker := "        // aspgen:relatedAssignments"
	if !strings.Contains(textContent, assignMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", assignMarker)
	}
	textContent = strings.Replace(textContent, assignMarker, "        this."+childCamel+"Store = "+childCamel+"Store;\n"+assignMarker, 1)

	collectionMarker := "    // aspgen:relatedCollections"
	if !strings.Contains(textContent, collectionMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", collectionMarker)
	}
	textContent = strings.Replace(textContent, collectionMarker, "    public ObservableCollection<"+childEntity+"Row> "+childEntity+"s { get; } = [];\n"+collectionMarker, 1)

	loadMarker := "        // aspgen:relatedLoads"
	if !strings.Contains(textContent, loadMarker) {
		return missingMarkerErr(parentEntity+"ViewModel.cs", loadMarker)
	}
	load := "        " + childEntity + "s.Clear();\n        foreach (var item in " + childCamel + "Store.GetAll()) " + childEntity + "s.Add(item);\n" + loadMarker
	textContent = strings.Replace(textContent, loadMarker, load, 1)

	return writeMarkerFile(path, textContent, dryRun)
}

// hasWebApiHost reports whether project has a --context/--arch engine WebApi
// host (cqrs/es tiers only; ar/dm stay headless class libraries).
func hasWebApiHost(project string) bool {
	return exists(filepath.Join(project, "src", "WebApi", "Program.cs"))
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
