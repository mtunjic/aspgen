package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const importDBTestScript = `
CREATE TABLE Customers (
    Id INTEGER PRIMARY KEY,
    Name TEXT NOT NULL,
    Active BOOLEAN
);

CREATE TABLE Orders (
    Id INTEGER PRIMARY KEY,
    Total DECIMAL(18,2) NOT NULL
);
`

func writeImportDBTestScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.sql")
	if err := os.WriteFile(path, []byte(importDBTestScript), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewWithScriptImportsEntities(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "ScriptApp")
	if err := Run([]string{
		"new", "ScriptApp", "--app", "webapi", "--simple",
		"--script", script, "--provider", "sqlite", "--tables", "all",
		"--output", project,
	}); err != nil {
		t.Fatal(err)
	}
	model, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Customer.cs"))
	if err != nil {
		t.Fatalf("Customer.cs not generated: %v", err)
	}
	content := string(model)
	if !strings.Contains(content, "public string Name") {
		t.Fatalf("Customer.cs missing Name property:\n%s", content)
	}
	if strings.Count(content, "Id") != 1 {
		t.Fatalf("Customer.cs should have exactly one Id member (from the template, not a synthesized PK column):\n%s", content)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Order.cs")); err != nil {
		t.Fatalf("Order.cs not generated (table name should singularize Orders -> Order): %v", err)
	}
	dbContext, err := os.ReadFile(filepath.Join(project, "src/WebApi/Data/AppDbContext.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbContext), "Customers") || !strings.Contains(string(dbContext), "Orders") {
		t.Fatalf("AppDbContext.cs missing DbSet registrations:\n%s", dbContext)
	}
	schemaSQL, err := os.ReadFile(filepath.Join(project, "schema.sql"))
	if err != nil {
		t.Fatalf("schema.sql backup not written: %v", err)
	}
	if !strings.Contains(string(schemaSQL), "CREATE TABLE Customers") || !strings.Contains(string(schemaSQL), "CREATE TABLE Orders") {
		t.Fatalf("unexpected schema.sql content:\n%s", schemaSQL)
	}
}

func TestImportDBCmdIncremental(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "IncrementalApp")
	if err := Run([]string{"new", "IncrementalApp", "--app", "webapi", "--simple", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{
		"import-db", "--project", project,
		"--script", script, "--provider", "sqlite", "--tables", "Orders",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Order.cs")); err != nil {
		t.Fatalf("Order.cs not generated: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Customer.cs")); err == nil {
		t.Fatal("Customer.cs should not be generated since --tables only requested Orders")
	}
	manifest, err := loadManifest(project)
	if err != nil {
		t.Fatal(err)
	}
	if !hasComponent(manifest.Components, "db-import:sqlite") {
		t.Fatalf("manifest missing db-import component: %#v", manifest.Components)
	}
	for _, component := range manifest.Components {
		if strings.Contains(component, script) {
			t.Fatalf("manifest must not contain the script path/connection details: %#v", manifest.Components)
		}
	}
}

func TestImportDBRequiresContextOnRenoirProfile(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "RenoirImportDemo")
	if err := Run([]string{"new", "RenoirImportDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"import-db", "--project", project, "--script", script, "--provider", "sqlite"})
	if err == nil || !strings.Contains(err.Error(), "--context") {
		t.Fatalf("expected an error requiring --context on the Renoir profile, got: %v", err)
	}
}

func TestImportDBGeneratesRenoirAggregates(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "RenoirImportDemo2")
	if err := Run([]string{"new", "RenoirImportDemo2", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{
		"import-db", "--project", project, "--script", script, "--provider", "sqlite",
		"--context", "Catalog",
	}); err != nil {
		t.Fatal(err)
	}
	aggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirImportDemo2.DomainModel", "Catalog", "Customer.cs"))
	if err != nil {
		t.Fatalf("Customer.cs not generated: %v", err)
	}
	if !strings.Contains(string(aggregate), "public string Name") {
		t.Fatalf("Customer.cs missing Name property:\n%s", aggregate)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src", "RenoirImportDemo2.DomainModel", "Catalog", "Order.cs")); err != nil {
		t.Fatalf("Order.cs not generated (table name should singularize Orders -> Order): %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "schema.sql")); err != nil {
		t.Fatalf("schema.sql backup not written: %v", err)
	}
	manifest, err := loadManifest(project)
	if err != nil {
		t.Fatal(err)
	}
	if !contextExists(manifest.Contexts, "Catalog") {
		t.Fatalf("expected Catalog context to be auto-created, got: %#v", manifest.Contexts)
	}
}

func TestNewWithScriptImportsRenoirAggregates(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "RenoirScriptApp")
	if err := Run([]string{
		"new", "RenoirScriptApp", "--app", "blazor",
		"--script", script, "--provider", "sqlite", "--context", "Catalog",
		"--output", project,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src", "RenoirScriptApp.DomainModel", "Catalog", "Customer.cs")); err != nil {
		t.Fatalf("Customer.cs not generated: %v", err)
	}
	crudService, err := os.ReadFile(filepath.Join(project, "src", "RenoirScriptApp.Application", "CustomerCrudService.cs"))
	if err != nil {
		t.Fatalf("CustomerCrudService.cs not generated: %v", err)
	}
	if !strings.Contains(string(crudService), "Name") {
		t.Fatalf("CustomerCrudService.cs missing Name field:\n%s", crudService)
	}
}

func TestImportDBRequiresProvider(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "NoProviderApp")
	if err := Run([]string{"new", "NoProviderApp", "--app", "webapi", "--simple", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"import-db", "--project", project, "--script", script}); err == nil {
		t.Fatal("expected an error when --provider is missing")
	}
}
