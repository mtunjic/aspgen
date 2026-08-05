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

func TestImportDBCmdIncremental(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "IncrementalApp")
	if err := Run([]string{"new", "IncrementalApp", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{
		"import-db", "--project", project, "--context", "Catalog",
		"--script", script, "--provider", "sqlite", "--tables", "Orders",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Catalog/Order.cs")); err != nil {
		t.Fatalf("Order.cs not generated: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(project, "src/WebApi/Models/Catalog/Customer.cs")); err == nil {
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

func TestImportDBRequiresContext(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "ImportNoContextDemo")
	if err := Run([]string{"new", "ImportNoContextDemo", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"import-db", "--project", project, "--script", script, "--provider", "sqlite"})
	if err == nil || !strings.Contains(err.Error(), "--context") {
		t.Fatalf("expected an error requiring --context, got: %v", err)
	}
}

func TestImportDBRequiresProvider(t *testing.T) {
	script := writeImportDBTestScript(t)
	project := filepath.Join(t.TempDir(), "NoProviderApp")
	if err := Run([]string{"new", "NoProviderApp", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"import-db", "--project", project, "--script", script}); err == nil {
		t.Fatal("expected an error when --provider is missing")
	}
}
