package generator

import (
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

func TestSplitRelationArgs(t *testing.T) {
	entities := []EntityMeta{
		{Name: "Customer", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
	}
	remaining, relations, manyToMany, err := splitRelationArgs("Order", []string{"total:decimal", "customer:Customer", "notes:string?"}, entities, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 || remaining[0] != "total:decimal" || remaining[1] != "notes:string?" {
		t.Fatalf("unexpected remaining args: %#v", remaining)
	}
	if len(relations) != 1 {
		t.Fatalf("expected one relation, got %#v", relations)
	}
	if len(manyToMany) != 0 {
		t.Fatalf("expected no many-to-many relations, got %#v", manyToMany)
	}
	rel := relations[0]
	if rel.Name != "Customer" || rel.Target != "Customer" || rel.FKProperty != "CustomerId" || rel.DisplayProperty != "Name" || rel.Optional {
		t.Fatalf("unexpected relation: %#v", rel)
	}
}

func TestSplitRelationArgsManyToMany(t *testing.T) {
	entities := []EntityMeta{
		{Name: "Tag", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
	}
	remaining, relations, manyToMany, err := splitRelationArgs("Post", []string{"title:string", "tags:Tag[]"}, entities, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0] != "title:string" {
		t.Fatalf("unexpected remaining args: %#v", remaining)
	}
	if len(relations) != 0 {
		t.Fatalf("expected no many-to-one relations, got %#v", relations)
	}
	if len(manyToMany) != 1 {
		t.Fatalf("expected one many-to-many relation, got %#v", manyToMany)
	}
	rel := manyToMany[0]
	if rel.Name != "Tags" || rel.Target != "Tag" || rel.JoinEntity != "PostTag" || rel.DisplayProperty != "Name" {
		t.Fatalf("unexpected many-to-many relation: %#v", rel)
	}
}

func TestSplitRelationArgsOptional(t *testing.T) {
	entities := []EntityMeta{{Name: "Customer"}}
	_, relations, _, err := splitRelationArgs("Order", []string{"customer:Customer?"}, entities, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || !relations[0].Optional {
		t.Fatalf("expected optional relation, got %#v", relations)
	}
	if relations[0].DisplayProperty != "Id" {
		t.Fatalf("expected fallback display property Id, got %q", relations[0].DisplayProperty)
	}
}

func TestSplitRelationArgsLeavesUnknownTargetForParseProperties(t *testing.T) {
	remaining, relations, _, err := splitRelationArgs("Order", []string{"customer:Customer"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 0 || len(remaining) != 1 {
		t.Fatalf("expected token left for parseProperties to reject, got remaining=%#v relations=%#v", remaining, relations)
	}
	if _, err := parseProperties(remaining); err == nil {
		t.Fatal("expected parseProperties to reject the unknown type")
	}
}

func TestSplitRelationArgsRejectsCrossContextTarget(t *testing.T) {
	entities := []EntityMeta{{Name: "Customer", Context: "Sales"}}
	if _, _, _, err := splitRelationArgs("Order", []string{"customer:Customer"}, entities, "Billing"); err == nil {
		t.Fatal("expected an error for a cross-context relation target")
	}
}

func TestHasScalarPropertyArgs(t *testing.T) {
	if hasScalarPropertyArgs([]string{"--project", "./demo"}) {
		t.Fatal("expected no scalar property args")
	}
	if !hasScalarPropertyArgs([]string{"--project", "./demo", "name:string"}) {
		t.Fatal("expected scalar property arg to be detected")
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
	if err := rejectAggregateReservedProperties([]Property{{Name: "Id"}}, ""); err == nil {
		t.Fatal("expected aggregate identity property to be reserved")
	}
}

func TestAggregateRejectsESReservedProperties(t *testing.T) {
	if err := rejectAggregateReservedProperties([]Property{{Name: "Version"}}, "es"); err == nil {
		t.Fatal("expected es-tier aggregate Version property to be reserved")
	}
	if err := rejectAggregateReservedProperties([]Property{{Name: "Deleted"}}, "es"); err == nil {
		t.Fatal("expected es-tier aggregate Deleted property to be reserved")
	}
	if err := rejectAggregateReservedProperties([]Property{{Name: "Version"}}, "dm"); err != nil {
		t.Fatalf("Version should not be reserved outside es tier: %v", err)
	}
}

func TestDottedProjectNameGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Commerce")
	if err := Run([]string{"new", "Markosoft.Commerce", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
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
	if err := Run([]string{"new", "ThemedDemo", "--context", "Sales", "--arch", "dm", "-ui", "wpf", "--theme:wpfui", "--output", project}); err != nil {
		t.Fatal(err)
	}
	csproj, err := os.ReadFile(filepath.Join(project, "src/Desktop/ThemedDemo.Desktop.csproj"))
	if err != nil || !strings.Contains(string(csproj), `<PackageReference Include="WPF-UI" Version="4.3.0" />`) {
		t.Fatalf("WPF-UI package was not added: %v", err)
	}
	app, err := os.ReadFile(filepath.Join(project, "src/Desktop/App.xaml"))
	if err != nil || !strings.Contains(string(app), `<ui:ThemesDictionary Theme="Light" />`) || !strings.Contains(string(app), `<ui:ControlsDictionary />`) {
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
	if err != nil || !strings.Contains(string(manifest), `"theme:wpfui"`) || !strings.Contains(string(manifest), `"theme-mode:light"`) {
		t.Fatalf("WPF-UI theme was not recorded in the manifest: %v", err)
	}
	if err := Run([]string{"add", "aggregate", "Event", "title:string", "count:int", "price:decimal", "eventDate:date", "startsAt:datetime", "active:bool", "externalId:guid", "--context", "Sales", "--project", project}); err != nil {
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
		"TextFillColorSecondaryBrush",
		"Add new Event",
		"PrevPageCommand",
		"NextPageCommand",
		"PageLabel",
	} {
		if !strings.Contains(viewText, expected) {
			t.Fatalf("WPF-UI entity list view was missing %q", expected)
		}
	}
	editView, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Event/Views/EventEditView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	editViewText := string(editView)
	for _, expected := range []string{
		"ui:NumberBox",
		"ui:CalendarDatePicker",
		`Date="{Binding Form.EventDate`,
		"ui:TimePicker",
		`SelectedTime="{Binding Form.StartsAtTime`,
		"ui:ToggleSwitch",
	} {
		if !strings.Contains(editViewText, expected) {
			t.Fatalf("WPF-UI entity edit view was missing %q", expected)
		}
	}
	if strings.Contains(editViewText, `SelectedDate="{Binding Form.EventDate`) {
		t.Fatal("WPF-UI CalendarDatePicker must bind Date rather than SelectedDate")
	}
	detailsView, err := os.ReadFile(filepath.Join(project, "src/Desktop/Modules/Event/Views/EventDetailsView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailsView), "Command=\"{Binding EditCommand}\"") || !strings.Contains(string(detailsView), "Command=\"{Binding DeleteCommand}\"") {
		t.Fatal("WPF-UI details view is missing Edit/Delete buttons")
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

func TestWpfUIThemeModeGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "DarkDemo")
	if err := Run([]string{"new", "DarkDemo", "--context", "Sales", "--arch", "dm", "-ui", "wpf", "--theme:wpfui", "--theme-mode:dark", "--output", project}); err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile(filepath.Join(project, "src/Desktop/App.xaml"))
	if err != nil || !strings.Contains(string(app), `<ui:ThemesDictionary Theme="Dark" />`) {
		t.Fatalf("WPF-UI dark theme was not generated: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest), `"theme-mode:dark"`) {
		t.Fatalf("WPF-UI dark theme mode was not recorded in the manifest: %v", err)
	}
}

func TestDatabaseProviderGeneration(t *testing.T) {
	sqliteProject := filepath.Join(t.TempDir(), "SqliteApi")
	if err := Run([]string{"new", "SqliteApi", "--context", "Sales", "--arch", "cqrs", "--output", sqliteProject}); err != nil {
		t.Fatal(err)
	}
	sqliteProjectFile, err := os.ReadFile(filepath.Join(sqliteProject, "src/SqliteApi.Infrastructure/SqliteApi.Infrastructure.csproj"))
	if err != nil || !strings.Contains(string(sqliteProjectFile), "Microsoft.EntityFrameworkCore.Sqlite") || !strings.Contains(string(sqliteProjectFile), "SQLitePCLRaw.lib.e_sqlite3") {
		t.Fatalf("SQLite provider was not generated: %v", err)
	}
	sqliteProgram, err := os.ReadFile(filepath.Join(sqliteProject, "src/SqliteApi.Infrastructure/DependencyInjection.cs"))
	if err != nil || !strings.Contains(string(sqliteProgram), "UseSqlite") || !strings.Contains(string(sqliteProgram), "Data Source=SqliteApi.db") {
		t.Fatalf("SQLite configuration was not generated: %v", err)
	}
	manifest2, err := os.ReadFile(filepath.Join(sqliteProject, ".aspgen/manifest.json"))
	if err != nil || !strings.Contains(string(manifest2), `"database:sqlite"`) {
		t.Fatalf("SQLite provider was not recorded: %v", err)
	}

	postgresProject := filepath.Join(t.TempDir(), "PostgresApi")
	if err := Run([]string{"new", "PostgresApi", "--context", "Sales", "--arch", "cqrs", "--database:postgres", "--output", postgresProject}); err != nil {
		t.Fatal(err)
	}
	postgresProjectFile, err := os.ReadFile(filepath.Join(postgresProject, "src/PostgresApi.Infrastructure/PostgresApi.Infrastructure.csproj"))
	if err != nil || !strings.Contains(string(postgresProjectFile), "Npgsql.EntityFrameworkCore.PostgreSQL") || strings.Contains(string(postgresProjectFile), "EntityFrameworkCore.Sqlite") {
		t.Fatalf("PostgreSQL provider was not generated: %v", err)
	}
	postgresProgram, err := os.ReadFile(filepath.Join(postgresProject, "src/PostgresApi.Infrastructure/DependencyInjection.cs"))
	if err != nil || !strings.Contains(string(postgresProgram), "UseNpgsql") {
		t.Fatalf("PostgreSQL configuration was not generated: %v", err)
	}

	if err := Run([]string{"new", "InvalidDatabase", "--context", "Sales", "--arch", "cqrs", "--database", "mysql", "--output", filepath.Join(t.TempDir(), "InvalidDatabase")}); err == nil {
		t.Fatal("expected unsupported database error")
	}
}

func TestDddAggregateGeneration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "CatalogDemo")
	if err := Run([]string{"new", "CatalogDemo", "--context", "Catalog", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "active:bool", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"src/CatalogDemo.DomainModel/Catalog/Product.cs",
		"src/CatalogDemo.DomainModel/DomainException.cs",
		"src/CatalogDemo.DomainModel/DomainGuard.cs",
		"src/CatalogDemo.Application/ProductCrudService.cs",
		"src/CatalogDemo.Application/ProductValidator.cs",
	} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing DDD file %s: %v", path, err)
		}
	}
	service, err := os.ReadFile(filepath.Join(project, "src/CatalogDemo.Application/ProductCrudService.cs"))
	if err != nil || !strings.Contains(string(service), "public sealed record ProductRequest") || !strings.Contains(string(service), "public sealed record ProductView") {
		t.Fatalf("CRUD service should expose separate input and view records: %v", err)
	}
	aggregate, err := os.ReadFile(filepath.Join(project, "src/CatalogDemo.DomainModel/Catalog/Product.cs"))
	if err != nil || !strings.Contains(string(aggregate), "DomainGuard.Required(name, nameof(name))") {
		t.Fatalf("aggregate should enforce required string invariants: %v", err)
	}
	validator, err := os.ReadFile(filepath.Join(project, "src/CatalogDemo.Application/ProductValidator.cs"))
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
