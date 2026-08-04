package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedLayoutIntegration exercises the public generation workflow and
// checks that each profile puts files in the project that owns them. This is
// intentionally filesystem-based rather than template-based: a template can
// render successfully while still targeting the wrong layer or project.
func TestGeneratedLayoutIntegration(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		entity       []string
		want         []string
		forbid       []string
		inSolution   []string
		projectRoots []string
	}{
		{
			name:         "clean webapi",
			args:         []string{"new", "CleanApi", "--app", "webapi"},
			entity:       []string{"Person", "name:string", "age:int", "born:date", "active:bool"},
			want:         []string{"src/Domain/Entities/Person.cs"},
			forbid:       []string{"src/WebApi/Models/Person.cs", "src/WebApi/Data/AppDbContext.cs", "src/Infrastructure/Persistence/PersonRepository.cs"},
			inSolution:   []string{"src\\Domain\\Domain.csproj", "src\\Application\\Application.csproj", "src\\Infrastructure\\Infrastructure.csproj", "src\\WebApi\\WebApi.csproj"},
			projectRoots: []string{"src/Domain", "src/Application", "src/Infrastructure", "src/WebApi"},
		},
		{
			name:         "ddd fullstack",
			args:         []string{"new", "DddApp", "--app", "fullstack", "--backend:ddd"},
			entity:       []string{"Person", "name:string", "age:int"},
			want:         []string{"src/Domain/Entities/Person.cs", "src/Infrastructure/Persistence/PersonRepository.cs", "src/Application/Features/Person/CreatePersonCommand.cs", "src/WebApi/Features/Person/PersonEndpoints.cs", "src/Desktop/Modules/Person/PersonModule.cs"},
			forbid:       []string{"src/WebApi/Models/Person.cs", "src/WebApi/Data/AppDbContext.cs"},
			inSolution:   []string{"src\\Domain\\Domain.csproj", "src\\Application\\Application.csproj", "src\\Infrastructure\\Infrastructure.csproj", "src\\WebApi\\WebApi.csproj", "src\\Desktop\\DddApp.Desktop.csproj"},
			projectRoots: []string{"src/Domain", "src/Application", "src/Infrastructure", "src/WebApi", "src/Desktop"},
		},
		{
			name:   "simple fullstack",
			args:   []string{"new", "SimpleApp", "--app", "fullstack", "--simple", "--theme:wpfui"},
			entity: []string{"Person", "name:string", "age:int", "born:date", "active:bool"},
			want: []string{
				"src/WebApi/Models/Person.cs",
				"src/WebApi/Data/AppDbContext.cs",
				"src/WebApi/Features/Person/PersonEndpoints.cs",
				"src/Desktop/Modules/Person/PersonModule.cs",
				"src/Desktop/Modules/Person/Views/PersonView.xaml",
			},
			forbid: []string{
				"src/Domain/Entities/Person.cs",
				"src/Application/Features/Person/CreatePersonCommand.cs",
				"src/Infrastructure/Persistence/PersonRepository.cs",
			},
			inSolution:   []string{"src\\WebApi\\WebApi.csproj", "src\\Desktop\\SimpleApp.Desktop.csproj"},
			projectRoots: []string{"src/WebApi", "src/Desktop"},
		},
		{
			name:         "wpf only",
			args:         []string{"new", "DesktopApp", "--app", "wpf"},
			entity:       []string{"Person", "name:string", "active:bool"},
			want:         []string{"src/Desktop/Modules/Person/PersonModule.cs", "src/Desktop/Modules/Person/Services/PersonStore.cs"},
			forbid:       []string{"src/WebApi/Features/Person/PersonEndpoints.cs", "src/Domain/Entities/Person.cs"},
			inSolution:   []string{"src\\Desktop\\DesktopApp.Desktop.csproj"},
			projectRoots: []string{"src/Desktop"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), filepath.Base(test.args[1]))
			args := append(append([]string{}, test.args...), "--output", project)
			if err := Run(args); err != nil {
				t.Fatal(err)
			}
			if err := Run(append(append([]string{"add", "entity"}, test.entity...), "--project", project)); err != nil {
				t.Fatal(err)
			}

			for _, relative := range test.want {
				assertExists(t, project, relative)
			}
			for _, relative := range test.forbid {
				if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(relative))); err == nil {
					t.Errorf("file is in the wrong generated location: %s", relative)
				}
			}

			manifestBytes, err := os.ReadFile(filepath.Join(project, ".aspgen", "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest Manifest
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				t.Fatal(err)
			}
			if !contains(manifest.Components, "entity:"+test.entity[0]) {
				t.Fatalf("manifest does not record generated entity: %#v", manifest.Components)
			}

			solution, err := os.ReadFile(filepath.Join(project, manifest.Project+".sln"))
			if err != nil {
				t.Fatal(err)
			}
			for _, projectPath := range test.inSolution {
				if !strings.Contains(string(solution), projectPath) {
					t.Errorf("solution is missing project %s", projectPath)
				}
			}
			assertNoOrphanSourceFiles(t, project, test.projectRoots)
		})
	}
}

