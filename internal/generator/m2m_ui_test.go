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
		// the list view is now a thin subclass of the shared base
		"ListViewModelBase<IPostStore, PostRow, PostSearchCriteria, PostPageResult>",
		"private readonly ICustomerStore customerStore;",
		"private readonly ITagStore tagStore;",
		"public PostViewModel(IPostStore store, IAppNavigationService navigation, ICustomerStore customerStore, ITagStore tagStore",
		"protected override string EntityName => \"Post\";",
		`protected override string EditViewName => "PostEdit";`,
		`protected override string DetailsViewName => "PostDetails";`,
		"public ObservableCollection<CustomerRow> CustomerItems { get; } = [];",
		"protected override PostSearchCriteria BuildCriteria(int page)",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostViewModel (list) rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The list view model must NOT contain the shared plumbing (it lives in
	// the base) nor the edit form + m2m logic.
	for _, forbidden := range []string{"OnNavigatedTo", "TagOptions", "IPostTagStore", "Form.", "SaveCommand", "TryBuild", "DelegateCommand<PostRow> EditCommand"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("PostViewModel (list) must not contain %q\n--- rendered ---\n%s", forbidden, out)
		}
	}
}

func TestRenderWpfListBase(t *testing.T) {
	out := renderTemplate(t, "files/wpf/src/Desktop/Shared/ListViewModelBase.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public abstract class ListViewModelBase<TStore, TRow, TCriteria, TPage> : BindableBase, INavigationAware",
		"public void OnNavigatedTo(NavigationContext navigationContext)",
		"LoadRelated();",
		"protected abstract TCriteria BuildCriteria(int page);",
		"public DelegateCommand<TRow> EditCommand { get; }",
		"public DelegateCommand<TRow> DeleteCommand { get; }",
		"public string PageLabel => $\"Page {Page} of {TotalPages}\";",
		`Navigation.GoTo(EditViewName);`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("ListViewModelBase missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestWpfEditViewModelManyToManyRendering(t *testing.T) {
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}EditViewModel.cs.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		"EditViewModelBase<IPostStore, PostRow, PostEditor>",
		"public PostEditViewModel(IPostStore store, IAppNavigationService navigation, ICustomerStore customerStore, ITagStore tagStore, IPostTagStore postTagStore)",
		"public ObservableCollection<TagOption> TagOptions { get; } = [];",
		"new PostTagSearchCriteria(null, EditingId, null, 1, 1000)",
		"new PostTagRow(0, saved.Id, id)",
		"postTagStore.Delete(link.Id);",
		"protected override void SyncRelated(PostRow saved)",
		"protected override bool TryBuild(out PostRow value)",
		"public sealed class TagOption : BindableBase",
		"public sealed class PostEditor : BindableBase",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditViewModel rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfEditBase(t *testing.T) {
	out := renderTemplate(t, "files/wpf/src/Desktop/Shared/EditViewModelBase.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public abstract class EditViewModelBase<TStore, TRow, TForm> : BindableBase, INavigationAware",
		"protected abstract bool TryBuild(out TRow value);",
		"protected abstract void SyncRelated(TRow saved);",
		"public DelegateCommand SaveCommand { get; }",
		"public DelegateCommand CancelCommand { get; }",
		`ValidationMessage = $"Could not save {EntityName}. You must add its related record(s) first.";`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("EditViewModelBase missing %q\n--- rendered ---\n%s", expected, out)
		}
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
		"public PostDetailsViewModel(IPostStore store, IAppNavigationService navigation, ICustomerStore customerStore, ITagStore tagStore, IPostTagStore postTagStore)",
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
	d := m2mData()
	d.Theme = "wpfui"
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}View.xaml.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		// the list uses a WPF-UI ListView (plain ListView for non-wpfui)
		`<ui:ListView Margin="0,0,0,8" ItemsSource="{Binding Items}"`,
		`Command="{Binding DataContext.EditCommand, RelativeSource={RelativeSource AncestorType=UserControl}}"`,
		`Command="{Binding DataContext.DeleteCommand, RelativeSource={RelativeSource AncestorType=UserControl}}"`,
		// search + advanced filters kept on the first page
		"SearchText",
		"Advanced filters",
		// pagination
		`Command="{Binding PrevPageCommand}"`,
		`Command="{Binding NextPageCommand}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostView.xaml (list) rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The old DataGrid and the inline edit form must be gone.
	for _, forbidden := range []string{"DataGrid", "ItemsSource=\"{Binding TagOptions}\"", "Form.Title", "ValidationMessage"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("PostView.xaml (list) must not contain %q\n--- rendered ---\n%s", forbidden, out)
		}
	}
}

func TestWpfEditViewManyToManyRendering(t *testing.T) {
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}EditView.xaml.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		`Text="{Binding Form.Title, UpdateSourceTrigger=PropertyChanged}"`,
		`ItemsSource="{Binding CustomerItems}" DisplayMemberPath="Name" SelectedValuePath="Id" SelectedValue="{Binding Form.CustomerId, Mode=TwoWay}"`,
		// fields flow 2-up in a responsive WrapPanel
		"<WrapPanel>",
		`Width="320" Margin="0,0,16,14"`,
		`ItemsControl ItemsSource="{Binding TagOptions}"`,
		`IsChecked="{Binding IsSelected, Mode=TwoWay}"`,
		`Content="{Binding Display}"`,
		`Command="{Binding SaveCommand}"`,
		`Command="{Binding CancelCommand}"`,
		`Text="{Binding ValidationMessage}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditView.xaml rendering is missing %q\n--- rendered ---\n%s", expected, out)
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
		`@page "/blog/posts"`,
		`href="/blog/posts/edit"`,
		"Advanced filters",
		"filterTitleContains",
		"filterCustomerId",
		"filterCustomerNameContains",
		"ViewDetails",
		"DeleteAsync",
		`Navigation.NavigateTo($"/blog/posts/{id}")`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostCrud.razor (list) rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The inline edit form + m2m logic moved to the dedicated Edit page.
	for _, forbidden := range []string{"EditForm", "TagOptions", "SyncTagsAsync", "form."} {
		if strings.Contains(out, forbidden) {
			t.Errorf("PostCrud.razor (list) must not contain %q\n--- rendered ---\n%s", forbidden, out)
		}
	}
}

