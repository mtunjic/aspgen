package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProperties(t *testing.T) {
	properties, err := parseProperties([]string{"name:string", "age:int", "active:bool", "born:date"})
	if err != nil {
		t.Fatal(err)
	}
	if len(properties) != 4 || properties[2].CSharpType != "bool" || properties[3].CSharpType != "DateOnly" || properties[0].DisplayName != "Name" {
		t.Fatalf("unexpected properties: %#v", properties)
	}
}

func TestHumanizePropertyName(t *testing.T) {
	for input, expected := range map[string]string{"CustomerName": "Customer Name", "publishedDate": "Published Date", "APIKey": "Api Key", "order_id": "Order Id"} {
		if actual := humanize(input); actual != expected {
			t.Fatalf("humanize(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestParsePropertiesRejectsUnknownType(t *testing.T) {
	if _, err := parseProperties([]string{"name:money"}); err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestSeedCountOptions(t *testing.T) {
	seed, count, err := seedOption([]string{"--seed", "dummy", "200"})
	if err != nil || seed != "dummy" || count != 200 {
		t.Fatalf("unexpected spaced seed count: %q, %d, %v", seed, count, err)
	}
	seed, count, err = seedOption([]string{"--seed:dummy:25"})
	if err != nil || seed != "dummy" || count != 25 {
		t.Fatalf("unexpected compact seed count: %q, %d, %v", seed, count, err)
	}
}

func TestAggregateRejectsReservedIdentityProperty(t *testing.T) {
	if err := rejectAggregateReservedProperties([]Property{{Name: "Id"}}); err == nil {
		t.Fatal("expected aggregate identity property to be reserved")
	}
}

func TestNewAndIncrementalGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Demo")
	if err := Run([]string{"new", "Demo", "--app", "fullstack", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "active:bool", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "module", "Customers", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		".aspgen/manifest.json",
		"src/Domain/Entities/Customer.cs",
		"src/Domain/Entities/Order.cs",
		"src/Desktop/Modules/Customer/CustomerModule.cs",
		"src/Desktop/Modules/Customer/Views/CustomerView.xaml",
		"src/Desktop/Modules/Customer/ViewModels/CustomerViewModel.cs",
		"src/Desktop/Modules/Customers/CustomersModule.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing generated file %s: %v", path, err)
		}
	}
	app, err := os.ReadFile(filepath.Join(project, "src/Desktop/App.xaml.cs"))
	if err != nil || !strings.Contains(string(app), "moduleCatalog.AddModule<Demo.Desktop.Modules.Customer.CustomerModule>();") || !strings.Contains(string(app), "moduleCatalog.AddModule<Demo.Modules.Customers.CustomersModule>();") {
		t.Fatalf("module was not registered in App.xaml.cs: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if !contains(manifest.Components, "entity:Customer") || !contains(manifest.Components, "module:Customers") {
		t.Fatalf("manifest missing components: %#v", manifest.Components)
	}
	projectFile, err := os.ReadFile(filepath.Join(project, "src/Desktop/Demo.Desktop.csproj"))
	if err != nil || !strings.Contains(string(projectFile), `Compile Update="Modules/Customer/CustomerModule.cs"`) || !strings.Contains(string(projectFile), `Page Update="Modules/Customer/Views/CustomerView.xaml"`) {
		t.Fatalf("incremental WPF files were not registered in the project file: %v", err)
	}
}

func TestDottedProjectNameGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Commerce")
	if err := Run([]string{"new", "Markosoft.Commerce", "--app", "webapi", "--output", project}); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"project": "Markosoft.Commerce"`) {
		t.Fatalf("dotted project name was not recorded: %v", err)
	}
	program, err := os.ReadFile(filepath.Join(project, "src/WebApi/Program.cs"))
	if err != nil || !strings.Contains(string(program), "Markosoft.Commerce") {
		t.Fatalf("dotted namespace was not generated: %v", err)
	}
}

func TestWpfUIThemeGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "ThemedDemo")
	if err := Run([]string{"new", "ThemedDemo", "--app", "wpf", "--theme:wpfui", "--output", project}); err != nil {
		t.Fatal(err)
	}
	csproj, err := os.ReadFile(filepath.Join(project, "src/Desktop/ThemedDemo.Desktop.csproj"))
	if err != nil || !strings.Contains(string(csproj), `<PackageReference Include="WPF-UI" Version="4.3.0" />`) {
		t.Fatalf("WPF-UI package was not added: %v", err)
	}
	app, err := os.ReadFile(filepath.Join(project, "src/Desktop/App.xaml"))
	if err != nil || !strings.Contains(string(app), `<ui:ThemesDictionary Theme="Dark" />`) || !strings.Contains(string(app), `<ui:ControlsDictionary />`) {
		t.Fatalf("WPF-UI resources were not added: %v", err)
	}
	shell, err := os.ReadFile(filepath.Join(project, "src/Desktop/Shell.xaml"))
	if err != nil || !strings.Contains(string(shell), "ui:FluentWindow") || !strings.Contains(string(shell), "ui:NavigationView") || !strings.Contains(string(shell), "MainRegion") {
		t.Fatalf("WPF-UI shell was not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "src/Desktop/Navigation/NavigationRegistry.cs")); err != nil {
		t.Fatalf("WPF-UI navigation registry was not generated: %v", err)
	}
	shellText := string(shell)
	for _, expected := range []string{"ui:TitleBar", "ui:NavigationView", "ui:AutoSuggestBox", "Search modules", "ThemeButtonText", "Settings", "Alt", "OemComma", "DarkTheme24", "ApplicationBackgroundBrush"} {
		if !strings.Contains(shellText, expected) {
			t.Fatalf("WPF-UI shell is missing %q", expected)
		}
	}
	dashboard, err := os.ReadFile(filepath.Join(project, "src/Desktop/Views/DashboardView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"LinearGradientBrush", "Your modules", "WrapPanel", "ui:CardAction"} {
		if !strings.Contains(string(dashboard), expected) {
			t.Fatalf("WPF-UI dashboard is missing %q", expected)
		}
	}
	navigation, err := os.ReadFile(filepath.Join(project, "src/Desktop/Navigation/NavigationRegistry.cs"))
	if err != nil || !strings.Contains(string(navigation), "NavigationViewItemSeparator") || !strings.Contains(string(navigation), "Content = \"Home\"") {
		t.Fatalf("WPF-UI navigation hierarchy was not generated: %v", err)
	}
	for _, path := range []string{
		"src/Desktop/Views/DashboardView.xaml",
		"src/Desktop/Views/DashboardView.xaml.cs",
		"src/Desktop/Views/DashboardViewModel.cs",
		"src/Desktop/Views/SettingsView.xaml",
		"src/Desktop/Views/SettingsView.xaml.cs",
		"src/Desktop/Views/SettingsViewModel.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, path)); err != nil {
			t.Fatalf("WPF-UI shell view was not generated: %s: %v", path, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(project, "src/Desktop/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"CalendarDatePicker", "InfoBar", "Ctrl+N", "Ctrl+S"} {
		if !strings.Contains(string(readme), expected) {
			t.Fatalf("WPF-UI README is missing %q", expected)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"theme:wpfui"`) {
		t.Fatalf("WPF-UI theme was not recorded in the manifest: %v", err)
	}
	if err := Run([]string{"add", "entity", "Event", "title:string", "count:int", "price:decimal", "eventDate:date", "startsAt:datetime", "active:bool", "externalId:guid", "--project", project}); err != nil {
		t.Fatal(err)
	}
	view, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Event/Views/EventView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	viewText := string(view)
	for _, expected := range []string{
		"ui:DataGrid",
		"ui:Button",
		"ui:TextBlock",
		"ui:TextBox",
		"ui:NumberBox",
		"ControlStrokeColorDefaultBrush",
		"TextFillColorSecondaryBrush",
		"ui:CalendarDatePicker",
		`Date="{Binding Form.EventDate`,
		"ui:TimePicker",
		`SelectedTime="{Binding Form.StartsAtTime`,
		"ui:ToggleSwitch",
	} {
		if !strings.Contains(viewText, expected) {
			t.Fatalf("WPF-UI entity control was not generated: missing %q", expected)
		}
	}
	if strings.Contains(viewText, `SelectedDate="{Binding Form.EventDate`) {
		t.Fatal("WPF-UI CalendarDatePicker must bind Date rather than SelectedDate")
	}
	menu, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Event/Views/EventMenuView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(menu), `Icon="{ui:SymbolIcon Table24}"`) {
		t.Fatal("WPF-UI module menu icon was not generated")
	}
	module, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Event/EventModule.cs"))
	if err != nil || !strings.Contains(string(module), `navigation.Register("Event", "EventView"`) {
		t.Fatalf("WPF-UI module was not registered with NavigationView: %v", err)
	}
}

func TestDddBackendEntityGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "DddApi")
	if err := Run([]string{"new", "DddApi", "--app", "fullstack", "--backend:ddd", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Person", "name:string", "age:int", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"src/Application/Features/Person/CreatePersonCommand.cs",
		"src/Application/Features/Person/CreatePersonHandler.cs",
		"src/Application/Features/Person/GetPersonsQuery.cs",
		"src/Application/Features/Person/GetPersonsHandler.cs",
		"src/Application/Features/Person/GetPersonByIdQuery.cs",
		"src/Application/Features/Person/GetPersonByIdHandler.cs",
		"src/Application/Features/Person/UpdatePersonCommand.cs",
		"src/Application/Features/Person/UpdatePersonHandler.cs",
		"src/Application/Features/Person/DeletePersonCommand.cs",
		"src/Application/Features/Person/DeletePersonHandler.cs",
		"src/Infrastructure/Persistence/PersonRepository.cs",
		"src/WebApi/Features/Person/PersonEndpoints.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing DDD backend file %s: %v", path, err)
		}
	}
	context, err := os.ReadFile(filepath.Join(project, "src/Infrastructure/Persistence/AppDbContext.cs"))
	if err != nil || !strings.Contains(string(context), "DbSet<Person> Persons") {
		t.Fatalf("entity was not added to AppDbContext: %v", err)
	}
	program, err := os.ReadFile(filepath.Join(project, "src/WebApi/Program.cs"))
	if err != nil || !strings.Contains(string(program), "app.MapPersonEndpoints();") {
		t.Fatalf("DDD endpoints were not registered: %v", err)
	}
	endpoints, err := os.ReadFile(filepath.Join(project, "src/WebApi/Features/Person/PersonEndpoints.cs"))
	if err != nil || !strings.Contains(string(endpoints), "IHandler<CreatePersonCommand, PersonResponse>") || !strings.Contains(string(endpoints), "IHandler<DeletePersonCommand, bool>") {
		t.Fatalf("CRUD endpoints should use CQRS handlers: %v", err)
	}
	applicationDI, err := os.ReadFile(filepath.Join(project, "src/Application/DependencyInjection.cs"))
	if err != nil || strings.Contains(string(applicationDI), "PersonService") {
		t.Fatalf("DDD generation left a stale PersonService registration: %v", err)
	}
}

func TestLocalDddWpfGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "LocalDesktop")
	if err := Run([]string{"new", "LocalDesktop", "--app", "wpf", "--backend", "ddd", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "paid:bool", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"src/Domain/Domain.csproj",
		"src/Application/Application.csproj",
		"src/Infrastructure/Infrastructure.csproj",
		"src/Infrastructure/Persistence/AppDbContext.cs",
		"src/Desktop/LocalDesktop.Desktop.csproj",
		"src/Domain/Entities/Order.cs",
		"src/Desktop/Modules/Order/OrderModule.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing local DDD WPF file %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project, "src/WebApi")); err == nil {
		t.Fatal("local DDD WPF should not generate a WebApi directory")
	}
	solution, err := os.ReadFile(filepath.Join(project, "LocalDesktop.sln"))
	if err != nil || strings.Contains(string(solution), "WebApi.csproj") || !strings.Contains(string(solution), "Infrastructure.csproj") {
		t.Fatalf("local DDD WPF solution has the wrong project graph: %v", err)
	}
}

func TestSimpleProfileGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "SimpleApp")
	if err := Run([]string{"new", "SimpleApp", "--app", "fullstack", "--simple", "--theme:wpfui", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Person", "name:string", "age:int", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"SimpleApp.sln",
		"src/WebApi/WebApi.csproj",
		"src/WebApi/Data/AppDbContext.cs",
		"src/WebApi/Models/Person.cs",
		"src/WebApi/Features/Person/PersonEndpoints.cs",
		"src/Desktop/Modules/Person/PersonModule.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing simple profile file %s: %v", path, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"backend:simple"`) {
		t.Fatalf("simple backend was not recorded: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(project, "README.md"))
	if err != nil || !strings.Contains(string(readme), "dotnet run --project .\\src\\WebApi\\WebApi.csproj") || !strings.Contains(string(readme), "default database is SQLite") {
		t.Fatalf("simple README is missing build/run instructions: %v", err)
	}
	store, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Person/Services/PersonStore.cs"))
	if err != nil || !strings.Contains(string(store), "GetFromJsonAsync") || !strings.Contains(string(store), `"/api/person"`) {
		t.Fatalf("simple WPF store should use the generated API: %v", err)
	}
	module, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Person/PersonModule.cs"))
	if err != nil || !strings.Contains(string(module), "ASPGENT_API_URL") {
		t.Fatalf("simple WPF module should register configurable API HTTP: %v", err)
	}
}

func TestDummySeedGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "SeedApp")
	if err := Run([]string{"new", "SeedApp", "--app", "fullstack", "--simple", "--seed:dummy", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Person", "name:string", "age:int", "born:date", "active:bool", "--project", project}); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join(project, "src/WebApi/Data/DatabaseSeeder.cs"))
	if err != nil || !strings.Contains(string(seed), "new Person") || !strings.Contains(string(seed), "sample 1") || !strings.Contains(string(seed), "new DateOnly") {
		t.Fatalf("simple dummy seed was not generated: %v", err)
	}
	program, err := os.ReadFile(filepath.Join(project, "src/WebApi/Program.cs"))
	if err != nil || !strings.Contains(string(program), "DatabaseSeeder.SeedAsync") {
		t.Fatalf("simple startup seeding was not wired: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"seed:dummy:3"`) {
		t.Fatalf("seed profile was not recorded: %v", err)
	}

	ddd := filepath.Join(t.TempDir(), "DddSeedApp")
	if err := Run([]string{"new", "DddSeedApp", "--app", "webapi", "--backend", "ddd", "--seed", "dummy", "--output", ddd}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Person", "name:string", "age:int", "--project", ddd}); err != nil {
		t.Fatal(err)
	}
	dddSeed, err := os.ReadFile(filepath.Join(ddd, "src/WebApi/Seeding/DatabaseSeeder.cs"))
	if err != nil || !strings.Contains(string(dddSeed), "new Person(") || !strings.Contains(string(dddSeed), "db.Persons.AnyAsync") {
		t.Fatalf("DDD dummy seed was not generated: %v", err)
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Demo")
	if err := Run([]string{"new", "Demo", "--app", "webapi", "--output", project, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project); err == nil {
		t.Fatal("dry-run created project")
	}
}

func TestDiscoverProjectRootFromNestedDirectory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "DiscoverDemo")
	if err := Run([]string{"new", "DiscoverDemo", "--app", "webapi", "--output", project}); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(project, "src", "WebApi")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	discovered, err := discoverProjectRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if discovered != project {
		t.Fatalf("discovered %q, want %q", discovered, project)
	}
}

func TestWebAPIProfileGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "ApiDemo")
	if err := Run([]string{"new", "ApiDemo", "--app", "webapi", "--output", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"ApiDemo.sln",
		"README.md",
		"src/Domain/Domain.csproj",
		"src/Application/Application.csproj",
		"src/Infrastructure/Infrastructure.csproj",
		"src/Infrastructure/Persistence/AppDbContext.cs",
		"src/WebApi/WebApi.csproj",
		"src/WebApi/appsettings.json",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing Web API file %s: %v", path, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(project, "README.md"))
	if err != nil || !strings.Contains(string(readme), "dotnet build .\\ApiDemo.sln") || !strings.Contains(string(readme), "localhost:5000/health") {
		t.Fatalf("generated API README is missing build/run instructions: %v", err)
	}
}

func TestDatabaseProviderGeneration(t *testing.T) {
	sqliteProject := filepath.Join(t.TempDir(), "SqliteApi")
	if err := Run([]string{"new", "SqliteApi", "--app", "webapi", "--output", sqliteProject}); err != nil {
		t.Fatal(err)
	}
	sqliteProjectFile, err := os.ReadFile(filepath.Join(sqliteProject, "src/Infrastructure/Infrastructure.csproj"))
	if err != nil || !strings.Contains(string(sqliteProjectFile), "Microsoft.EntityFrameworkCore.Sqlite") || !strings.Contains(string(sqliteProjectFile), "SQLitePCLRaw.lib.e_sqlite3") {
		t.Fatalf("SQLite provider was not generated: %v", err)
	}
	sqliteProgram, err := os.ReadFile(filepath.Join(sqliteProject, "src/Infrastructure/DependencyInjection.cs"))
	if err != nil || !strings.Contains(string(sqliteProgram), "UseSqlite") {
		t.Fatalf("SQLite configuration was not generated: %v", err)
	}
	sqliteSettings, err := os.ReadFile(filepath.Join(sqliteProject, "src/WebApi/appsettings.json"))
	if err != nil || !strings.Contains(string(sqliteSettings), "Data Source=sqlite-api.db") {
		t.Fatalf("SQLite connection string was not generated: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(sqliteProject, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"database:sqlite"`) {
		t.Fatalf("SQLite provider was not recorded: %v", err)
	}

	postgresProject := filepath.Join(t.TempDir(), "PostgresApi")
	if err := Run([]string{"new", "PostgresApi", "--app", "webapi", "--database:postgres", "--output", postgresProject}); err != nil {
		t.Fatal(err)
	}
	postgresProjectFile, err := os.ReadFile(filepath.Join(postgresProject, "src/Infrastructure/Infrastructure.csproj"))
	if err != nil || !strings.Contains(string(postgresProjectFile), "Npgsql.EntityFrameworkCore.PostgreSQL") || strings.Contains(string(postgresProjectFile), "EntityFrameworkCore.Sqlite") {
		t.Fatalf("PostgreSQL provider was not generated: %v", err)
	}
	postgresProgram, err := os.ReadFile(filepath.Join(postgresProject, "src/Infrastructure/DependencyInjection.cs"))
	if err != nil || !strings.Contains(string(postgresProgram), "UseNpgsql") {
		t.Fatalf("PostgreSQL configuration was not generated: %v", err)
	}

	if err := Run([]string{"new", "InvalidDatabase", "--app", "webapi", "--database", "mysql", "--output", filepath.Join(t.TempDir(), "InvalidDatabase")}); err == nil {
		t.Fatal("expected unsupported database error")
	}
}

func TestWebAPIFeatureGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "FeatureDemo")
	if err := Run([]string{"new", "FeatureDemo", "--app", "webapi", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "feature", "CreateCustomer", "name:string", "age:int", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"src/Application/Features/CreateCustomer/CreateCustomerRequest.cs",
		"src/Application/Features/CreateCustomer/CreateCustomerHandler.cs",
		"src/Application/Features/CreateCustomer/CreateCustomerValidator.cs",
		"src/WebApi/Features/CreateCustomer/CreateCustomerEndpoints.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing feature file %s: %v", path, err)
		}
	}
	program, err := os.ReadFile(filepath.Join(project, "src/WebApi/Program.cs"))
	if err != nil || !strings.Contains(string(program), "app.MapCreateCustomerEndpoints();") {
		t.Fatalf("feature endpoint was not registered: %v", err)
	}
}

func TestRenoirProfileGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RenoirDemo")
	if err := Run([]string{"new", "RenoirDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"RenoirDemo.sln",
		"src/RenoirDemo.DomainModel/BaseEntity.cs",
		"src/RenoirDemo.Application/Settings/RenoirSettings.cs",
		"src/RenoirDemo.Persistence/RenoirDemoDatabase.cs",
		"src/RenoirDemo.AppBlazor/Program.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing Renoir file %s: %v", path, err)
		}
	}
}

func TestDddAggregateGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RenoirDemo")
	if err := Run([]string{"new", "RenoirDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "active:bool", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"src/RenoirDemo.DomainModel/Contexts/Catalog/Aggregates/Product.cs",
		"src/RenoirDemo.DomainModel/DomainException.cs",
		"src/RenoirDemo.DomainModel/DomainGuard.cs",
		"src/RenoirDemo.Application/Contexts/Catalog/ProductCrudService.cs",
		"src/RenoirDemo.Application/Contexts/Catalog/ProductValidator.cs",
		"src/RenoirDemo.AppBlazor/Components/Pages/Catalog/ProductCrud.razor",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing DDD file %s: %v", path, err)
		}
	}
	service, err := os.ReadFile(filepath.Join(project, "src/RenoirDemo.Application/Contexts/Catalog/ProductCrudService.cs"))
	if err != nil || !strings.Contains(string(service), "public sealed record ProductRequest") || !strings.Contains(string(service), "public sealed record ProductView") {
		t.Fatalf("CRUD service should expose separate input and view records: %v", err)
	}
	aggregate, err := os.ReadFile(filepath.Join(project, "src/RenoirDemo.DomainModel/Contexts/Catalog/Aggregates/Product.cs"))
	if err != nil || !strings.Contains(string(aggregate), "DomainGuard.Required(name, nameof(name))") {
		t.Fatalf("aggregate should enforce required string invariants: %v", err)
	}
	validator, err := os.ReadFile(filepath.Join(project, "src/RenoirDemo.Application/Contexts/Catalog/ProductValidator.cs"))
	if err != nil || !strings.Contains(string(validator), "RuleFor(x => x.Name).NotEmpty()") {
		t.Fatalf("CRUD request should have FluentValidation rules: %v", err)
	}
}

func TestRenderString(t *testing.T) {
	result, err := renderString("{{ pascal .Name }} {{ kebab .Name }}", data{Name: "BookShelf"})
	if err != nil || !strings.Contains(result, "BookShelf book-shelf") {
		t.Fatalf("unexpected render result %q, error %v", result, err)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
