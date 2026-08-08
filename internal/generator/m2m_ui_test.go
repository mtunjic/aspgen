package generator

import (
	"strings"
	"testing"

	"aspgen/internal/templates"
)

// renderTemplate loads a .tmpl file from the embedded templates.FS and
// renders it with d, returning the raw output so tests can assert on the
// exact generated C#/Razor/XAML. This exercises the template branch logic
// (many-to-one dropdowns vs many-to-many multi-select, theme, backend,
// store-injection deduplication) deterministically without a full
// `aspgen new`/`add` integration run.
func renderTemplate(t *testing.T, path string, d data) string {
	t.Helper()
	content, err := templates.FS.ReadFile(path)
	if err != nil {
		t.Fatalf("read template %s: %v", path, err)
	}
	rendered, err := renderString(string(content), d)
	if err != nil {
		t.Fatalf("render template %s: %v", path, err)
	}
	return rendered
}

// m2mData returns a representative aggregate carrying one optional
// many-to-one relation (customer:Customer) and one many-to-many relation
// (tags:Tag[]), the richest combination a single entity can declare.
func m2mData() data {
	return data{
		Project:   "Demo",
		Namespace: "Demo.Desktop.Modules.Post",
		Name:      "Post",
		Context:   "Blog",
		Aggregate: "Post",
		Properties: []Property{
			{Name: "Title", DisplayName: "Title", CSharpType: "string", UIControl: "InputText"},
			{Name: "Body", DisplayName: "Body", CSharpType: "string?", UIControl: "InputText"},
			{Name: "CustomerId", DisplayName: "Customer", CSharpType: "long?", UIControl: "InputNumber", RelationTarget: "Customer", RelationDisplayProperty: "Name"},
		},
		Relations: []Relation{
			{Name: "Customer", Target: "Customer", FKProperty: "CustomerId", DisplayProperty: "Name", Optional: true},
		},
		ManyToMany: []ManyToManyRelation{
			{Name: "Tags", DisplayName: "Tags", Target: "Tag", JoinEntity: "PostTag", DisplayProperty: "Name"},
		},
		Backend: "dm",
		Theme:   "",
	}
}

const wpfViewModelPath = "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}ViewModel.cs.tmpl"

