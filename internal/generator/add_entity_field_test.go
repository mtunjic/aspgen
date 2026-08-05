package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddEntityFieldContextAggregate(t *testing.T) {
	project := filepath.Join(t.TempDir(), "FieldDemo")
	if err := Run([]string{"new", "FieldDemo", "--context", "Catalog", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity-field", "Product", "category:string", "--project", project}); err != nil {
		t.Fatal(err)
	}

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "FieldDemo.DomainModel", "Catalog", "Product.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public string Category { get; private set; } = default!;") {
		t.Fatalf("expected Category property on Product aggregate: %s", aggregate)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "FieldDemo.Application", "ProductCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "Category") {
		t.Fatalf("expected Category to appear in ProductCrudService.cs: %s", crudService)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	existing := findEntityMeta(manifest.Entities, "Product")
	if existing == nil || len(existing.Properties) != 2 {
		t.Fatalf("expected Product to have 2 recorded properties (name, category), got %#v", existing)
	}
}

func TestAddEntityFieldArTierEntity(t *testing.T) {
	project := filepath.Join(t.TempDir(), "ArFieldDemo")
	if err := Run([]string{"new", "ArFieldDemo", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Product", "name:string", "price:decimal", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity-field", "Product", "sku:string", "--project", project}); err != nil {
		t.Fatal(err)
	}

	model, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Models", "Catalog", "Product.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(model), "public string Sku { get; set; } = default!;") {
		t.Fatalf("expected Sku property on Product model: %s", model)
	}

	endpoints, err := os.ReadFile(filepath.Join(project, "src", "WebApi", "Features", "Catalog", "Product", "ProductEndpoints.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(endpoints), "entity.Sku = request.Sku;") {
		t.Fatalf("expected Sku assignment in ProductEndpoints.cs: %s", endpoints)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	existing := findEntityMeta(manifest.Entities, "Product")
	if existing == nil || len(existing.Properties) != 3 {
		t.Fatalf("expected Product to have 3 recorded properties (name, price, sku), got %#v", existing)
	}
}

func TestAddEntityFieldErrors(t *testing.T) {
	project := filepath.Join(t.TempDir(), "FieldErrDemo")
	if err := Run([]string{"new", "FieldErrDemo", "--context", "Catalog", "--arch", "ar", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "entity", "Customer", "name:string", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{"add", "entity-field", "Unknown", "notes:string", "--project", project}); err == nil {
		t.Fatal("expected an error when adding a field to a non-existent entity")
	}
	if err := Run([]string{"add", "entity-field", "Customer", "name:string", "--project", project}); err == nil {
		t.Fatal("expected an error when adding a duplicate property name")
	}
}
