package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func owningProjectFile(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".csproj") {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no owning .csproj found for generated file below %s", start)
		}
		dir = parent
	}
}

func updateProjectFileItems(projectFile string, files []string, dryRun bool) error {
	content, err := os.ReadFile(projectFile)
	if err != nil {
		return err
	}
	textContent := string(content)
	const marker = "  <!-- aspgen:files -->"
	if !strings.Contains(textContent, marker) {
		textContent = strings.Replace(textContent, "</Project>", "  <ItemGroup>\n"+marker+"\n  </ItemGroup>\n</Project>", 1)
	}
	var additions []string
	for _, file := range files {
		item := "Compile"
		if strings.HasSuffix(strings.ToLower(file), ".xaml") {
			item = "Page"
			if strings.EqualFold(filepath.Base(file), "App.xaml") {
				item = "ApplicationDefinition"
			}
		}
		line := fmt.Sprintf("    <%s Update=\"%s\" />", item, file)
		if !strings.Contains(textContent, line) {
			additions = append(additions, line)
		}
	}
	if len(additions) == 0 {
		return nil
	}
	textContent = strings.Replace(textContent, marker, marker+"\n"+strings.Join(additions, "\n"), 1)
	if dryRun {
		fmt.Println("would update", projectFile)
		return nil
	}
	return os.WriteFile(projectFile, []byte(textContent), 0o644)
}

func projectFileName(project, suffix string) string {
	if strings.Contains(project, ".") {
		return project + "." + suffix + ".csproj"
	}
	return suffix + ".csproj"
}

func normalizeProjectFiles(root, project string) error {
	if !strings.Contains(project, ".") {
		return nil
	}
	for _, suffix := range []string{"Domain", "Application", "Infrastructure", "WebApi"} {
		oldPath := filepath.Join(root, "src", suffix, suffix+".csproj")
		newPath := filepath.Join(root, "src", suffix, projectFileName(project, suffix))
		if exists(oldPath) && oldPath != newPath {
			if exists(newPath) {
				return fmt.Errorf("cannot rename %s because %s already exists", oldPath, newPath)
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}
	return rewriteProjectReferences(root, project)
}

func rewriteProjectReferences(root, project string) error {
	replacements := map[string]string{}
	for _, suffix := range []string{"Domain", "Application", "Infrastructure", "WebApi"} {
		replacements[suffix+".csproj"] = projectFileName(project, suffix)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (filepath.Ext(path) != ".csproj" && filepath.Ext(path) != ".sln") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		textContent := string(content)
		for oldName, newName := range replacements {
			textContent = strings.ReplaceAll(textContent, oldName, newName)
		}
		return os.WriteFile(path, []byte(textContent), 0o644)
	})
}
