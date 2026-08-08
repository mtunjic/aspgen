package generator

import (
	"strings"
	"testing"
)

// Logging is a cross-cutting concern: every generated host bootstraps
// Serilog with configurable levels, and user-facing messages stay in the UI
// while full exception detail only ever goes to the structured log. These
// tests pin the Serilog wiring so it can't silently regress.

func TestRenderWebApiSerilog(t *testing.T) {
	d := m2mData()
	program := renderTemplate(t, "files/cqrs/src/WebApi/Program.cs.tmpl", d)
	for _, expected := range []string{
		"using Serilog;",
		"builder.Host.UseSerilog((context, configuration) => configuration.ReadFrom.Configuration(context.Configuration));",
		"app.UseSerilogRequestLogging();",
		"app.UseExceptionHandler(exceptionHandlerApp =>",
		`LogError(exception, "Unhandled exception processing {Method} {Path}", context.Request.Method, context.Request.Path)`,
		"application/problem+json",
		// user-facing ProblemDetails is generic; full detail goes to the log
		"An unexpected error occurred. Please try again later.",
	} {
		if !strings.Contains(program, expected) {
			t.Errorf("WebApi Program.cs missing %q\n--- rendered ---\n%s", expected, program)
		}
	}

	appsettings := renderTemplate(t, "files/cqrs/src/WebApi/appsettings.json.tmpl", d)
	for _, expected := range []string{`"Serilog"`, `"MinimumLevel"`, `"Microsoft.AspNetCore": "Warning"`, `"Name": "Console"`, `"Name": "File"`, "logs/"} {
		if !strings.Contains(appsettings, expected) {
			t.Errorf("WebApi appsettings.json missing %q\n--- rendered ---\n%s", expected, appsettings)
		}
	}
}

func TestRenderWebApiSerilogEsAndSimple(t *testing.T) {
	d := m2mData()
	for _, path := range []string{
		"files/es/src/WebApi/Program.cs.tmpl",
		"files/simple-webapi/src/WebApi/Program.cs.tmpl",
	} {
		out := renderTemplate(t, path, d)
		if !strings.Contains(out, "builder.Host.UseSerilog(") || !strings.Contains(out, "app.UseExceptionHandler(") {
			t.Errorf("%s missing Serilog bootstrap/exception handler\n--- rendered ---\n%s", path, out)
		}
	}
}

func TestRenderBlazorSerilog(t *testing.T) {
	d := m2mData()
	program := renderTemplate(t, "files/blazor-context/src/{{ .Project }}.AppBlazor/Program.cs.tmpl", d)
	for _, expected := range []string{"using Serilog;", "builder.Host.UseSerilog((context, configuration) => configuration.ReadFrom.Configuration(context.Configuration));"} {
		if !strings.Contains(program, expected) {
			t.Errorf("Blazor Program.cs missing %q\n--- rendered ---\n%s", expected, program)
		}
	}
	appsettings := renderTemplate(t, "files/blazor-context/src/{{ .Project }}.AppBlazor/appsettings.json.tmpl", d)
	if !strings.Contains(appsettings, `"Serilog"`) {
		t.Errorf("Blazor appsettings.json missing Serilog section\n%s", appsettings)
	}
	// CrudPageBase logs failures through ILogger<T> AND surfaces an honest
	// user-facing message (details only ever go to the log).
	base := renderTemplate(t, "files/blazor-context/src/{{ .Project }}.AppBlazor/Components/Shared/CrudPageBase.cs.tmpl", d)
	for _, expected := range []string{
		"[Inject] protected ILogger<CrudPageBase<TItem>> Logger",
		"Logger.LogWarning(\"Delete failed with {Status} for {ApiPath}/{Id}\"",
		"Logger.LogError(ex, \"Failed to load page {Page} of {ApiPath}\"",
		"protected string? errorMessage;",
		"errorMessage = \"Could not delete the record. Please try again.\";",
		"errorMessage = \"Could not load the records. Please try again.\";",
	} {
		if !strings.Contains(base, expected) {
			t.Errorf("CrudPageBase missing %q\n--- rendered ---\n%s", expected, base)
		}
	}
}

