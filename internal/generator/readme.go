package generator

import (
	"path/filepath"
	"strings"
)

// updateReadmeRunSection appends a "Run with the <UI>" section to the
// project-root README.md once a UI host is attached onto a --context/--arch
// project. The tier README is rendered at `new` time (before any UI is
// attached), so without this the root README only documents `dotnet run
// --project src\WebApi` — correct for the API but not for a fullstack app
// that needs the UI host started too. The append is idempotent (guarded by
// the section header).
func updateReadmeRunSection(project, projectName, ui, backend string, dryRun bool) error {
	section, header := readmeRunSection(projectName, ui, backend)
	if section == "" {
		return nil
	}
	path := filepath.Join(project, "README.md")
	content, err := readMarkerFile(path, "project README")
	if err != nil {
		return err
	}
	if strings.Contains(content, header) {
		return nil
	}
	content = normalizeCRLF(content)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += section
	return writeMarkerFile(path, content, dryRun)
}

// readmeRunSection builds the README section text for ui (empty when the UI
// needs no run documentation of its own).
func readmeRunSection(projectName, ui, backend string) (section, header string) {
	switch ui {
	case "blazor":
		return "\n## Run with the Blazor UI\n\n" +
			"The AppBlazor host is a separate process that talks to the WebApi over " +
			"HttpClient. Start the WebApi first (it listens on http://localhost:5000), " +
			"then the UI (http://localhost:5001):\n\n" +
			"```powershell\n" +
			"dotnet run --project src\\WebApi\n" +
			"dotnet run --project src\\" + projectName + ".AppBlazor\n" +
			"```\n\n" +
			"The API address the UI calls defaults to `http://localhost:5000` " +
			"(`ASPGENT_API_URL` to override); the UI's own listen port defaults to " +
			"`http://localhost:5001` (`ASPGENT_BLAZOR_URL` to override).\n", "## Run with the Blazor UI"
	case "mvc":
		return "\n## Run with the MVC UI\n\n" +
			"The WebMvc host calls the aggregates' CrudServices in-process (the dm tier " +
			"has no WebApi host):\n\n" +
			"```powershell\n" +
			"dotnet run --project src\\" + projectName + ".WebMvc\n" +
			"```\n", "## Run with the MVC UI"
	case "wpf":
		return "\n## Run with the WPF desktop app\n\n" +
			"The Desktop project lives under `src\\Desktop`; see `src\\Desktop\\README.md` " +
			"for full build/run instructions, including the two-process run when the " +
			"desktop app talks to the WebApi over HTTP (cqrs/es tiers).\n", "## Run with the WPF desktop app"
	}
	return "", ""
}