func assertNoOrphanSourceFiles(t *testing.T, root string, projectRoots []string) {
	t.Helper()
	known := make(map[string]bool, len(projectRoots))
	for _, projectRoot := range projectRoots {
		known[filepath.Clean(filepath.FromSlash(projectRoot))] = true
	}
	err := filepath.Walk(filepath.Join(root, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".cs" && extension != ".xaml" && extension != ".csproj" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) < 2 || !known[filepath.Clean(filepath.FromSlash(parts[0]+"/"+parts[1]))] {
			t.Errorf("source file is outside every generated project: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertExists(t *testing.T, root, relative string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
		t.Errorf("expected generated file %s: %v", relative, err)
	}
}

func TestIncrementalPlacementGuards(t *testing.T) {
	api := filepath.Join(t.TempDir(), "Api")
	if err := Run([]string{"new", "Api", "--app", "webapi", "--output", api}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "ui", "--theme:wpfui", "--project", api}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "module", "Reports", "--project", api}); err != nil {
		t.Fatal(err)
	}
	assertExists(t, api, "src/Desktop/Api.Desktop.csproj")
	assertExists(t, api, "src/Desktop/Modules/Reports/ReportsModule.cs")
	solution, err := os.ReadFile(filepath.Join(api, "Api.sln"))
	if err != nil || !strings.Contains(string(solution), `src\Desktop\Api.Desktop.csproj`) {
		t.Fatalf("incremental UI was not added to the solution: %v", err)
	}

	simple := filepath.Join(t.TempDir(), "Simple")
	if err := Run([]string{"new", "Simple", "--app", "webapi", "--simple", "--output", simple}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "database", "postgres", "--project", simple}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(simple, "src", "Infrastructure")); !os.IsNotExist(err) {
		t.Fatalf("simple database addition created an unrelated Infrastructure directory: %v", err)
	}

	plain := filepath.Join(t.TempDir(), "Plain")
	if err := Run([]string{"new", "Plain", "--app", "webapi", "--output", plain}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "module", "Reports", "--project", plain}); err == nil {
		t.Fatal("expected module generation to reject a project without WPF")
	}
	if _, err := os.Stat(filepath.Join(plain, "src", "Desktop")); !os.IsNotExist(err) {
		t.Fatalf("rejected module generation left Desktop files behind: %v", err)
	}
	desktopOnly := filepath.Join(t.TempDir(), "DesktopOnly")
	if err := Run([]string{"new", "DesktopOnly", "--app", "wpf", "--output", desktopOnly}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"add", "database", "postgres", "--project", desktopOnly}); err == nil {
		t.Fatal("expected database generation to reject a project without Web API")
	}
	if _, err := os.Stat(filepath.Join(desktopOnly, "src", "Infrastructure")); !os.IsNotExist(err) {
		t.Fatalf("rejected database generation left Infrastructure files behind: %v", err)
	}

	if err := Run([]string{"add", "feature", "CreatePerson", "name:string", "--project", simple}); err == nil {
		t.Fatal("expected feature generation to reject a simple project")
	}
	if err := Run([]string{"add", "service", "Email", "--project", simple}); err == nil {
		t.Fatal("expected service generation to reject a simple project")
	}
	if err := Run([]string{"add", "entity", "Order", "total:decimal", "--backend:ddd", "--project", simple}); err == nil {
		t.Fatal("expected conflicting backend generation to be rejected")
	}
	if _, err := os.Stat(filepath.Join(simple, "src", "Application")); !os.IsNotExist(err) {
		t.Fatalf("rejected simple operations left Application files behind: %v", err)
	}
}
