package generator

import (
	"strings"
	"testing"
)

// Column sorting threads sortBy/sortDesc through the whole search pipeline:
// EF/search service -> cqrs/es handlers + endpoints -> each frontend's store ->
// a sort selector in the list UI. These tests pin that wiring.

func TestRenderCrudServiceSorting(t *testing.T) {
	d := m2mData()
	out := renderTemplate(t, "files/dm-crud/src/{{ .Project }}.Application/{{ .Aggregate }}CrudService.cs.tmpl", d)
	for _, expected := range []string{
		"string? sortBy = null, bool sortDesc = false",
		`"Title" => sortDesc ? query.OrderByDescending(x => x.Title) : query.OrderBy(x => x.Title),`,
		`"Body" => sortDesc ? query.OrderByDescending(x => x.Body) : query.OrderBy(x => x.Body),`,
		"_ => query.OrderBy(x => x.Id),",
		"query.Skip((page - 1) * pageSize)",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("CrudService sort missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
	if strings.Contains(out, "query.OrderBy(x => x.Id).Skip") {
		t.Errorf("CrudService must not hard-code the Id ordering\n%s", out)
	}
}

func TestRenderEsHandlerSorting(t *testing.T) {
	out := renderTemplate(t, "files/es-feature/src/{{ .Project }}.Application/Features/{{ .Context }}/{{ .Aggregate }}/Search{{ .Aggregate }}sHandler.cs.tmpl", m2mData())
	if !strings.Contains(out, `"Title" => request.SortDesc ? query.OrderByDescending(x => x.Title) : query.OrderBy(x => x.Title),`) {
		t.Errorf("es handler missing sort switch\n%s", out)
	}
}

func TestRenderCqrsSearchCarriesSort(t *testing.T) {
	d := m2mData()
	query := renderTemplate(t, "files/cqrs-feature/src/{{ .Project }}.Application/Features/{{ .Context }}/{{ .Aggregate }}/Search{{ .Aggregate }}sQuery.cs.tmpl", d)
	if !strings.Contains(query, "string? SortBy = null, bool SortDesc = false") {
		t.Errorf("SearchQuery missing sort params\n%s", query)
	}
	handler := renderTemplate(t, "files/cqrs-feature/src/{{ .Project }}.Application/Features/{{ .Context }}/{{ .Aggregate }}/Search{{ .Aggregate }}sHandler.cs.tmpl", d)
	if !strings.Contains(handler, "request.Page, request.PageSize, cancellationToken, request.SortBy, request.SortDesc") {
		t.Errorf("SearchHandler missing sort forwarding\n%s", handler)
	}
	endpoints := renderTemplate(t, "files/cqrs-feature/src/WebApi/Features/{{ .Context }}/{{ .Aggregate }}/{{ .Aggregate }}Endpoints.cs.tmpl", d)
	for _, expected := range []string{"string? sortBy = null, bool sortDesc = false", "page, pageSize, sortBy, sortDesc);"} {
		if !strings.Contains(endpoints, expected) {
			t.Errorf("Endpoints missing %q\n%s", expected, endpoints)
		}
	}
}

func TestRenderWpfSortingUi(t *testing.T) {
	d := m2mData()
	d.Theme = "wpfui"
	listPage := renderTemplate(t, "files/wpf/src/Desktop/Shared/Controls/ListPage.xaml.tmpl", d)
	for _, expected := range []string{
		`ItemsSource="{Binding SortFields}"`,
		`SelectedValue="{Binding SortBy, Mode=TwoWay}"`,
		`IsChecked="{Binding SortDesc, Mode=TwoWay}"`,
	} {
		if !strings.Contains(listPage, expected) {
			t.Errorf("ListPage sort UI missing %q\n%s", expected, listPage)
		}
	}
	base := renderTemplate(t, "files/wpf/src/Desktop/Shared/ListViewModelBase.cs.tmpl", d)
	for _, expected := range []string{"public string? SortBy", "public bool SortDesc"} {
		if !strings.Contains(base, expected) {
			t.Errorf("ListViewModelBase missing %q\n%s", expected, base)
		}
	}
	// the module view model exposes the sortable columns and passes sort into
	// the criteria
	vm := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}ViewModel.cs.tmpl", d)
	for _, expected := range []string{
		"public IReadOnlyList<SortField> SortFields { get; }",
		`new("Title", "Title"),`,
		", page, PageSize, SortBy, SortDesc);",
	} {
		if !strings.Contains(vm, expected) {
			t.Errorf("WPF module view model missing %q\n%s", expected, vm)
		}
	}
}

func TestRenderBlazorSortingUi(t *testing.T) {
	d := m2mData()
	bar := renderTemplate(t, "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/SearchFilterBar.razor.tmpl", d)
	for _, expected := range []string{
		"SortFields.Count > 0",
		"SortByChanged.InvokeAsync(e.Value?.ToString())",
		"SortDescChanged.InvokeAsync",
	} {
		if !strings.Contains(bar, expected) {
			t.Errorf("SearchFilterBar sort UI missing %q\n%s", expected, bar)
		}
	}
	page := renderTemplate(t, "files/blazor-context-crud/src/{{ .Project }}.AppBlazor/Components/Pages/{{ .Context }}/{{ .Aggregate }}Crud.razor.tmpl", d)
	for _, expected := range []string{
		"private List<SortOption> sortFields = [];",
		"BuildSortFields();",
		"OnSortByChanged(v)",
		`if (!string.IsNullOrWhiteSpace(sortBy)) query.Add($"sortBy={sortBy}");`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("Blazor Crud page sort missing %q\n%s", expected, page)
		}
	}
}

func TestRenderMvcSortingUi(t *testing.T) {
	d := m2mData()
	bar := renderTemplate(t, "files/mvc-context/src/{{ .Project }}.WebMvc/Views/Shared/_FilterBar.cshtml.tmpl", d)
	for _, expected := range []string{
		"Model.SortableFields.Count > 0",
		`name="sortBy"`,
		`name="sortDesc"`,
	} {
		if !strings.Contains(bar, expected) {
			t.Errorf("_FilterBar sort UI missing %q\n%s", expected, bar)
		}
	}
	controller := renderTemplate(t, "files/mvc-context-crud/src/{{ .Project }}.WebMvc/Controllers/{{ .Aggregate }}Controller.cs.tmpl", d)
	for _, expected := range []string{
		"string? sortBy = null, bool sortDesc = false",
		"PageSize, cancellationToken, sortBy, sortDesc)",
		"sortBy, sortDesc);",
	} {
		if !strings.Contains(controller, expected) {
			t.Errorf("MVC controller sort missing %q\n%s", expected, controller)
		}
	}
}
