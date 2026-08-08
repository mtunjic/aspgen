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
		"public override string EntityName => \"Post\";",
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
		// dm-tier sync is one transactional service call
		"Store.ReplaceTags(saved.Id, selectedTag);",
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
		// per-field errors: FieldErrors dictionary + extraction of the server's
		// FluentValidation failures without referencing the type
		"public IReadOnlyDictionary<string, string> FieldErrors => fieldErrors;",
		"private bool ApplyValidationErrors(Exception ex)",
		"protected void SetFieldError(string property, string message)",
		"Please fix the highlighted fields.",
		// honest error split: FK violations hint at a missing relation,
		// everything else is reported generically (details only in the log)
		"private static bool IsForeignKeyFailure(Exception ex)",
		`? $"Could not save {EntityName}. You must add its related record(s) first."`,
		`: $"Could not save {EntityName}. Something went wrong; the details have been logged.";`,
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
		// the list view is now a thin composition of the shared ListPage:
		// only the entity-specific filter fields + row card template remain.
		"<controls:ListPage>",
		"<controls:ListPage.FilterFields>",
		"<controls:ListPage.RowTemplate>",
		// search + advanced filters chrome moved into the shared control
		`Command="{Binding DataContext.EditCommand, RelativeSource={RelativeSource AncestorType=UserControl}}"`,
		`Command="{Binding DataContext.DeleteCommand, RelativeSource={RelativeSource AncestorType=UserControl}}"`,
		`ItemsSource="{Binding CustomerItems}"`,
		`SelectedValue="{Binding Filter.CustomerId, Mode=TwoWay}"`,
		`Text="{Binding Filter.TitleContains, UpdateSourceTrigger=PropertyChanged}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostView.xaml (list) rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The old DataGrid, the inline edit form, and the chrome that now lives
	// in ListPage must all be gone from the module view.
	for _, forbidden := range []string{"DataGrid", "ItemsSource=\"{Binding TagOptions}\"", "Form.Title", "ValidationMessage", "Advanced filters", "SearchText", "ui:ListView"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("PostView.xaml (list) must not contain %q\n--- rendered ---\n%s", forbidden, out)
		}
	}
}

func TestRenderWpfListPage(t *testing.T) {
	for _, theme := range []string{"wpfui", ""} {
		d := m2mData()
		d.Theme = theme
		out := renderTemplate(t, "files/wpf/src/Desktop/Shared/Controls/ListPage.xaml.tmpl", d)
		for _, expected := range []string{
			// shared list chrome binds straight to the ListViewModelBase
			`ItemsSource="{Binding Items}"`,
			`SelectedItem="{Binding SelectedItem, Mode=TwoWay}"`,
			"SearchText",
			"Advanced filters",
			`Command="{Binding PrevPageCommand}"`,
			`Command="{Binding NextPageCommand}"`,
			`Command="{Binding NewCommand}"`,
			// aggregate-specific slots are fed by the module
			"FilterFields",
			"RowTemplate",
			`Text="{Binding PageLabel}"`,
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("ListPage.xaml (%q) is missing %q\n--- rendered ---\n%s", theme, expected, out)
			}
		}
	}
}

func TestRenderWpfEditPage(t *testing.T) {
	for _, theme := range []string{"wpfui", ""} {
		d := m2mData()
		d.Theme = theme
		out := renderTemplate(t, "files/wpf/src/Desktop/Shared/Controls/EditPage.xaml.tmpl", d)
		for _, expected := range []string{
			`Text="{Binding Title}"`,
			`Command="{Binding SaveCommand}"`,
			`Command="{Binding CancelCommand}"`,
			"ValidationMessage",
			"FormContent",
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("EditPage.xaml (%q) is missing %q\n--- rendered ---\n%s", theme, expected, out)
			}
		}
		// wpfui renders the validation as an InfoBar gated on HasValidationError;
		// the default theme renders a plain inline TextBlock.
		if theme == "wpfui" && !strings.Contains(out, "HasValidationError") {
			t.Errorf("EditPage.xaml (wpfui) is missing HasValidationError\n%s", out)
		}
	}
}

func TestRenderWpfDetailsPage(t *testing.T) {
	for _, theme := range []string{"wpfui", ""} {
		d := m2mData()
		d.Theme = theme
		out := renderTemplate(t, "files/wpf/src/Desktop/Shared/Controls/DetailsPage.xaml.tmpl", d)
		for _, expected := range []string{
			`Text="{Binding EntityName, StringFormat={}{0} details}"`,
			`Command="{Binding BackCommand}"`,
			"FieldsContent",
			// not-found empty state: hide the card, show "{Entity} not found"
			`Visibility="{Binding IsFound, Converter={StaticResource BoolToVisibility}}"`,
			`StringFormat={}{0} not found}`,
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("DetailsPage.xaml (%q) is missing %q\n--- rendered ---\n%s", theme, expected, out)
			}
		}
		if theme == "wpfui" && !strings.Contains(out, "IsOpen=\"{Binding IsNotFound}\"") {
			t.Errorf("DetailsPage.xaml (wpfui) is missing the InfoBar empty state\n%s", out)
		}
	}
}

func TestRenderWpfDetailsViewModelBase(t *testing.T) {
	out := renderTemplate(t, "files/wpf/src/Desktop/Shared/DetailsViewModelBase.cs.tmpl", m2mData())
	for _, expected := range []string{
		"public abstract class DetailsViewModelBase<TStore, TRow> : BindableBase, INavigationAware",
		"protected abstract void OnItemLoaded(long id);",
		"public DelegateCommand BackCommand { get; }",
		"Navigation.GoTo(ListViewName)",
		"public abstract string EntityName { get; }",
		"public bool IsNotFound => item is null;",
		"public bool IsFound => item is not null;",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("DetailsViewModelBase is missing %q\n--- rendered ---\n%s", expected, out)
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
		// the form is hosted by the shared EditPage (Save/Cancel/validation
		// live there, not in the module view)
		"<controls:EditPage>",
		"<controls:EditPage.FormContent>",
		// each field renders an inline error label bound to its FieldErrors entry
		`Text="{Binding FieldErrors[Title]}"`,
		`Text="{Binding FieldErrors[Body]}"`,
		`Text="{Binding FieldErrors[CustomerId]}"`,
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
		`@inherits CrudPageBase<PostView>`,
		`CreatePath="/blog/posts/edit"`,
		"filterTitleContains",
		"filterCustomerId",
		"filterCustomerNameContains",
		"ViewDetails",
		"DeleteAsync",
		// the page composes the shared components, not raw chrome
		"<SearchFilterBar",
		"<EntityTable",
		"<PaginationBar",
		// failed loads/deletes are surfaced to the user, not just logged
		"errorMessage is not null",
		`<div class="alert alert-danger" role="alert">@errorMessage</div>`,
		`Navigation.NavigateTo($"{ListPath}/{item.Id}")`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostCrud.razor (list) rendering is missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	// The shared chrome + logic live in the Components/Shared layer, not here.
	for _, forbidden := range []string{"EditForm", "TagOptions", "SyncTagsAsync", "form.", "Advanced filters", "LoadPageAsync", "protected const int PageSize"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("PostCrud.razor (list) must not contain %q\n--- rendered ---\n%s", forbidden, out)
		}
	}
}

func TestRenderBlazorSharedCrudComponents(t *testing.T) {
	d := m2mData()
	for _, tc := range []struct {
		path string
		want []string
	}{
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/SearchFilterBar.razor.tmpl",
			want: []string{"Advanced filters", `@onclick="ApplyAsync"`, `@onclick="ClearAsync"`, `field.Kind`, "relation", "number", "date", "bool"},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/EntityTable.razor.tmpl",
			want: []string{"@typeparam TItem", "table table-hover align-middle mb-0", `OnEdit.InvokeAsync`, `OnDelete.InvokeAsync`, "No records found.", `colspan="99"`},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/PaginationBar.razor.tmpl",
			want: []string{"Load more", "HasMore", "OnLoadMore"},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/CrudHeader.razor.tmpl",
			want: []string{"breadcrumb", "CreatePath", "+ Create new"},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/CrudPageBase.cs.tmpl",
			want: []string{"public abstract class CrudPageBase<TItem> : ComponentBase", "protected abstract Task<(List<TItem> Items, long TotalCount)> FetchPageAsync(int page)", "protected async Task SearchAsync()", "protected async Task LoadMoreAsync()", "protected async Task DeleteAsync(long id)", "protected const int PageSize = 25", "protected bool hasMore"},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/FilterField.cs.tmpl",
			want: []string{"public sealed class FilterField", "public Func<object?> GetValue", "public Action<object?> SetValue", "public sealed record FilterOption(long Id, string Display)"},
		},
		{
			path: "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/EntityColumn.cs.tmpl",
			want: []string{"public sealed class EntityColumn<TItem>", "public Func<TItem, object?> Display"},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			out := renderTemplate(t, tc.path, d)
			for _, expected := range tc.want {
				if !strings.Contains(out, expected) {
					t.Errorf("rendering is missing %q\n--- rendered ---\n%s", expected, out)
				}
			}
		})
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
		// failed saves are surfaced to the user, not silently dropped
		"private string? errorMessage;",
		"errorMessage = \"Could not save Post. Please check the form and try again.\";",
		// a failed/not-found record load surfaces a friendly message instead of
		// a silent empty form
		"errorMessage = \"This Post could not be found. It may have been deleted.\";",
		"errorMessage = \"Could not load this Post. Please try again.\";",
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
		// a failed/not-found load surfaces a friendly state, not an infinite
		// "Loading..." or a raw error page
		"private bool loadFailed;",
		`<div class="alert alert-danger" role="alert">This Post could not be found. It may have been deleted, or the service is unavailable.</div>`,
		"catch (Exception)",
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
		// the join rows are replaced in ONE transactional service call
		"private Task SyncTagsAsync(long PostId, long[] selectedTagIds, CancellationToken cancellationToken) =>",
		"service.ReplaceTagsAsync(PostId, selectedTagIds, cancellationToken);",
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
		`new CustomerCrudService(dbFactory, new CustomerValidator()).CreateAsync(new CustomerRequest("Name sample 1"))`,
		`new TagCrudService(dbFactory, new TagValidator()).CreateAsync(new TagRequest("Name sample 1"))`,
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

func TestQuickAddDefaultNullableFk(t *testing.T) {
	// A nullable FK must default to null, never 0: saving with 0 violates the
	// (optional) foreign-key constraint and, on the shared desktop DbContext,
	// poisons every later save.
	cases := map[string]string{
		"long?  fk":   quickAddDefault(Property{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer"}),
		"long   fk":   quickAddDefault(Property{Name: "RegionId", CSharpType: "long", RelationTarget: "Region"}),
		"int?":        quickAddDefault(Property{Name: "Age", CSharpType: "int?"}),
		"decimal?":    quickAddDefault(Property{Name: "Price", CSharpType: "decimal?"}),
		"guid?":       quickAddDefault(Property{Name: "Ref", CSharpType: "Guid?"}),
		"bool":        quickAddDefault(Property{Name: "Active", CSharpType: "bool"}),
		"string?":     quickAddDefault(Property{Name: "Notes", CSharpType: "string?"}),
		"date (none)": quickAddDefault(Property{Name: "When", CSharpType: "DateOnly"}),
	}
	want := map[string]string{
		"long?  fk": "null",
		"long   fk": "0",
		"int?":      "null",
		"decimal?":  "null",
		"guid?":     "null",
		"bool":      "false",
		"string?":   "null",
		"date (none)": "",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("quickAddDefault(%s) = %q, want %q", name, got, want[name])
		}
	}
}

func TestBuildRelationQuickAddsNullableFk(t *testing.T) {
	// Post has an OPTIONAL customer:Customer? -- the quick-add must create it
	// with CustomerId = null so the optional FK saves cleanly.
	entities := []EntityMeta{
		{Name: "Customer", Properties: []Property{{Name: "Name", CSharpType: "string"}}},
		{Name: "Post", Properties: []Property{
			{Name: "Title", CSharpType: "string"},
			{Name: "Body", CSharpType: "string?"},
			{Name: "CustomerId", CSharpType: "long?", RelationTarget: "Customer"},
		}},
	}
	quickAdds, _ := buildRelationQuickAdds(
		[]Relation{{Name: "Post", Target: "Post", DisplayProperty: "Title"}},
		entities)
	if got := quickAdds["Post"]; got != "new PostRow(0, name, null, null)" {
		t.Fatalf("Post quick-add (optional customer FK) = %q, want null FK default", got)
	}
}

func TestRenderDatabaseSaveSafety(t *testing.T) {
	for _, path := range []string{
		"files/dm/src/{{ .Project }}.Persistence/{{ .Project }}Database.cs.tmpl",
		"files/cqrs/src/{{ .Project }}.Persistence/{{ .Project }}Database.cs.tmpl",
	} {
		out := renderTemplate(t, path, m2mData())
		for _, expected := range []string{
			// a failed save resets the change tracker so one bad row can't
			// poison later saves on the shared singleton desktop DbContext
			"await SaveChangesSafelyAsync(cancellationToken);",
			"public async Task<int> SaveChangesSafelyAsync(CancellationToken cancellationToken = default)",
			"ChangeTracker.Clear();",
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("%s missing %q\n--- rendered ---\n%s", path, expected, out)
			}
		}
	}
}

func TestRenderCrudServiceUsesSafeSave(t *testing.T) {
	out := renderTemplate(t, "files/dm-crud/src/{{ .Project }}.Application/{{ .Aggregate }}CrudService.cs.tmpl", m2mData())
	if !strings.Contains(out, "await database.SaveChangesSafelyAsync(cancellationToken);") {
		t.Errorf("CrudService.CreateAsync must use the safe save that clears the tracker on failure\n--- rendered ---\n%s", out)
	}
	// The transactional m2m replace method legitimately uses the raw
	// SaveChangesAsync INSIDE its transaction (the transaction is the safety
	// net there); every other save path must stay on the safe helper.
	if !strings.Contains(out, "await database.SaveChangesAsync(cancellationToken);\n        await transaction.CommitAsync(cancellationToken);") {
		t.Errorf("CrudService.Replace* must save inside its transaction\n--- rendered ---\n%s", out)
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
		// clicking the dropdown flips it into editable mode; "+" commits.
		// The combo must NOT carry an x:Name (named elements inside a
		// UserControl's property content trip MC3093) -- the code-behind
		// finds it as the "+" button's sibling in the shared DockPanel.
		`Content="+"`,
		`Click="OnAddCustomerClick"`,
		`PreviewMouseLeftButtonDown="BeginCustomerQuickAdd"`,
		`DropDownOpened="OnCustomerDropDownOpened"`,
		`IsEditable="{Binding AddCustomerMode, Mode=TwoWay}"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEditView.xaml quick-add missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, `x:Name="customerCombo"`) {
		t.Errorf("PostEditView.xaml must not name the quick-add combo (MC3093 namescope conflict)\n%s", out)
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
		// "+" reads the typed text from the combo found via the sender's
		// DockPanel sibling (no x:Name, no binding)
		"private void OnAddCustomerClick(object sender, RoutedEventArgs e)",
		"sender is FrameworkElement { Parent: DockPanel panel }",
		"viewModel.AddCustomer(combo.Text)",
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
		"catch (Exception ex)",
		`AppLog.Error(ex, "Quick-add of {Target} failed for {Entity}", "Customer", "Post");`,
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
		// a failed quick-add is surfaced to the user, not silently dropped
		"errorMessage = \"Could not add Customer. Please try again.\";",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("PostEdit.razor quick-add missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderWpfAppSegoeFluentIcons(t *testing.T) {
	d := m2mData()
	d.Theme = "wpfui"
	out := renderTemplate(t, "files/wpf/src/Desktop/App.xaml.tmpl", d)
	// WPF UI's Fluent System Icons font lacks some Windows 11 icons; the
	// documented fix is a SegoeFluentIcons FontFamily resource pointing at the
	// system font so the missing glyphs render.
	for _, expected := range []string{
		`<FontFamily x:Key="SegoeFluentIcons">Segoe Fluent Icons</FontFamily>`,
		"pack://application:,,,/;component/Fonts/#Segoe Fluent Icons",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("wpfui App.xaml must register the Segoe Fluent Icons font (missing %q)\n%s", expected, out)
		}
	}
	d.Theme = ""
	out = renderTemplate(t, "files/wpf/src/Desktop/App.xaml.tmpl", d)
	if strings.Contains(out, "SegoeFluentIcons") {
		t.Errorf("non-wpfui App.xaml must not register the Fluent icon font\n%s", out)
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

func TestRenderTransactionalM2mSync(t *testing.T) {
	d := m2mData() // dm backend, tags:Tag[] join PostTag

	// The declaring aggregate's CrudService owns an atomic replace method.
	crud := renderTemplate(t, "files/dm-crud/src/{{ .Project }}.Application/{{ .Aggregate }}CrudService.cs.tmpl", d)
	for _, expected := range []string{
		"public async Task<CommandResponse> ReplaceTagsAsync(long postId, long[] selectedTagIds, CancellationToken cancellationToken = default)",
		"await using var transaction = await database.Database.BeginTransactionAsync(cancellationToken);",
		"database.PostTags.Where(x => x.PostId == postId)",
		"database.PostTags.RemoveRange(links.Where(l => !keep.Contains(l.TagId)));",
		"database.PostTags.Add(new PostTag(postId, id));",
		"await database.SaveChangesAsync(cancellationToken);",
		"await transaction.CommitAsync(cancellationToken);",
	} {
		if !strings.Contains(crud, expected) {
			t.Errorf("CrudService transactional m2m sync missing %q\n--- rendered ---\n%s", expected, crud)
		}
	}

	// dm WPF: the store exposes the sync and delegates to the service.
	istore := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Services/I{{ .Name }}Store.cs.tmpl", d)
	if !strings.Contains(istore, "void ReplaceTags(long postId, long[] selectedTagIds);") {
		t.Errorf("dm IStore missing ReplaceTags\n%s", istore)
	}
	store := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Services/{{ .Name }}Store.cs.tmpl", d)
	if !strings.Contains(store, "service.ReplaceTagsAsync(postId, selectedTagIds)") {
		t.Errorf("dm Store missing service.ReplaceTagsAsync\n%s", store)
	}

	// HTTP-backed (cqrs/es) WPF keeps the per-call loop and no store method.
	http := m2mData()
	http.Backend = "cqrs"
	httpIStore := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Services/I{{ .Name }}Store.cs.tmpl", http)
	if strings.Contains(httpIStore, "ReplaceTags") {
		t.Errorf("HTTP IStore must not declare ReplaceTags\n%s", httpIStore)
	}
	httpVM := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}EditViewModel.cs.tmpl", http)
	for _, expected := range []string{"new PostTagRow(0, saved.Id, id)", "postTagStore.Delete(link.Id);"} {
		if !strings.Contains(httpVM, expected) {
			t.Errorf("HTTP EditViewModel must keep the per-call m2m sync (missing %q)\n%s", expected, httpVM)
		}
	}
}