func TestBlazorEditManyToManyRendering(t *testing.T) {
	path := "files/blazor-context-crud/src/{{ .Project }}.AppBlazor/Components/Pages/{{ .Context }}/{{ .Aggregate }}Edit.razor.tmpl"
	out := renderTemplate(t, path, m2mData())
	for _, expected := range []string{
		`@page "/blog/posts/edit"`,
		`@page "/blog/posts/edit/{Id:long}"`,
		"@using Demo.Application.Features.Blog.PostTag",
		`<input type="checkbox" class="form-check-input" @bind="option.Selected" />`,
		"@option.Display",
		"private List<TagOption> TagOptions = [];",
		"await SyncTagsAsync(editingId);",
		"private async Task SyncTagsAsync(long PostId)",
		`$"/api/blog/post-tag/search?postId={ PostId }&pageSize=1000"`,
		"new PostTagPagedResponse([], 0, 1, 1000)",
		"new PostTagRequest(PostId, id)",
		"private sealed class TagOption",
		"private sealed class PostEditor",
		`<EditForm Model="form" OnValidSubmit="SaveAsync" FormName="PostEdit">`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEdit.razor rendering is missing %q\n--- rendered ---\n%s", expected, out)
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
	// For the API test, FK args reference the prerequisite View's id; scalar
	// props get seed literals.
	args := testRequestArgs([]Property{
		{Name: "RegionId", CSharpType: "long", RelationTarget: "Region"},
		{Name: "Name", CSharpType: "string"},
	})
	if len(args) != 2 || args[0] != "region.Id" || args[1] == "" {
		t.Fatalf("expected FK arg to reference the prereq View's id, got %#v", args)
	}
	// For the MVC test, a prereq is a raw long, so the FK arg is the var name.
	mvcArgs := testMvcArgs([]Property{
		{Name: "RegionId", CSharpType: "long", RelationTarget: "Region"},
		{Name: "Name", CSharpType: "string"},
	})
	if len(mvcArgs) != 2 || mvcArgs[0] != "region" {
		t.Fatalf("expected MVC FK arg to be the raw long var, got %#v", mvcArgs)
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
		`<base href="/" />`,
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

func TestBuildRelationQuickAdds(t *testing.T) {
	entities := []EntityMeta{
		{Name: "Customer", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		{Name: "Tag", Properties: []Property{{Name: "Name", CSharpType: "string"}, {Name: "Active", CSharpType: "bool"}}},
		{Name: "Event", Properties: []Property{{Name: "Name", CSharpType: "string"}, {Name: "StartsAt", CSharpType: "DateTime"}}},
		{Name: "Region", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		{Name: "Office", Properties: []Property{{Name: "Name", CSharpType: "string"}, {Name: "RegionId", CSharpType: "long", RelationTarget: "Region", RelationDisplayProperty: "Name"}}},
	}
	relations := []Relation{
		{Name: "Customer", Target: "Customer", DisplayProperty: "Name"},
		{Name: "Tag", Target: "Tag", DisplayProperty: "Name"},
		{Name: "Event", Target: "Event", DisplayProperty: "Name"},
		{Name: "Office", Target: "Office", DisplayProperty: "Name"},
	}
	quickAdds, missing := buildRelationQuickAdds(relations, entities)
	if quickAdds["Customer"] != "new CustomerRow(0, name)" {
		t.Errorf("Customer quick-add = %q", quickAdds["Customer"])
	}
	if quickAdds["Tag"] != "new TagRow(0, name, false)" {
		t.Errorf("Tag quick-add = %q", quickAdds["Tag"])
	}
	// A target with a required date can't be quick-added inline.
	if _, ok := quickAdds["Event"]; ok {
		t.Errorf("Event must not be quick-addable (required date): %q", quickAdds["Event"])
	}
	// A target with an FK names the entity that must exist first.
	if quickAdds["Office"] != "new OfficeRow(0, name, 0)" {
		t.Errorf("Office quick-add = %q", quickAdds["Office"])
	}
	if missing["Office"] != "Region" {
		t.Errorf("Office missing-entity = %q, want Region", missing["Office"])
	}
	if _, ok := missing["Customer"]; ok {
		t.Errorf("Customer has no FK, must not have a missing-entity entry")
	}
}

func TestRenderWpfEditViewQuickAdd(t *testing.T) {
	d := m2mData() // has customer:Customer (quick-addable) relation
	d.RelationQuickAdds = map[string]string{"Customer": "new CustomerRow(0, name)"}
	d.RelationQuickAddMissing = map[string]string{}
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}EditView.xaml.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		// clicking the dropdown flips it into editable mode; "+" commits
		`Content="+"`,
		`Click="OnAddCustomerClick"`,
		`x:Name="customerCombo"`,
		`PreviewMouseLeftButtonDown="BeginCustomerQuickAdd"`,
		`DropDownOpened="OnCustomerDropDownOpened"`,
		`IsEditable="{Binding AddCustomerMode, Mode=TwoWay}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditView.xaml quick-add missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, "NewCustomerName") {
		t.Errorf("PostEditView.xaml must not rely on a Text binding for the quick-add\n%s", out)
	}
}

func TestRenderWpfEditViewCodeBehindQuickAdd(t *testing.T) {
	d := m2mData() // has customer:Customer (quick-addable) relation
	d.RelationQuickAdds = map[string]string{"Customer": "new CustomerRow(0, name)"}
	d.RelationQuickAddMissing = map[string]string{}
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Views/{{ .Name }}EditView.xaml.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"private void BeginCustomerQuickAdd(object sender, MouseButtonEventArgs e)",
		"viewModel.BeginCustomerQuickAdd();",
		// list stays closed while typing after entering edit mode
		"private void OnCustomerDropDownOpened(object sender, System.EventArgs e)",
		"((ComboBox)sender).IsDropDownOpen = false;",
		// "+" reads the typed text straight from the combo
		"private void OnAddCustomerClick(object sender, RoutedEventArgs e)",
		"viewModel.AddCustomer(customerCombo.Text)",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditView.xaml.cs quick-add handler missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfEditViewModelQuickAddCatch(t *testing.T) {
	d := m2mData() // has customer:Customer (quick-addable relation)
	d.RelationQuickAdds = map[string]string{"Customer": "new CustomerRow(0, name)"}
	d.RelationQuickAddMissing = map[string]string{}
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}EditViewModel.cs.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"public bool AddCustomer(string name)",
		"catch (Exception)",
		`ValidationMessage = "Could not add Customer. You must add its related record(s) first.";`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditViewModel quick-add catch missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfEditViewModelQuickAddSpecificMessage(t *testing.T) {
	d := m2mData() // has customer:Customer (quick-addable relation)
	d.RelationQuickAdds = map[string]string{"Office": "new OfficeRow(0, name, 0)"}
	d.RelationQuickAddMissing = map[string]string{"Office": "Region"}
	// The edit view only renders quick-add blocks for relations present in
	// d.Relations; inject a relation so the block renders.
	d.Relations = append(d.Relations, Relation{Name: "Office", Target: "Office", FKProperty: "OfficeId", DisplayProperty: "Name", Optional: false})
	path := "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}EditViewModel.cs.tmpl"
	out := renderTemplate(t, path, d)
	if !strings.Contains(out, `ValidationMessage = "Could not add Office. You must add Region first.";`) {
		t.Errorf("PostEditViewModel must name the missing entity in the quick-add message\n--- rendered ---\n%s", out)
	}
}

func TestBlazorEditQuickAdd(t *testing.T) {
	d := m2mData() // has customer:Customer (quick-addable) relation
	d.RelationBlazorQuickAdds = map[string]string{"Customer": "new CustomerRequest(newCustomerName)"}
	path := "files/blazor-context-crud/src/{{ .Project }}.AppBlazor/Components/Pages/{{ .Context }}/{{ .Aggregate }}Edit.razor.tmpl"
	out := renderTemplate(t, path, d)
	for _, expected := range []string{
		"showAddCustomer",
		"newCustomerName",
		"private async Task AddCustomerAsync()",
		`await Http.PostAsJsonAsync("/api/blog/customer", new CustomerRequest(newCustomerName))`,
		"form.CustomerId = created.Id;",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEdit.razor quick-add missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfAppStyles(t *testing.T) {
	d := m2mData()
	d.Theme = "wpfui"
	out := renderTemplate(t, "files/wpf/src/Desktop/Themes/AppStyles.xaml.tmpl", d)
	for _, expected := range []string{
		`x:Key="CardStyle"`,
		`x:Key="CardSecondaryStyle"`,
		`x:Key="SectionHeaderStyle"`,
		`x:Key="ListHeaderStyle"`,
		`x:Key="HeaderBandStyle"`,
		"CardBackgroundFillColorDefaultBrush",
		"ControlFillColorSecondaryBrush",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("AppStyles.xaml missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfNavigationService(t *testing.T) {
	out := renderTemplate(t, "files/wpf/src/Desktop/Shared/NavigationService.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public interface IAppNavigationService",
		"void GoTo(string viewName);",
		"void GoTo(string viewName, long id);",
		"public sealed class AppNavigationService : IAppNavigationService",
		`regionManager.RequestNavigate("MainRegion", viewName, new NavigationParameters { { "id", id } });`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("NavigationService missing %q\n--- rendered ---\n%s", expected, out)
		}
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
