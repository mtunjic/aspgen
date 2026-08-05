package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddEntityFieldSimpleBackend(t *testing.T) {
	project := filepath.Join(t.TempDir(), "FieldDemo")
	if err := Run([]string{"new", "FieldDemo", "--app", "webapi", "--simple", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity-field", "Customer", "notes:string", "--project", project}); err != nil {
		t.Fatal(err)
	}

	model, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(model), "public string Notes { get; set; }") {
		t.Fatalf("expected Notes property on Customer model: %s", model)
	}

	endpoints, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Features", "Customer", "CustomerEndpoints.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(endpoints), "entity.Notes = request.Notes;") {
		t.Fatalf("expected Notes assignment in CustomerEndpoints.cs: %s", endpoints)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	existing := findEntityMeta(manifest.Entities, "Customer")
	if existing == nil || len(existing.Properties) != 2 {
		t.Fatalf("expected Customer to have 2 recorded properties (name, notes), got %#v", existing)
	}
}

func TestAddEntityFieldRenoirAggregate(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RenoirFieldDemo")
	if err := Run([]string{"new", "RenoirFieldDemo", "--app", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity-field", "Product", "category:string", "--project", project}); err != nil {
		t.Fatal(err)
	}

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "RenoirFieldDemo.DomainModel", "Catalog", "Product.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public string Category { get; private set; } = default!;") {
		t.Fatalf("expected Category property on Product aggregate: %s", aggregate)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "RenoirFieldDemo.Application", "ProductCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "Category") {
		t.Fatalf("expected Category to appear in ProductCrudService.cs: %s", crudService)
	}
}

func TestAddEntityFieldErrors(t *testing.T) {
	project := filepath.Join(t.TempDir(), "FieldErrDemo")
	if err := Run([]string{"new", "FieldErrDemo", "--app", "webapi", "--simple", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "--project", project}); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{"add", "entity-field", "Unknown", "notes:string", "--project", project}); err == nil {
		t.Fatal("expected an error when adding a field to a non-existent entity")
	}
	if err := Run([]string{"add", "entity-field", "Customer", "name:string", "--project", project}); err == nil {
		t.Fatal("expected an error when adding a duplicate property name")
	}
}
