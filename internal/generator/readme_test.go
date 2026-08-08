package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadmeRunSection(t *testing.T) {
	tests := []struct {
		ui, backend, project string
		wantIn               []string
	}{
		{
			ui: "blazor", backend: "cqrs", project: "C",
			wantIn: []string{"## Run with the Blazor UI", "dotnet run --project src\\WebApi", "dotnet run --project src\\C.AppBlazor", "ASPGENT_API_URL", "ASPGENT_BLAZOR_URL", "http://localhost:5001"},
		},
		{
			ui: "mvc", backend: "dm", project: "D",
			wantIn: []string{"## Run with the MVC UI", "dotnet run --project src\\D.WebMvc"},
		},
		{
			ui: "wpf", backend: "cqrs", project: "B",
			wantIn: []string{"## Run with the WPF desktop app", "src\\Desktop\\README.md"},
		},
	}
	for _, tt := range tests {
		section, header := readmeRunSection(tt.project, tt.ui, tt.backend)
		if header == "" || !strings.Contains(section, header) {
			t.Errorf("readmeRunSection(%q) header mismatch: section = %q, header = %q", tt.ui, section, header)
		}
		for _, want := range tt.wantIn {
			if !strings.Contains(section, want) {
				t.Errorf("readmeRunSection(%q) missing %q; got:\n%s", tt.ui, want, section)
			}
		}
	}
	if section, _ := readmeRunSection("X", "spa", "cqrs"); section != "" {
		t.Errorf("spa should need no README section, got %q", section)
	}
}

func TestUpdateReadmeRunSection(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Demo\n\n## Build\n\nrestore/build steps.\n\n## Run\n\ndotnet run --project src\\WebApi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateReadmeRunSection(dir, "Demo", "blazor", "cqrs", false); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "## Run with the Blazor UI") {
		t.Fatalf("README missing Blazor run section; got:\n%s", content)
	}
	// Idempotent: a second call must not duplicate the section.
	if err := updateReadmeRunSection(dir, "Demo", "blazor", "cqrs", false); err != nil {
		t.Fatal(err)
	}
	content2, _ := os.ReadFile(readme)
	if got := strings.Count(string(content2), "## Run with the Blazor UI"); got != 1 {
		t.Fatalf("README Blazor section duplicated (count=%d); got:\n%s", got, content2)
	}
}
