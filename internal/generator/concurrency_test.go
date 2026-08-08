package generator

import (
	"strings"
	"testing"
)

// Optimistic concurrency: every entity carries a Version bumped on each write;
// an update sends back the version the form loaded and is rejected with a
// conflict when the row moved on (409 over HTTP, a ConcurrencyConflictException
// in the WPF store, a model error in MVC). These tests pin the wiring.

func TestRenderCrudServiceOptimisticConcurrency(t *testing.T) {
	d := m2mData()
	out := renderTemplate(t, "files/dm-crud/src/{{ .Project }}.Application/{{ .Aggregate }}CrudService.cs.tmpl", d)
	for _, expected := range []string{
		"if (entity.Version != request.Version) return CommandResponse.ConcurrencyConflict();",
		", int Version);",                // View record carries the version
		"int Version = 0);",              // Request carries the expected version (default 0)
		"x.Version)).ToListAsync(cancellationToken);",
		", x.Version)).ToArrayAsync(cancellationToken);",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("CrudService OC missing %q\n--- rendered ---\n%s", expected, out)
		}
	}
}

func TestRenderCommandResponseConflict(t *testing.T) {
	for _, path := range []string{
		"files/dm/src/{{ .Project }}.DomainModel/CommandResponse.cs.tmpl",
		"files/cqrs/src/{{ .Project }}.DomainModel/CommandResponse.cs.tmpl",
	} {
		out := renderTemplate(t, path, m2mData())
		for _, expected := range []string{
			`public const string ConcurrencyConflictMessage = "This record was modified by someone else. Please reload and try again.";`,
			"public static CommandResponse ConcurrencyConflict() => Fail().AddMessage(ConcurrencyConflictMessage);",
		} {
			if !strings.Contains(out, expected) {
				t.Errorf("%s missing %q\n--- rendered ---\n%s", path, expected, out)
			}
		}
	}
}

func TestRenderBaseEntityVersion(t *testing.T) {
	for _, path := range []string{
		"files/dm/src/{{ .Project }}.DomainModel/BaseEntity.cs.tmpl",
		"files/cqrs/src/{{ .Project }}.DomainModel/BaseEntity.cs.tmpl",
	} {
		out := renderTemplate(t, path, m2mData())
		if !strings.Contains(out, "public int Version { get; private set; }") {
			t.Errorf("%s missing Version\n%s", path, out)
		}
	}
	methods := renderTemplate(t, "files/dm/src/{{ .Project }}.DomainModel/BaseEntity.Methods.cs.tmpl", m2mData())
	if !strings.Contains(methods, "Version++;") {
		t.Errorf("BaseEntity.Methods must bump Version on Mark\n%s", methods)
	}
}

func TestRenderCqrsUpdateCarriesVersion(t *testing.T) {
	d := m2mData()
	command := renderTemplate(t, "files/cqrs-feature/src/{{ .Project }}.Application/Features/{{ .Context }}/{{ .Aggregate }}/Update{{ .Aggregate }}Command.cs.tmpl", d)
	if !strings.Contains(command, ", int Version);") {
		t.Errorf("UpdateCommand missing Version\n%s", command)
	}
	handler := renderTemplate(t, "files/cqrs-feature/src/{{ .Project }}.Application/Features/{{ .Context }}/{{ .Aggregate }}/Update{{ .Aggregate }}Handler.cs.tmpl", d)
	for _, expected := range []string{
		"request.Version), cancellationToken);",
		"new Error(\"conflict\", result.Message)",
	} {
		if !strings.Contains(handler, expected) {
			t.Errorf("UpdateHandler missing %q\n%s", expected, handler)
		}
	}
	endpoints := renderTemplate(t, "files/cqrs-feature/src/WebApi/Features/{{ .Context }}/{{ .Aggregate }}/{{ .Aggregate }}Endpoints.cs.tmpl", d)
	for _, expected := range []string{
		"request.Version);", // UpdateCommand gets the request's version
		`Results.Conflict(result.Error.Message)`,
	} {
		if !strings.Contains(endpoints, expected) {
			t.Errorf("Endpoints missing %q\n%s", expected, endpoints)
		}
	}
}

func TestRenderWpfOptimisticConcurrency(t *testing.T) {
	d := m2mData()
	row := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Models/{{ .Name }}Row.cs.tmpl", d)
	if !strings.Contains(row, ", int Version)") {
		t.Errorf("Row missing Version\n%s", row)
	}
	entityRow := renderTemplate(t, "files/wpf/src/Desktop/Shared/IEntityRow.cs.tmpl", d)
	if !strings.Contains(entityRow, "int Version { get; }") {
		t.Errorf("IEntityRow missing Version\n%s", entityRow)
	}
	base := renderTemplate(t, "files/wpf/src/Desktop/Shared/EditViewModelBase.cs.tmpl", d)
	for _, expected := range []string{
		"protected int EditingVersion;",
		"EditingVersion = item?.Version ?? 0;",
		"catch (ConcurrencyConflictException ex)",
	} {
		if !strings.Contains(base, expected) {
			t.Errorf("EditViewModelBase missing %q\n%s", expected, base)
		}
	}
	store := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Services/{{ .Name }}Store.cs.tmpl", d)
	for _, expected := range []string{
		"value.Version);", // Request built with the row's version
		"created.Version);",
		`throw new global::Demo.Desktop.Shared.ConcurrencyConflictException(result.Message ?? "Save failed.");`,
	} {
		if !strings.Contains(store, expected) {
			t.Errorf("dm Store missing %q\n%s", expected, store)
		}
	}
	// HTTP-backed store surfaces the WebApi's 409 as a conflict exception.
	http := m2mData()
	http.Backend = "cqrs"
	httpStore := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/Services/{{ .Name }}Store.cs.tmpl", http)
	if !strings.Contains(httpStore, "if ((int)update.StatusCode == 409)") {
		t.Errorf("HTTP Store missing the 409 conflict handling\n%s", httpStore)
	}
	vm := renderTemplate(t, "files/wpf-entity/src/Desktop/Modules/{{ .Name }}/ViewModels/{{ .Name }}EditViewModel.cs.tmpl", d)
	if !strings.Contains(vm, ", EditingVersion);") {
		t.Errorf("EditViewModel TryBuild missing EditingVersion\n%s", vm)
	}
}
