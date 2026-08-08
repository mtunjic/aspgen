package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertExists(t *testing.T, root, relative string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
		t.Errorf("expected generated file %s: %v", relative, err)
	}
}

func TestManyToManyWpfUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "WpfM2MDemo")
	if err := Run([]string{"new", "WpfM2MDemo", "--context", "Blog", "--arch", "dm", "-ui", "wpf", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertM2mWpfModule(t, project, "Post")

	// retrofit case: aggregates added first, -ui wpf attached afterwards -
	// the many-to-many metadata must be recovered from the manifest.
	retrofit := filepath.Join(t.TempDir(), "WpfM2MRetroDemo")
	if err := Run([]string{"new", "WpfM2MRetroDemo", "--context", "Blog", "--arch", "dm", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "wpf", "--framework", "wpf", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertM2mWpfModule(t, retrofit, "Post")
}

func assertM2mWpfModule(t *testing.T, project, aggregate string) {
	t.Helper()
	view, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "Views", aggregate+"View.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The list view is a ListView (not a DataGrid) with per-row edit/delete,
	// and no inline edit form / many-to-many picker.
	for _, expected := range []string{"ListView", `DataContext.EditCommand`, `DataContext.DeleteCommand`, "Advanced filters"} {
		if !strings.Contains(string(view), expected) {
			t.Fatalf("%sView.xaml list rendering missing %q: %s", aggregate, expected, view)
		}
	}
	for _, forbidden := range []string{"DataGrid", "TagOptions"} {
		if strings.Contains(string(view), forbidden) {
			t.Fatalf("%sView.xaml must not contain %q: %s", aggregate, forbidden, view)
		}
	}
	vm, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "ViewModels", aggregate+"ViewModel.cs"))
	if err != nil {
		t.Fatal(err)
	}
	vmText := string(vm)
	if !strings.Contains(vmText, `EditViewName => "PostEdit"`) || !strings.Contains(vmText, "ListViewModelBase<IPostStore, PostRow, PostSearchCriteria, PostPageResult>") {
		t.Fatalf("%sViewModel.cs list wiring missing edit navigation: %s", aggregate, vmText)
	}
	// The many-to-many picker + sync now live in the dedicated edit view.
	editView, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "Views", aggregate+"EditView.xaml"))
	if err != nil {
		t.Fatalf("%sEditView.xaml was not generated: %v", aggregate, err)
	}
	for _, expected := range []string{"ItemsSource=\"{Binding TagOptions}\"", "IsChecked=\"{Binding IsSelected, Mode=TwoWay}\"", "Command=\"{Binding SaveCommand}\"", "Command=\"{Binding CancelCommand}\""} {
		if !strings.Contains(string(editView), expected) {
			t.Fatalf("%sEditView.xaml multi-select missing %q: %s", aggregate, expected, editView)
		}
	}
	editVM, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "ViewModels", aggregate+"EditViewModel.cs"))
	if err != nil {
		t.Fatalf("%sEditViewModel.cs was not generated: %v", aggregate, err)
	}
	editVMText := string(editVM)
	for _, expected := range []string{
		"IPostTagStore postTagStore",
		"ObservableCollection<TagOption> TagOptions { get; }",
		"new PostTagSearchCriteria(null, EditingId, null, 1, 1000)",
		"new PostTagRow(0, saved.Id, id)",
		"EditViewModelBase<IPostStore, PostRow, PostEditor>",
		"public sealed class TagOption : BindableBase",
	} {
		if !strings.Contains(editVMText, expected) {
			t.Fatalf("%sEditViewModel.cs missing %q", aggregate, expected)
		}
	}
	detailsVM, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "ViewModels", aggregate+"DetailsViewModel.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailsVM), "TagsDisplay") || !strings.Contains(string(detailsVM), "IPostTagStore postTagStore") {
		t.Fatalf("%sDetailsViewModel.cs missing many-to-many display wiring: %s", aggregate, detailsVM)
	}
	detailsView, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", aggregate, "Views", aggregate+"DetailsView.xaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailsView), "Text=\"{Binding TagsDisplay}\"") {
		t.Fatalf("%sDetailsView.xaml missing many-to-many display: %s", aggregate, detailsView)
	}
}

func TestManyToManyBlazorUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "BlazorM2MDemo")
	if err := Run([]string{"new", "BlazorM2MDemo", "--context", "Blog", "--arch", "cqrs", "-ui", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(project, "src", "BlazorM2MDemo.AppBlazor", "Components", "Pages", "Blog", "PostCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	pageText := string(page)
	for _, expected := range []string{
		// the list page is now form-free: search + filters + a table
		`@page "/blog/posts"`,
		`href="/blog/posts/edit"`,
		"Advanced filters",
		"filterTitleContains",
		"ViewDetails",
		"DeleteAsync",
	} {
		if !strings.Contains(pageText, expected) {
			t.Fatalf("PostCrud.razor missing %q: %s", expected, page)
		}
	}
	if strings.Contains(pageText, "EditForm") {
		t.Fatalf("PostCrud.razor must not contain the inline edit form: %s", page)
	}
	// the create/edit form moved to its own page
	editPage, err := os.ReadFile(filepath.Join(project, "src", "BlazorM2MDemo.AppBlazor", "Components", "Pages", "Blog", "PostEdit.razor"))
	if err != nil {
		t.Fatalf("PostEdit.razor was not generated: %v", err)
	}
	editText := string(editPage)
	for _, expected := range []string{
		`@page "/blog/posts/edit"`,
		`@page "/blog/posts/edit/{Id:long}"`,
		"type=\"checkbox\" class=\"form-check-input\" @bind=\"option.Selected\"",
		"private List<TagOption> TagOptions = [];",
		"await SyncTagsAsync(editingId);",
		"private async Task SyncTagsAsync(long PostId)",
		"new PostTagRequest(PostId, id)",
		"private sealed class TagOption",
		"new PostTagPagedResponse([], 0, 1, 1000)",
	} {
		if !strings.Contains(editText, expected) {
			t.Fatalf("PostEdit.razor missing %q: %s", expected, editPage)
		}
	}
	details, err := os.ReadFile(filepath.Join(project, "src", "BlazorM2MDemo.AppBlazor", "Components", "Pages", "Blog", "PostDetails.razor"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"selectedTagIds", "TagOptions.Where(x => selectedTagIds.Contains(x.Id))"} {
		if !strings.Contains(string(details), expected) {
			t.Fatalf("PostDetails.razor missing %q: %s", expected, details)
		}
	}
}

func TestManyToManyMvcUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MvcM2MDemo")
	if err := Run([]string{"new", "MvcM2MDemo", "--context", "Blog", "--arch", "dm", "-ui", "mvc", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	controller, err := os.ReadFile(filepath.Join(project, "src", "MvcM2MDemo.WebMvc", "Controllers", "PostController.cs"))
	if err != nil {
		t.Fatal(err)
	}
	controllerText := string(controller)
	for _, expected := range []string{
		"PostTagCrudService postTagService",
		"long[]? selectedTagIds",
		"await SyncTagsAsync(created.Id, selectedTagIds ?? [], cancellationToken);",
		"private async Task SyncTagsAsync(long PostId, long[] selectedTagIds, CancellationToken cancellationToken)",
		"new PostTagRequest(PostId, id)",
		"ViewBag.TagSelectedIds",
	} {
		if !strings.Contains(controllerText, expected) {
			t.Fatalf("PostController.cs missing %q: %s", expected, controller)
		}
	}
	create, err := os.ReadFile(filepath.Join(project, "src", "MvcM2MDemo.WebMvc", "Views", "Post", "Create.cshtml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"name=\"selectedTagIds\"", "checked=\"@(((List<long>)ViewBag.TagSelectedIds).Contains(option.Id))\""} {
		if !strings.Contains(string(create), expected) {
			t.Fatalf("Create.cshtml missing %q: %s", expected, create)
		}
	}
	details, err := os.ReadFile(filepath.Join(project, "src", "MvcM2MDemo.WebMvc", "Views", "Post", "Details.cshtml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(details), "ViewBag.TagSelectedIds") {
		t.Fatalf("Details.cshtml missing joined-names display: %s", details)
	}
}

func TestRelationUnitTestGenerated(t *testing.T) {
	project := filepath.Join(t.TempDir(), "RelTestDemo")
	if err := Run([]string{"new", "RelTestDemo", "--context", "Blog", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Customer", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "body:string?", "customer:Customer?", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}

	testFile, err := os.ReadFile(filepath.Join(project, "tests", "RelTestDemo.UnitTests", "PostRelationTests.cs"))
	if err != nil {
		t.Fatalf("relation unit test was not generated: %v", err)
	}
	testText := string(testFile)
	for _, expected := range []string{
		"public class PostRelationTests",
		`var post = new Post("Title sample 1", "Body sample 1", customer.Id);`,
		"db.PostTags.Add(new PostTag(post.Id, tag1.Id));",
		"Assert.Equal(2, TagLinks.Count);",
		"Assert.Single(CustomerMatch);",
	} {
		if !strings.Contains(testText, expected) {
			t.Fatalf("PostRelationTests.cs missing %q: %s", expected, testFile)
		}
	}
	csproj, err := os.ReadFile(filepath.Join(project, "tests", "RelTestDemo.UnitTests", "RelTestDemo.UnitTests.csproj"))
	if err != nil || !strings.Contains(string(csproj), `<Compile Update="PostRelationTests.cs" />`) {
		t.Fatalf("relation unit test not registered in the tests csproj: %v", err)
	}

	// A relation-less aggregate must NOT get a relation test.
	if err := Run([]string{"add", "aggregate", "PlainNote", "text:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(project, "tests", "RelTestDemo.UnitTests", "PlainNoteRelationTests.cs")); !os.IsNotExist(err) {
		t.Fatalf("relation-less aggregate should not get a relation test: %v", err)
	}
}

func TestRelationApiTestGenerated(t *testing.T) {
	project := filepath.Join(t.TempDir(), "ApiRelTestDemo")
	if err := Run([]string{"new", "ApiRelTestDemo", "--context", "Blog", "--arch", "cqrs", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	apiTest, err := os.ReadFile(filepath.Join(project, "tests", "ApiRelTestDemo.IntegrationTests", "PostRelationApiTests.cs"))
	if err != nil {
		t.Fatalf("relation WebApi integration test was not generated: %v", err)
	}
	for _, expected := range []string{
		"public class PostRelationApiTests",
		`$"/api/blog/post-tag/search?postId={ post.Id }&page=1&pageSize=100"`,
		"new PostTagRequest(post.Id, tag1.Id)",
	} {
		if !strings.Contains(string(apiTest), expected) {
			t.Fatalf("PostRelationApiTests.cs missing %q: %s", expected, apiTest)
		}
	}
	csproj, err := os.ReadFile(filepath.Join(project, "tests", "ApiRelTestDemo.IntegrationTests", "ApiRelTestDemo.IntegrationTests.csproj"))
	if err != nil || !strings.Contains(string(csproj), `<Compile Update="PostRelationApiTests.cs" />`) {
		t.Fatalf("relation integration test not registered in the csproj: %v", err)
	}
}

func TestRelationMvcTestGenerated(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MvcRelTestDemo")
	if err := Run([]string{"new", "MvcRelTestDemo", "--context", "Blog", "--arch", "dm", "-ui", "mvc", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	// dm-tier MVC projects must get an IntegrationTests project (the WebMvc
	// is a real ASP.NET Core host) plus the per-aggregate relation test.
	mvcTest, err := os.ReadFile(filepath.Join(project, "tests", "MvcRelTestDemo.IntegrationTests", "PostMvcRelationTests.cs"))
	if err != nil {
		t.Fatalf("MVC relation integration test was not generated: %v", err)
	}
	for _, expected := range []string{
		"public class PostMvcRelationTests",
		`new KeyValuePair<string, string>("selectedTagIds", tag1.ToString()),`,
		`PostAsync("/blog/post/create", form)`,
	} {
		if !strings.Contains(string(mvcTest), expected) {
			t.Fatalf("PostMvcRelationTests.cs missing %q: %s", expected, mvcTest)
		}
	}
	csproj, err := os.ReadFile(filepath.Join(project, "tests", "MvcRelTestDemo.IntegrationTests", "MvcRelTestDemo.IntegrationTests.csproj"))
	if err != nil || !strings.Contains(string(csproj), "Microsoft.AspNetCore.Mvc.Testing") {
		t.Fatalf("MVC integration test project was not generated: %v", err)
	}
	solution, err := os.ReadFile(filepath.Join(project, "MvcRelTestDemo.sln"))
	if err != nil || !strings.Contains(string(solution), "MvcRelTestDemo.IntegrationTests") {
		t.Fatalf("MVC integration test project not in solution: %v", err)
	}
	// The WebMvc host must expose Program for WebApplicationFactory and
	// bootstrap its schema so it can actually serve queries.
	program, err := os.ReadFile(filepath.Join(project, "src", "MvcRelTestDemo.WebMvc", "Program.cs"))
	if err != nil || !strings.Contains(string(program), "public partial class Program") || !strings.Contains(string(program), "EnsureCreated") {
		t.Fatalf("WebMvc Program.cs missing WebApplicationFactory/schema bootstrap: %v", err)
	}
}

func TestMvcNavAndHomeRedirect(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MvcNavDemo")
	if err := Run([]string{"new", "MvcNavDemo", "--context", "Blog", "--arch", "dm", "-ui", "mvc", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}

	layout, err := os.ReadFile(filepath.Join(project, "src", "MvcNavDemo.WebMvc", "Views", "Shared", "_Layout.cshtml"))
	if err != nil {
		t.Fatalf("MVC layout was not generated (was it excluded from embed?): %v", err)
	}
	for _, expected := range []string{
		`asp-controller="Tag" asp-action="Index">Tag</a>`,
		`asp-controller="Post" asp-action="Index">Post</a>`,
		"bootstrap@5.3.3",
	} {
		if !strings.Contains(string(layout), expected) {
			t.Fatalf("_Layout.cshtml missing %q: %s", expected, layout)
		}
	}

	home, err := os.ReadFile(filepath.Join(project, "src", "MvcNavDemo.WebMvc", "Controllers", "HomeController.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), `RedirectToAction("Index", "Tag")`) {
		t.Fatalf("HomeController should redirect to the first aggregate: %s", home)
	}

	// retrofit: aggregates added first, -ui mvc attached later - nav + redirect
	// must be patched for the pre-existing aggregates too.
	retrofit := filepath.Join(t.TempDir(), "MvcNavRetroDemo")
	if err := Run([]string{"new", "MvcNavRetroDemo", "--context", "Blog", "--arch", "dm", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "mvc", "--framework", "mvc", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	layout, err = os.ReadFile(filepath.Join(retrofit, "src", "MvcNavRetroDemo.WebMvc", "Views", "Shared", "_Layout.cshtml"))
	if err != nil {
		t.Fatalf("retrofit MVC layout missing: %v", err)
	}
	if !strings.Contains(string(layout), `asp-controller="Tag"`) || !strings.Contains(string(layout), `asp-controller="Post"`) {
		t.Fatalf("retrofit layout missing nav links: %s", layout)
	}
}

func TestBlazorNavLinks(t *testing.T) {
	project := filepath.Join(t.TempDir(), "BlazorNavDemo")
	if err := Run([]string{"new", "BlazorNavDemo", "--context", "Blog", "--arch", "cqrs", "-ui", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	layout, err := os.ReadFile(filepath.Join(project, "src", "BlazorNavDemo.AppBlazor", "Components", "Layout", "MainLayout.razor"))
	if err != nil {
		t.Fatalf("Blazor MainLayout was not generated: %v", err)
	}
	for _, expected := range []string{
		`href="/blog/tags">Tags</NavLink>`,
		`href="/blog/posts">Posts</NavLink>`,
		"bootstrap",
	} {
		if !strings.Contains(string(layout), expected) {
			t.Fatalf("MainLayout.razor missing %q: %s", expected, layout)
		}
	}

	// retrofit: aggregates first, -ui blazor attached later - nav must be
	// patched for the pre-existing aggregates too.
	retrofit := filepath.Join(t.TempDir(), "BlazorNavRetroDemo")
	if err := Run([]string{"new", "BlazorNavRetroDemo", "--context", "Blog", "--arch", "cqrs", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "blazor", "--framework", "blazor", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	layout, err = os.ReadFile(filepath.Join(retrofit, "src", "BlazorNavRetroDemo.AppBlazor", "Components", "Layout", "MainLayout.razor"))
	if err != nil {
		t.Fatalf("retrofit Blazor MainLayout missing: %v", err)
	}
	if !strings.Contains(string(layout), `href="/blog/tags">Tags`) || !strings.Contains(string(layout), `href="/blog/posts">Posts`) {
		t.Fatalf("retrofit MainLayout missing nav links: %s", layout)
	}
}

func TestManyToManyEsTier(t *testing.T) {
	project := filepath.Join(t.TempDir(), "EsM2MDemo")
	if err := Run([]string{"new", "EsM2MDemo", "--context", "Blog", "--arch", "es", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Blog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	// es-tier aggregates are event-sourced (no renoir-aggregate shape), but
	// must still carry the // aspgen:navigation marker so the join entity's
	// inverse collections land on both sides.
	for _, agg := range []string{"Post", "Tag"} {
		aggregateFile, err := os.ReadFile(filepath.Join(project, "src", "EsM2MDemo.DomainModel", "Blog", agg+".cs"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(aggregateFile), "// aspgen:navigation") || !strings.Contains(string(aggregateFile), "ICollection<PostTag> PostTags") {
			t.Fatalf("%s.cs missing es-tier inverse navigation: %s", agg, aggregateFile)
		}
	}
}

func TestManyToManyRelationGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "M2MRenoirDemo")
	if err := Run([]string{"new", "M2MRenoirDemo", "--context", "Catalog", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Tag", "name:string", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Post", "title:string", "tags:Tag[]", "--context", "Catalog", "--project", project}); err != nil {
		t.Fatal(err)
	}

	joinAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "PostTag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(joinAggregate), "PostId") || !strings.Contains(string(joinAggregate), "TagId") {
		t.Fatalf("expected both foreign keys in the PostTag join aggregate: %s", joinAggregate)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.Application", "PostTagCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "PostId") || !strings.Contains(string(crudService), "TagId") {
		t.Fatalf("expected both foreign keys in PostTagCrudService.cs: %s", crudService)
	}

	postAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "Post.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(postAggregate), "PostTag") {
		t.Fatalf("expected an inverse PostTag navigation on Post: %s", postAggregate)
	}

	tagAggregate, err := os.ReadFile(filepath.Join(project, "src", "M2MRenoirDemo.DomainModel", "Catalog", "Tag.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tagAggregate), "PostTag") {
		t.Fatalf("expected an inverse PostTag navigation on Tag: %s", tagAggregate)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entities) != 3 {
		t.Fatalf("expected three recorded entities (Tag, Post, PostTag), got %#v", manifest.Entities)
	}
}

func TestEntityRelationshipGenerationRenoir(t *testing.T) {
	project := filepath.Join(t.TempDir(), "SalesDemo")
	if err := Run([]string{"new", "SalesDemo", "--context", "Sales", "--arch", "dm", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Customer", "name:string", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "customer:Customer", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}

	aggregate, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.DomainModel", "Sales", "Order.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(aggregate), "public long CustomerId { get; private set; }") || !strings.Contains(string(aggregate), "public Customer? Customer { get; private set; }") {
		t.Fatalf("expected foreign key and navigation property in Order.cs: %s", aggregate)
	}

	config, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.Persistence", "OrderConfiguration.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "builder.HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);") {
		t.Fatalf("expected relation fluent config in OrderConfiguration.cs: %s", config)
	}

	crudService, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.Application", "OrderCrudService.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(crudService), "CustomerId") {
		t.Fatalf("expected the Customer foreign key to appear in OrderCrudService.cs: %s", crudService)
	}

	customerAggregate, err := os.ReadFile(filepath.Join(project, "src", "SalesDemo.DomainModel", "Sales", "Customer.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customerAggregate), "public ICollection<Order> Orders { get; set; } = [];") {
		t.Fatalf("expected inverse Orders navigation in Customer.cs: %s", customerAggregate)
	}

	if err := Run([]string{"add", "context", "Support", "--arch", "dm", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Ticket", "subject:string", "customer:Customer", "--context", "Support", "--project", project}); err == nil {
		t.Fatal("expected a cross-context relation to be rejected")
	}
	if _, err := os.Stat(filepath.Join(project, "src", "SalesDemo.DomainModel", "Support", "Ticket.cs")); !os.IsNotExist(err) {
		t.Fatalf("rejected cross-context aggregate generation left files behind: %v", err)
	}
}

// TestContextArchWpfUI covers -ui wpf on the --context/--arch engine: both
// attaching it at `new` time (aggregates added afterwards) and attaching it
// later via `add ui` (retrofitting a pre-existing aggregate).
func TestContextArchWpfUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "CqrsWpfDemo")
	if err := Run([]string{"new", "CqrsWpfDemo", "--context", "Billing", "--arch", "cqrs", "-ui", "wpf", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/Desktop/CqrsWpfDemo.Desktop.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/Desktop/Modules/Product/Services/ProductStore.cs")
	assertExists(t, project, "src/Desktop/Modules/Product/Views/ProductView.xaml")

	store, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Product", "Services", "ProductStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(store), `"/api/billing/product"`) {
		t.Fatalf("expected a context-qualified route in ProductStore.cs: %s", store)
	}

	appHost, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "App.xaml.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(appHost), "moduleCatalog.AddModule<CqrsWpfDemo.Desktop.Modules.Product.ProductModule>();") {
		t.Fatalf("expected ProductModule registered in App.xaml.cs: %s", appHost)
	}

	solution, err := os.ReadFile(filepath.Join(project, "CqrsWpfDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\Desktop\\CqrsWpfDemo.Desktop.csproj") {
		t.Fatalf("expected Desktop project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "CqrsWpfDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui wpf solution rewrite: %s", solution)
	}

	// retrofit case: es-tier project without -ui at `new` time, aggregate
	// added first, `add ui --framework wpf` attached afterwards.
	retrofit := filepath.Join(t.TempDir(), "EsWpfDemo")
	if err := Run([]string{"new", "EsWpfDemo", "--context", "Sales", "--arch", "es", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Order", "--framework", "wpf", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/Desktop/Modules/Order/Services/OrderStore.cs")
	retrofitStore, err := os.ReadFile(filepath.Join(retrofit, "src", "Desktop", "Modules", "Order", "Services", "OrderStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retrofitStore), `"/api/sales/order"`) {
		t.Fatalf("expected a context-qualified route in retrofitted OrderStore.cs: %s", retrofitStore)
	}

	// dm tier now supports -ui wpf too, in-process (no WebApi host to call).
	dm := filepath.Join(t.TempDir(), "DmWpfDemo")
	if err := Run([]string{"new", "DmWpfDemo", "--context", "Ops", "--arch", "dm", "-ui", "wpf", "--output", dm}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Widget", "name:string", "--context", "Ops", "--project", dm}); err != nil {
		t.Fatal(err)
	}
	dmStore, err := os.ReadFile(filepath.Join(dm, "src", "Desktop", "Modules", "Widget", "Services", "WidgetStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dmStore), "WidgetCrudService service") {
		t.Fatalf("expected dm-tier WidgetStore.cs to call the CrudService in-process: %s", dmStore)
	}
	if strings.Contains(string(dmStore), "HttpClient") {
		t.Fatalf("dm-tier WidgetStore.cs should not use HttpClient (in-process only): %s", dmStore)
	}
	dmModule, err := os.ReadFile(filepath.Join(dm, "src", "Desktop", "Modules", "Widget", "WidgetModule.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dmModule), "containerRegistry.RegisterSingleton<DmWpfDemoDatabase>") ||
		!strings.Contains(string(dmModule), "containerRegistry.Register<IValidator<WidgetRequest>, WidgetValidator>();") ||
		!strings.Contains(string(dmModule), "containerRegistry.RegisterSingleton<WidgetCrudService>();") {
		t.Fatalf("expected dm-tier WidgetModule.cs to register the DbContext/validator/CrudService in DryIoc: %s", dmModule)
	}

	// retrofit case with a relation: aggregates added first, -ui wpf attached afterwards.
	dmRetrofit := filepath.Join(t.TempDir(), "DmWpfRetroDemo")
	if err := Run([]string{"new", "DmWpfRetroDemo", "--context", "Sales", "--arch", "dm", "--output", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Category", "name:string", "--context", "Sales", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "category:Category", "--context", "Sales", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Product", "--framework", "wpf", "--project", dmRetrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, dmRetrofit, "src/Desktop/Modules/Category/Services/CategoryStore.cs")
	assertExists(t, dmRetrofit, "src/Desktop/Modules/Product/Services/ProductStore.cs")

	// ar tier is still rejected (no dm+ CrudService/aggregate support at all).
	ar := filepath.Join(t.TempDir(), "ArWpfDemo")
	if err := Run([]string{"new", "ArWpfDemo", "--context", "Ops", "--arch", "ar", "-ui", "wpf", "--output", ar}); err == nil {
		t.Fatal("expected -ui wpf to be rejected for an ar-tier context")
	}
}

// TestContextArchWpfUIMixedTiers covers a project whose contexts are at
// DIFFERENT arch tiers (cqrs bootstrapped first via `new`, dm added
// afterwards via `add context`) - a regression test for a bug where every
// aggregate's WPF Store used the SAME project-wide backend (whichever
// context was created first), instead of each aggregate's own context's
// tier, wrongly generating an HTTP Store (and a Desktop.csproj missing the
// Application project reference) for the dm-tier aggregate.
func TestContextArchWpfUIMixedTiers(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MixedTierDemo")
	if err := Run([]string{"new", "MixedTierDemo", "--context", "Sales", "--arch", "cqrs", "--output", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "context", "Inventory", "--arch", "dm", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "StockItem", "quantity:int", "--context", "Inventory", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", project}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "wpf", "--framework", "wpf", "--project", project}); err != nil {
		t.Fatal(err)
	}

	stockItemStore, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "StockItem", "Services", "StockItemStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stockItemStore), "StockItemCrudService service") {
		t.Fatalf("expected the dm-tier StockItem aggregate to get an in-process Store even though Sales (cqrs) was created first: %s", stockItemStore)
	}
	if strings.Contains(string(stockItemStore), "HttpClient http") {
		t.Fatalf("dm-tier StockItemStore.cs should not use HttpClient: %s", stockItemStore)
	}

	orderStore, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "Modules", "Order", "Services", "OrderStore.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(orderStore), "HttpClient http") {
		t.Fatalf("expected the cqrs-tier Order aggregate to keep its HTTP Store: %s", orderStore)
	}

	desktopCsproj, err := os.ReadFile(filepath.Join(project, "src", "Desktop", "MixedTierDemo.Desktop.csproj"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desktopCsproj), `MixedTierDemo.Application\MixedTierDemo.Application.csproj`) {
		t.Fatalf("expected Desktop.csproj to reference the Application project since a dm-tier context exists: %s", desktopCsproj)
	}
}

// TestContextArchBlazorUI covers -ui blazor on the --context/--arch engine,
// mirroring TestContextArchWpfUI: attaching at `new` time (aggregates added
// afterwards) and attaching later via `add ui` (retrofitting a pre-existing
// aggregate).
func TestContextArchBlazorUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "BlazorCqrsDemo")
	if err := Run([]string{"new", "BlazorCqrsDemo", "--context", "Billing", "--arch", "cqrs", "-ui", "blazor", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/BlazorCqrsDemo.AppBlazor.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductCrud.razor")
	assertExists(t, project, "src/BlazorCqrsDemo.AppBlazor/Components/Pages/Billing/ProductDetails.razor")

	page, err := os.ReadFile(filepath.Join(project, "src", "BlazorCqrsDemo.AppBlazor", "Components", "Pages", "Billing", "ProductCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `private const string ApiPath = "/api/billing/product";`) {
		t.Fatalf("expected a context-qualified API path in ProductCrud.razor: %s", page)
	}

	solution, err := os.ReadFile(filepath.Join(project, "BlazorCqrsDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\BlazorCqrsDemo.AppBlazor\\BlazorCqrsDemo.AppBlazor.csproj") {
		t.Fatalf("expected AppBlazor project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "BlazorCqrsDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui blazor solution rewrite: %s", solution)
	}

	// retrofit case: es-tier project without -ui at `new` time, aggregate
	// added first, `add ui --framework blazor` attached afterwards.
	retrofit := filepath.Join(t.TempDir(), "EsBlazorDemo")
	if err := Run([]string{"new", "EsBlazorDemo", "--context", "Sales", "--arch", "es", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Order", "total:decimal", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Order", "--framework", "blazor", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/EsBlazorDemo.AppBlazor/Components/Pages/Sales/OrderCrud.razor")
	retrofitPage, err := os.ReadFile(filepath.Join(retrofit, "src", "EsBlazorDemo.AppBlazor", "Components", "Pages", "Sales", "OrderCrud.razor"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(retrofitPage), `private const string ApiPath = "/api/sales/order";`) {
		t.Fatalf("expected a context-qualified API path in retrofitted OrderCrud.razor: %s", retrofitPage)
	}
}

// TestContextArchMvcUI covers -ui mvc on the --context/--arch engine: dm is
// the only supported tier (headless, in-process CrudService calls instead
// of HTTP), attaching both at `new` time and via `add ui` afterward
// (retrofitting pre-existing aggregates, including one with a relation).
func TestContextArchMvcUI(t *testing.T) {
	project := filepath.Join(t.TempDir(), "MvcDmDemo")
	if err := Run([]string{"new", "MvcDmDemo", "--context", "Billing", "--arch", "dm", "-ui", "mvc", "--output", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/MvcDmDemo.WebMvc/MvcDmDemo.WebMvc.csproj")
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "--context", "Billing", "--project", project}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, project, "src/MvcDmDemo.WebMvc/Controllers/ProductController.cs")
	assertExists(t, project, "src/MvcDmDemo.WebMvc/Views/Product/Index.cshtml")

	controller, err := os.ReadFile(filepath.Join(project, "src", "MvcDmDemo.WebMvc", "Controllers", "ProductController.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(controller), `[Route("billing/product")]`) {
		t.Fatalf("expected a context-qualified route in ProductController.cs: %s", controller)
	}

	program, err := os.ReadFile(filepath.Join(project, "src", "MvcDmDemo.WebMvc", "Program.cs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(program), "builder.Services.AddScoped<ProductCrudService>();") {
		t.Fatalf("expected ProductCrudService registered in Program.cs: %s", program)
	}

	solution, err := os.ReadFile(filepath.Join(project, "MvcDmDemo.sln"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(solution), "src\\MvcDmDemo.WebMvc\\MvcDmDemo.WebMvc.csproj") {
		t.Fatalf("expected WebMvc project in solution: %s", solution)
	}
	if !strings.Contains(string(solution), "MvcDmDemo.UnitTests") {
		t.Fatalf("expected test projects to survive the -ui mvc solution rewrite: %s", solution)
	}

	// retrofit case: dm-tier project without -ui at `new` time, two
	// aggregates (one relation) added first, `add ui --framework mvc`
	// attached afterwards - both must get retrofitted.
	retrofit := filepath.Join(t.TempDir(), "MvcRetroDemo")
	if err := Run([]string{"new", "MvcRetroDemo", "--context", "Sales", "--arch", "dm", "--output", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Category", "name:string", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "aggregate", "Product", "name:string", "price:decimal", "category:Category", "--context", "Sales", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "Product", "--framework", "mvc", "--project", retrofit}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, retrofit, "src/MvcRetroDemo.WebMvc/Controllers/CategoryController.cs")
	assertExists(t, retrofit, "src/MvcRetroDemo.WebMvc/Controllers/ProductController.cs")

	// cqrs/es tiers still reject -ui mvc (no CrudService for es; wpf/blazor
	// cover cqrs already).
	cqrs := filepath.Join(t.TempDir(), "CqrsMvcDemo")
	if err := Run([]string{"new", "CqrsMvcDemo", "--context", "Ops", "--arch", "cqrs", "-ui", "mvc", "--output", cqrs}); err == nil {
		t.Fatal("expected -ui mvc to be rejected for a cqrs-tier context")
	}
}
