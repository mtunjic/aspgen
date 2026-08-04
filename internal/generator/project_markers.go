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
