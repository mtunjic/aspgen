package generator

import (
	"strings"
	"testing"
)

// The desktop (dm-tier) data layer uses IDbContextFactory so every CrudService
// call runs on its own pooled DbContext: the singleton services are invoked
// from background threads (Task.Run offload), and sharing one EF context
// would race. These tests pin the factory wiring across the generated hosts.

func TestRenderCrudServiceUsesDbContextFactory(t *testing.T) {
	out := renderTemplate(t, "files/dm-crud/src/{{ .Project }}.Application/{{ .Aggregate }}CrudService.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public sealed class PostCrudService(IDbContextFactory<DemoDatabase> databaseFactory, IValidator<PostRequest> validator)",
		"await using var database = databaseFactory.CreateDbContext();",
		"await database.SaveChangesSafelyAsync(cancellationToken);",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("CrudService missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, "PostCrudService(DemoDatabase database") {
		t.Errorf("CrudService must not take the raw DbContext\n--- rendered ---\n%s", out)
	}
}

func TestRenderRepositoryUsesDbContextFactory(t *testing.T) {
	out := renderTemplate(t, "files/renoir-repository/src/{{ .Project }}.Persistence/Repositories/{{ .Name }}.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public sealed class Post(IDbContextFactory<DemoDatabase> databaseFactory) : IPost",
		"await using var database = databaseFactory.CreateDbContext();",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("repository missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfModuleRegistersDbContextFactory(t *testing.T) {
	d := m2mData()
	d.Backend = "dm"
	out := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/{{ .Name }}Module.cs.tmpl", d)
	for _, expected := range []string{
		"containerRegistry.RegisterSingleton<IDbContextFactory<DemoDatabase>>(() =>",
		"new PooledDbContextFactory<DemoDatabase>(options)",
		"db.Database.EnsureCreated();",
		"containerRegistry.RegisterSingleton<PostCrudService>();",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("WPF module missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, "RegisterSingleton<DemoDatabase>") {
		t.Errorf("WPF module must not register a shared singleton DbContext\n--- rendered ---\n%s", out)
	}
}

func TestRenderWebDependencyInjectionRegistersDbContextFactory(t *testing.T) {
	for _, path := range []string{
		"files/cqrs/src/{{ .Project }}.Infrastructure/DependencyInjection.cs.tmpl",
		"files/mvc-context/src/{{ .Project }}.WebMvc/Program.cs.tmpl",
	} {
		out := renderTemplate(t, path, m2mData())
		for _, expected := range []string{
			// the factory is a singleton with SELF-CONTAINED options so it never
			// depends on the scoped AddDbContext<...> options registration
			"AddSingleton<IDbContextFactory<DemoDatabase>>",
			"new PooledDbContextFactory<DemoDatabase>",
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("%s missing %q (CrudService now needs IDbContextFactory)\n--- rendered ---\n%s", path, expected, out)
			}
		}
		if strings.Contains(out, "AddDbContextFactory<DemoDatabase>") {
			t.Errorf("%s must not use AddDbContextFactory (would consume the scoped options)\n--- rendered ---\n%s", path, out)
		}
	}
}

func TestRenderMvcRelationTestUsesDbContextFactory(t *testing.T) {
	m := &Manifest{
		Project: "Demo",
		Entities: []EntityMeta{
			{Name: "Customer", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
			{Name: "Tag", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		},
	}
	props := []Property{
		{Name: "Title", CSharpType: "string"},
		{Name: "Body", CSharpType: "string?"},
		{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer", RelationDisplayProperty: "Name"},
	}
	relations := []Relation{{Name: "Customer", Target: "Customer", FKProperty: "CustomerId", DisplayProperty: "Name", Optional: true}}
	manyToMany := []ManyToManyRelation{{Name: "Tags", DisplayName: "Tags", Target: "Tag", JoinEntity: "PostTag", DisplayProperty: "Name"}}

	d := data{
		Project:      "Demo",
		Context:      "Blog",
		Aggregate:    "Post",
		RelationTest: buildRelationTest(m, "Blog", "Post", props, relations, manyToMany),
	}
	path := "files/tests-mvc-relations/tests/{{ .Project }}.IntegrationTests/{{ .Aggregate }}MvcRelationTests.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"GetRequiredService<IDbContextFactory<DemoDatabase>>()",
		"new CustomerCrudService(dbFactory, new CustomerValidator())",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("MvcRelationTests missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}
