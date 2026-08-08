package generator

import (
	"strings"
	"testing"
)

// Every relation-less dm/cqrs aggregate gets a generated CRUD-service round-trip
// smoke test (tests-crud group) that runs the same in-process data path the
// desktop/MVC frontends call. Relation aggregates are covered by their own
// relation tests, so they must NOT get one.

func TestSeedArgs(t *testing.T) {
	props := []Property{
		{Name: "Title", CSharpType: "string"},
		{Name: "Age", CSharpType: "int"},
		{Name: "Active", CSharpType: "bool"},
	}
	if got := seedArgs(props); got != `"Title sample 1", 20, true` {
		t.Errorf("seedArgs = %q", got)
	}
}

func TestSearchNullArgs(t *testing.T) {
	cases := []struct {
		props []Property
		want  string
	}{
		{[]Property{{Name: "Title", CSharpType: "string"}}, ", null"},
		{[]Property{{Name: "Age", CSharpType: "int"}}, ", null, null"},
		{[]Property{{Name: "Active", CSharpType: "bool"}}, ", null"},
		{[]Property{{Name: "Title", CSharpType: "string"}, {Name: "Age", CSharpType: "int"}}, ", null, null, null"},
		{[]Property{{Name: "When", CSharpType: "DateOnly"}}, ", null, null"},
		// an FK property's filter is a single long?; a nested relation field
		// adds one string?
		{[]Property{
			{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer", RelationDisplayProperty: "Name"},
		}, ", null, null"},
		{[]Property{{Name: "Tags", CSharpType: "Tag[]"}}, ""},
	}
	for _, tc := range cases {
		if got := searchNullArgs(tc.props); got != tc.want {
			t.Errorf("searchNullArgs(%#v) = %q, want %q", tc.props, got, tc.want)
		}
	}
}

func TestRenderCrudServiceSmokeTest(t *testing.T) {
	d := m2mData()
	d.Relations = nil
	d.ManyToMany = nil
	d.Properties = []Property{
		{Name: "Title", DisplayName: "Title", CSharpType: "string", UIControl: "InputText"},
		{Name: "Age", DisplayName: "Age", CSharpType: "int", UIControl: "InputNumber"},
		{Name: "Active", DisplayName: "Active", CSharpType: "bool", UIControl: "InputCheckbox"},
	}
	path := "files/tests-crud/tests/{{ .Project }}.UnitTests/{{ .Aggregate }}CrudServiceTests.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"public class PostCrudServiceTests",
		"PooledDbContextFactory<DemoDatabase>",
		"UseSqlite(connectionString)",
		"new PostCrudService(factory, new PostValidator());",
		`new PostRequest("Title sample 1", 20, true)`,
		"var created = await service.CreateAsync(request);",
		"Assert.True(created.Id > 0, \"create should assign a persisted id\")",
		"var fetched = await service.GetByIdAsync(created.Id);",
		"var (items, totalCount) = await service.SearchAsync(null, null, null, null, null, 1, 25);",
		"var updated = await service.UpdateAsync(created.Id, request);",
		"await service.DeleteAsync(created.Id);",
		"Assert.Equal(0, remaining);",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("CrudService smoke test missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}