func TestWpfViewModelManyToManyRendering(t *testing.T) {
	out := renderTemplate(t, wpfViewModelPath, m2mData())
	for _, expected := range []string{
		// stores injected for the m2o target and the m2m target + join store
		"private readonly ICustomerStore customerStore;",
		"private readonly ITagStore tagStore;",
		"private readonly IPostTagStore postTagStore;",
		// ctor params in the same order
		"public PostViewModel(IPostStore store, IRegionManager regionManager, ICustomerStore customerStore, ITagStore tagStore, IPostTagStore postTagStore",
		// pickers + list reload on navigation (Prism reuses the view)
		"public sealed class PostViewModel : BindableBase, INavigationAware",
		"public void OnNavigatedTo(NavigationContext navigationContext)",
		"LoadRelated();",
		// option collection + hydration using the target's display property
		"public ObservableCollection<TagOption> TagOptions { get; } = [];",
		"TagOptions.Add(new TagOption(item.Id, item.Name));",
		// selection restored on edit from the join entity's rows
		"new PostTagSearchCriteria(null, SelectedItem.Id, null, 1, 1000)",
		"option.IsSelected = linkedTag.Contains(option.Id);",
		// save captures the created id and reconciles join rows
		"var saved = store.Save(editingId, value);",
		"new PostTagRow(0, saved.Id, id)",
		"postTagStore.Delete(link.Id);",
		// reset on New()/Cancel()
		"foreach (var option in TagOptions) option.IsSelected = false;",
		// generated option wrapper class
		"public sealed class TagOption : BindableBase",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostViewModel rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The initial load must move out of the constructor (it would otherwise
	// never re-run on navigation-back, leaving pickers stale).
	if strings.Contains(out, "LoadRelated();\n        Search();\n    }") {
		t.Errorf("PostViewModel constructor must not perform the initial load (moved to OnNavigatedTo)\n%s", out)
	}
}

func TestWpfViewModelManyToManyDeduplicatesStore(t *testing.T) {
	// One entity declaring BOTH tag:Tag (many-to-one) and tags:Tag[] (many-to-
	// many) must inject ITagStore exactly once (field + ctor param), not twice.
	d := m2mData()
	d.Properties = append(d.Properties,
		Property{Name: "TagId", DisplayName: "Tag", CSharpType: "long", UIControl: "InputNumber", RelationTarget: "Tag", RelationDisplayProperty: "Name"})
	d.Relations = append(d.Relations, Relation{Name: "Tag", Target: "Tag", FKProperty: "TagId", DisplayProperty: "Name"})
	out := renderTemplate(t, wpfViewModelPath, d)
	if got := strings.Count(out, "ITagStore tagStore"); got != 2 {
		t.Fatalf("ITagStore must appear exactly twice (field + ctor param), got %d occurrences\n%s", got, out)
	}
}

func TestWpfViewModelWithoutManyToMany(t *testing.T) {
	// A relation-less aggregate must render no m2m artifacts at all.
	d := m2mData()
	d.Relations = nil
	d.ManyToMany = nil
	d.Properties = []Property{{Name: "Title", DisplayName: "Title", CSharpType: "string", UIControl: "InputText"}}
	out := renderTemplate(t, wpfViewModelPath, d)
	for _, forbidden := range []string{"TagOptions", "PostTag", "TagOption"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("relation-less PostViewModel must not contain %q\n%s", forbidden, out)
		}
	}
}

func TestWpfDetailsViewModelManyToManyRendering(t *testing.T) {
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}DetailsViewModel.cs.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		"private readonly ITagStore tagStore;",
		"private readonly IPostTagStore postTagStore;",
		"public PostDetailsViewModel(IPostStore store, IRegionManager regionManager, ICustomerStore customerStore, ITagStore tagStore, IPostTagStore postTagStore)",
		"public string TagsDisplay { get; private set; } = string.Empty;",
		"new PostTagSearchCriteria(null, id, null, 1, 1000)",
		`TagsDisplay = string.Join(", ", tagStore.GetAll().Where(x => linkedTag.Contains(x.Id)).Select(x => x.Name));`,
		"RaisePropertyChanged(nameof(TagsDisplay));",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostDetailsViewModel rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestWpfViewManyToManyRendering(t *testing.T) {
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}View.xaml.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		`<TextBlock Text="Tags" />`,
		`ItemsControl Margin="0,4,0,0" ItemsSource="{Binding TagOptions}"`,
		`IsChecked="{Binding IsSelected, Mode=TwoWay}"`,
		`Content="{Binding Display}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostView.xaml rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestWpfDetailsViewManyToManyRendering(t *testing.T) {
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}DetailsView.xaml.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		`<TextBlock FontWeight="SemiBold" Text="Tags" />`,
		`Text="{Binding TagsDisplay}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostDetailsView.xaml rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestBlazorCrudManyToManyRendering(t *testing.T) {
	path := "files/blazor-context-crud/src/{{ .Project }}.AppBlazor/Components/Pages/{{ .Context }}/{{ .Aggregate }}Crud.razor.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		"@using Demo.Application.Features.Blog.PostTag",
		`<input type="checkbox" class="form-check-input" @bind="option.Selected" />`,
		"@option.Display",
		"private List<TagOption> TagOptions = [];",
		"await SyncTagsAsync(savedId);",
		"foreach (var option in TagOptions) option.Selected = false;",
		"private async Task LoadSelectedTagAsync(long id)",
		`$"/api/blog/post-tag/search?postId={id}&pageSize=1000"`,
		"private async Task SyncTagsAsync(long PostId)",
		`$"/api/blog/post-tag/search?postId={ PostId }&pageSize=1000"`,
		"new PostTagPagedResponse([], 0, 1, 1000)",
		"new PostTagRequest(PostId, id)",
		"private sealed class TagOption",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostCrud.razor rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestBlazorDetailsManyToManyRendering(t *testing.T) {
	path := "files/blazor-context-crud/src/{{ .Project }}.AppBlazor/Components/Pages/{{ .Context }}/{{ .Aggregate }}Details.razor.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		"@using Demo.Application.Features.Blog.PostTag",
		`<dt class="col-sm-3 fw-semibold">Tags</dt>`,
		`@string.Join(", ", TagOptions.Where(x => selectedTagIds.Contains(x.Id)).Select(x => x.Name))`,
		"private HashSet<long> selectedTagIds = [];",
		`$"/api/blog/post-tag/search?postId={Id}&pageSize=1000"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostDetails.razor rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestMvcControllerManyToManyRendering(t *testing.T) {
	path := "files/mvc-context-crud/src/{{ .Project }}.WebMvc/Controllers/{{ .Aggregate }}Controller.cs.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		"TagCrudService tagService",
		"PostTagCrudService postTagService",
		"long[]? selectedTagIds",
		"await SyncTagsAsync(created.Id, selectedTagIds ?? [], cancellationToken);",
		"await SyncTagsAsync(id, selectedTagIds ?? [], cancellationToken);",
		"ViewBag.TagSelectedIds",
		"private async Task SyncTagsAsync(long PostId, long[] selectedTagIds, CancellationToken cancellationToken)",
		"postTagService.SearchAsync(null, PostId, null, 1, 1000, cancellationToken)",
		"new PostTagRequest(PostId, id)",
		// SelectedIds must always be List<long> (the views cast to it), never
		// a raw long[] which would throw a RuntimeBinderException.
		"ViewBag.TagSelectedIds = new List<long>();",
		"ViewBag.TagSelectedIds = (selectedTagIds ?? []).ToList();",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostController rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, "ViewBag.TagSelectedIds = Array.Empty<long>();") {
		t.Errorf("PostController must not assign a raw long[] to TagSelectedIds\n%s", out)
	}
}

func TestMvcCreateEditDetailsManyToManyRendering(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{
			path: "files/mvc-context-crud/src/{{ .Project }}.WebMvc/Views/{{ .Aggregate }}/Create.cshtml.tmpl",
			want: []string{
				`name="selectedTagIds"`,
				`checked="@(((List<long>)ViewBag.TagSelectedIds).Contains(option.Id))"`,
				`@option.Name`,
				// validation summary must only render when there are errors,
				// otherwise a fresh page shows an empty red alert box
				"@if (!ViewData.ModelState.IsValid)",
				`<div asp-validation-summary="All" class="alert alert-danger"></div>`,
			},
		},
		{
			path: "files/mvc-context-crud/src/{{ .Project }}.WebMvc/Views/{{ .Aggregate }}/Edit.cshtml.tmpl",
			want: []string{
				`name="selectedTagIds"`,
				`checked="@(((List<long>)ViewBag.TagSelectedIds).Contains(option.Id))"`,
				"@if (!ViewData.ModelState.IsValid)",
			},
		},
		{
			path: "files/mvc-context-crud/src/{{ .Project }}.WebMvc/Views/{{ .Aggregate }}/Details.cshtml.tmpl",
			want: []string{
				`<dt class="col-sm-3 fw-semibold">Tags</dt>`,
				`((List<long>)ViewBag.TagSelectedIds).Contains(x.Id)`,
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			out := renderTemplate(t, tc.path, m2mData())
			for _, expected := range tc.want {
				if !strings.Contains(out, expected) {
					t.Errorf("rendering is missing %q\n--- rendered ---\n%s", expected, out)
				}
			}
		})
	}
}

func TestEsAggregateTemplateHasNavigationMarker(t *testing.T) {
	path := "files/es-aggregate/src/{{ .Project }}.DomainModel/{{ .Context }}/{{ .Aggregate }}.cs.tmpl"
	content, err := templates.FS.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// updateInverseNavigation refuses to patch an aggregate class that lacks
	// the marker; es-tier aggregates must carry it like renoir-aggregate and
	// ar-entity do, or relations on es-tier aggregates error out.
	if !strings.Contains(string(content), "    // aspgen:navigation") {
		t.Fatalf("es-aggregate template is missing the // aspgen:navigation marker:\n%s", content)
	}
}

func TestBuildRelationTest(t *testing.T) {
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

	rt := buildRelationTest(m, "Blog", "Post", props, relations, manyToMany)
	if rt.Var != "post" {
		t.Fatalf("expected aggregate var 'post', got %q", rt.Var)
	}
	if len(rt.CtorArgs) != 3 || rt.CtorArgs[0] == "" || rt.CtorArgs[1] == "" || rt.CtorArgs[2] != "customer.Id" {
		t.Fatalf("unexpected aggregate ctor args: %#v", rt.CtorArgs)
	}
	if len(rt.Relations) != 1 || rt.Relations[0].Var != "customer" || len(rt.Relations[0].CtorArgs) != 1 {
		t.Fatalf("unexpected m2o target: %#v", rt.Relations)
	}
	if len(rt.ManyToMany) != 1 || rt.ManyToMany[0].Var != "tag1" || rt.ManyToMany[0].SecondVar != "tag2" || rt.ManyToMany[0].JoinEntity != "PostTag" {
		t.Fatalf("unexpected m2m target: %#v", rt.ManyToMany)
	}
	// FormFields: FK -> target id expression, scalars -> quoted form values.
	if len(rt.FormFields) != 3 ||
		rt.FormFields[0].Name != "Title" || rt.FormFields[0].Value != `"Title sample"` ||
		rt.FormFields[2].Name != "CustomerId" || rt.FormFields[2].Value != "customer.ToString()" {
		t.Fatalf("unexpected form fields: %#v", rt.FormFields)
	}
}

func TestRenderTestsRelationsTemplate(t *testing.T) {
	m := &Manifest{
		Project: "Demo",
		Entities: []EntityMeta{
			{Name: "Customer", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
			{Name: "Tag", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		},
	}
	props := []Property{
		{Name: "Title", CSharpType: "string"},
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
	path := "files/tests-relations/tests/{{ .Project }}.UnitTests/{{ .Aggregate }}RelationTests.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"public class PostRelationTests",
		"var customer = new Customer(\"Name sample 1\");",
		"var tag1 = new Tag(\"Name sample 1\");",
		`var post = new Post("Title sample 1", customer.Id);`,
		"db.PostTags.Add(new PostTag(post.Id, tag1.Id));",
		"Assert.Equal(2, TagLinks.Count);",
		"Assert.Single(CustomerMatch);",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostRelationTests rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestTestCtorArgsForeignKeyUsesTargetId(t *testing.T) {
	args := testCtorArgs([]Property{
		{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer"},
		{Name: "Name", CSharpType: "string"},
	})
	if len(args) != 2 || args[0] != "customer.Id" {
		t.Fatalf("expected FK arg to reference the target's id, got %#v", args)
	}
}

func TestTestRequestArgsSeedForeignKeys(t *testing.T) {
	args := testRequestArgs([]Property{
		{Name: "PostId", CSharpType: "long", RelationTarget: "Post"},
		{Name: "Name", CSharpType: "string"},
	})
	if len(args) != 2 || args[0] == "" || strings.Contains(args[0], ".Id") || args[1] == "" {
		t.Fatalf("expected FK args to be numeric seeds, got %#v", args)
	}
}

func TestFormValue(t *testing.T) {
	cases := map[string]string{
		"Title sample":  testFormValue(Property{Name: "Title", CSharpType: "string"}),
		"Body sample":   testFormValue(Property{Name: "Body", CSharpType: "string?"}),
		"20":            testFormValue(Property{Name: "Age", CSharpType: "int"}),
		"2020-01-01":    testFormValue(Property{Name: "When", CSharpType: "DateOnly"}),
		"true":          testFormValue(Property{Name: "Active", CSharpType: "bool"}),
	}
	for want, got := range cases {
		if got != want {
			t.Errorf("testFormValue = %q, want %q", got, want)
		}
	}
}

func TestRenderTestsMvcRelationsTemplate(t *testing.T) {
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
		"public class PostMvcRelationTests",
		`new CustomerCrudService(db, new CustomerValidator()).CreateAsync(new CustomerRequest("Name sample 1"))`,
		`new TagCrudService(db, new TagValidator()).CreateAsync(new TagRequest("Name sample 1"))`,
		`new KeyValuePair<string, string>("Title", "Title sample"),`,
		`new KeyValuePair<string, string>("CustomerId", customer.ToString()),`,
		`new KeyValuePair<string, string>("selectedTagIds", tag1.ToString()),`,
		"Assert.Equal(HttpStatusCode.Redirect, response.StatusCode);",
		`Assert.DoesNotContain("alert-danger", createBody);`,
		`new KeyValuePair<string, string>("Title", "")`,
		"Assert.Contains(\"is required\", badBody);",
		"Assert.Equal(2, TagLinks.Count);",
		"Assert.Equal(customer, aggregate.CustomerId);",
		// nested relation search assertions on the Index page
		`?customerNameContains={Uri.EscapeDataString("Name sample")}`,
		`Assert.DoesNotContain("No Posts found", await nestedHit.Content.ReadAsStringAsync());`,
		`Assert.Contains("No Posts found", await nestedMiss.Content.ReadAsStringAsync());`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostMvcRelationTests rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderMvcLayoutTemplate(t *testing.T) {
	path := "files/mvc-context/src/{{ .Project }}.WebMvc/Views/Shared/_Layout.cshtml.tmpl"
	out := renderTemplate(t, path, data{Project: "Demo"})
	for _, expected := range []string{
		"bootstrap@5.3.3/dist/css/bootstrap.min.css",
		"<!-- aspgen:nav -->",
		`<a class="navbar-brand" asp-controller="Home" asp-action="Index">Demo</a>`,
		"@RenderBody()",
		"@await RenderSectionAsync(\"Scripts\", required: false)",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("_Layout.cshtml rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderMvcHomeControllerTemplate(t *testing.T) {
	path := "files/mvc-context/src/{{ .Project }}.WebMvc/Controllers/HomeController.cs.tmpl"
	out := renderTemplate(t, path, data{Project: "Demo"})
	if !strings.Contains(out, "// aspgen:redirect") {
		t.Fatalf("HomeController template is missing the redirect marker\n%s", out)
	}
}

func TestRelationNestedFilterField(t *testing.T) {
	cases := []struct {
		name string
		p    Property
		want string
	}{
		{"plain string", Property{Name: "Title", CSharpType: "string"}, ""},
		{"relation", Property{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer", RelationDisplayProperty: "Name"}, "CustomerNameContains"},
		{"id display", Property{Name: "CustomerId", CSharpType: "long", RelationTarget: "Customer", RelationDisplayProperty: "Id"}, ""},
		{"join fk suppressed", Property{Name: "TagId", CSharpType: "long", RelationTarget: "Tag", RelationDisplayProperty: "Name", NoNestedFilter: true}, ""},
	}
	for _, tc := range cases {
		if got := relationNestedFilterField(tc.p); got != tc.want {
			t.Errorf("%s: relationNestedFilterField = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRelationFilterHelpers(t *testing.T) {
	props := []Property{
		{Name: "Title", CSharpType: "string"},
		{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer", RelationDisplayProperty: "Name"},
	}
	if got := relationFilterParamDecls(props, "camel"); got != ", string? customerNameContains" {
		t.Errorf("paramDecls = %q", got)
	}
	if got := relationFilterParamNames(props, "request.", "pascal"); got != ", request.CustomerNameContains" {
		t.Errorf("paramNames = %q", got)
	}
	if got := hasRelationNestedFilters(props); !got {
		t.Error("expected nested filters to be detected")
	}
	// dm/cqrs: traverse the navigation property.
	wantDm := "if (!string.IsNullOrWhiteSpace(customerNameContains)) query = query.Where(x => x.Customer != null && x.Customer.Name.Contains(customerNameContains));"
	if got := relationFilterWhereClauses(props, "x", "", ""); got != wantDm {
		t.Errorf("dm where = \n%q\nwant\n%q", got, wantDm)
	}
	// es: subquery into the target read model.
	wantEs := "if (!string.IsNullOrWhiteSpace(request.CustomerNameContains)) query = query.Where(x => database.CustomerReadModels.Any(customer => customer.Id == x.CustomerId && customer.Name.Contains(request.CustomerNameContains)));"
	if got := relationFilterWhereClauses(props, "x", "request.", "es"); got != wantEs {
		t.Errorf("es where = \n%q\nwant\n%q", got, wantEs)
	}
}

func TestRenderBlazorLayoutTemplate(t *testing.T) {
	path := "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Layout/MainLayout.razor.tmpl"
	out := renderTemplate(t, path, data{Project: "Demo"})
	for _, expected := range []string{
		"@inherits LayoutComponentBase",
		`<NavLink class="navbar-brand" href="/" Match="NavLinkMatch.All">Demo</NavLink>`,
		"<!-- aspgen:nav -->",
		"@Body",
		"bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("MainLayout.razor rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderBlazorAppTemplate(t *testing.T) {
	path := "files/blazor-context/src/{{ .Project }}.AppBlazor/App.razor.tmpl"
	out := renderTemplate(t, path, data{Project: "Demo"})
	for _, expected := range []string{
		"bootstrap@5.3.3/dist/css/bootstrap.min.css",
		"<Routes />",
		"_framework/blazor.web.js",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("App.razor rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestBlazorNavHref(t *testing.T) {
	if got := blazorNavHref("Blog", "Post"); got != "/blog/posts" {
		t.Errorf("blazorNavHref = %q, want /blog/posts", got)
	}
}

func TestRenderTestsIntegrationRelationsTemplate(t *testing.T) {
	m := &Manifest{
		Project: "Demo",
		Entities: []EntityMeta{
			{Name: "Customer", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
			{Name: "Tag", Context: "Blog", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		},
	}
	props := []Property{
		{Name: "Title", CSharpType: "string"},
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
	path := "files/tests-integration-relations/tests/{{ .Project }}.IntegrationTests/{{ .Aggregate }}RelationApiTests.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"using Demo.Application;",
		"public class PostRelationApiTests",
		`new CustomerRequest("Name sample 1")`,
		`new TagRequest("Name sample 1")`,
		`new PostRequest("Title sample 1", customer.Id)`,
		`new PostTagRequest(post.Id, tag1.Id)`,
		`$"/api/blog/post-tag/search?postId={ post.Id }&page=1&pageSize=100"`,
		`$"/api/blog/post/search?customerId={ customer.Id }&page=1&pageSize=100"`,
		"Assert.Equal(2, TagLinks!.Items.Count);",
		"Assert.Single(CustomerMatch!.Items);",
		// nested relation search via the display property
		`$"/api/blog/post/search?customerNameContains={Uri.EscapeDataString("Name sample")}&page=1&pageSize=100"`,
		"Assert.Single(CustomerNameMatch!.Items);",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostRelationApiTests rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}