func TestRenderMvcSerilog(t *testing.T) {
	d := m2mData()
	program := renderTemplate(t, "files/mvc-context/src/{{ .Project }}.WebMvc/Program.cs.tmpl", d)
	for _, expected := range []string{"using Serilog;", "builder.Host.UseSerilog(", `app.UseExceptionHandler("/Home/Error");`} {
		if !strings.Contains(program, expected) {
			t.Errorf("WebMvc Program.cs missing %q\n--- rendered ---\n%s", expected, program)
		}
	}
	home := renderTemplate(t, "files/mvc-context/src/{{ .Project }}.WebMvc/Controllers/HomeController.cs.tmpl", d)
	if !strings.Contains(home, "public IActionResult Error() => View();") {
		t.Errorf("HomeController missing friendly Error action\n%s", home)
	}
	errorView := renderTemplate(t, "files/mvc-context/src/{{ .Project }}.WebMvc/Views/Home/Error.cshtml.tmpl", d)
	if !strings.Contains(errorView, "Something went wrong") {
		t.Errorf("Error.cshtml missing friendly message\n%s", errorView)
	}
	appsettings := renderTemplate(t, "files/mvc-context/src/{{ .Project }}.WebMvc/appsettings.json.tmpl", d)
	if !strings.Contains(appsettings, `"Serilog"`) {
		t.Errorf("WebMvc appsettings.json missing Serilog section\n%s", appsettings)
	}
}

func TestRenderWpfAppLog(t *testing.T) {
	d := m2mData()
	log := renderTemplate(t, "files/wpf/src/Desktop/Shared/AppLogger.cs.tmpl", d)
	for _, expected := range []string{
		"public static class AppLog",
		"public static void Configure(LogEventLevel minimumLevel = LogEventLevel.Information)",
		// runtime-overridable level via ASPGENT_LOG_LEVEL, no rebuild needed
		`Environment.GetEnvironmentVariable("ASPGENT_LOG_LEVEL")`,
		"Enum.TryParse(envLevel, ignoreCase: true, out LogEventLevel configured)",
		`Log.Information("Logging started at {MinimumLevel}", minimumLevel);`,
		"WriteTo.Debug()",
		"WriteTo.File(",
		"rollingInterval: RollingInterval.Day",
		"public static void Error(Exception? exception, string messageTemplate, params object?[] propertyValues)",
	} {
		if !strings.Contains(log, expected) {
			t.Errorf("AppLogger missing %q\n--- rendered ---\n%s", expected, log)
		}
	}
	app := renderTemplate(t, "files/wpf/src/Desktop/App.xaml.cs.tmpl", d)
	for _, expected := range []string{"AppLog.Configure();", "DispatcherUnhandledException += (_, e) =>", "AppLog.Error(e.Exception, \"Unhandled UI exception\")", "AppLog.CloseAndFlush();"} {
		if !strings.Contains(app, expected) {
			t.Errorf("App.xaml.cs missing %q\n--- rendered ---\n%s", expected, app)
		}
	}
	// Base ViewModels log at the points that used to swallow exceptions.
	edit := renderTemplate(t, "files/wpf/src/Desktop/Shared/EditViewModelBase.cs.tmpl", d)
	if !strings.Contains(edit, "AppLog.Error(ex, \"Save failed for {Entity}\", EntityName)") {
		t.Errorf("EditViewModelBase missing save logging\n%s", edit)
	}
	list := renderTemplate(t, "files/wpf/src/Desktop/Shared/ListViewModelBase.cs.tmpl", d)
	for _, expected := range []string{"AppLog.Error(ex, \"Search failed for {Entity}\", EntityName)", "AppLog.Error(ex, \"Delete failed for {Entity} #{Id}\", EntityName, item.Id)"} {
		if !strings.Contains(list, expected) {
			t.Errorf("ListViewModelBase missing %q\n--- rendered ---\n%s", expected, list)
		}
	}
}
