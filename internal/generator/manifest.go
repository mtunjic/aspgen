package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Manifest struct {
	Project    string    `json:"project"`
	Components []string  `json:"components"`
	Contexts   []Context `json:"contexts,omitempty"`
}

type Context struct {
	Name       string   `json:"name"`
	Aggregates []string `json:"aggregates,omitempty"`
}

func loadManifest(root string) (Manifest, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("project directory %s does not exist; create it first with aspgen new", root)
		}
		return Manifest{}, fmt.Errorf("cannot access project directory %s: %w", root, err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".aspgen", "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("not an aspgen project: missing .aspgen/manifest.json in %s; run from a generated project or pass --project PATH", root)
	}
	var m Manifest
	return m, json.Unmarshal(b, &m)
}

func discoverProjectRoot(start string) (string, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if exists(filepath.Join(root, ".aspgen", "manifest.json")) {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("not an aspgen project: missing .aspgen/manifest.json; run from a generated project or pass --project PATH")
		}
		root = parent
	}
}
func saveManifest(root string, m Manifest) error {
	sort.Strings(m.Components)
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.MkdirAll(filepath.Join(root, ".aspgen"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".aspgen", "manifest.json"), append(b, '\n'), 0o644)
}

// registerGeneratedProjectFiles makes incremental files explicit in their
// owning project without duplicating SDK-style default items.
func registerGeneratedProjectFiles(root string, dryRun bool) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	projects := make(map[string][]string)
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		normalized := filepath.ToSlash(path)
		if entry.IsDir() || strings.Contains(normalized, "/bin/") || strings.Contains(normalized, "/obj/") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".cs" && ext != ".xaml" {
			return nil
		}
		projectFile, err := owningProjectFile(filepath.Dir(path))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(projectFile), path)
		if err != nil {
			return err
		}
		projects[projectFile] = append(projects[projectFile], filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}
	for projectFile, files := range projects {
		sort.Strings(files)
		if err := updateProjectFileItems(projectFile, files, dryRun); err != nil {
			return err
		}
	}
	return nil
}

func componentTheme(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "theme:") {
			return strings.TrimPrefix(component, "theme:")
		}
	}
	return ""
}

func componentBackend(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "backend:") {
			return strings.TrimPrefix(component, "backend:")
		}
	}
	return ""
}

func componentDatabase(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "database:") {
			return strings.TrimPrefix(component, "database:")
		}
	}
	return "sqlite"
}

func componentSeed(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "seed:") {
			value := strings.TrimPrefix(component, "seed:")
			return strings.Split(value, ":")[0]
		}
	}
	return ""
}

func componentSeedCount(components []string) int {
	for _, component := range components {
		if strings.HasPrefix(component, "seed:") {
			parts := strings.Split(component, ":")
			if len(parts) > 2 {
				if count, err := strconv.Atoi(parts[2]); err == nil && count > 0 {
					return count
				}
			}
			return 3
		}
	}
	return 0
}

func rejectAggregateReservedProperties(properties []Property) error {
	for _, property := range properties {
		if property.Name == "Id" {
			return errors.New("aggregate property \"id\" is reserved for the aggregate identity")
		}
	}
	return nil
}

func isRenoir(m Manifest) bool { return hasComponent(m.Components, "renoir") }
func isWebAPI(m Manifest) bool { return hasComponent(m.Components, "webapi") }

// isNonSimpleWebAPI reports whether m is a webapi/fullstack project using a
// non-simple (DDD or default) backend profile.
func isNonSimpleWebAPI(m Manifest) bool {
	return isWebAPI(m) && !hasComponent(m.Components, "backend:simple")
}

// isWPFProject reports whether m has a WPF desktop UI component.
func isWPFProject(m Manifest) bool { return hasComponent(m.Components, "wpf") }

// isLocalDDDWpf reports whether backend selects the local (non-webapi) DDD
// wpf profile for project m.
func isLocalDDDWpf(m Manifest, backend string) bool {
	return backend == "ddd" && isWPFProject(m)
}
func hasComponent(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func contextExists(contexts []Context, name string) bool {
	for _, context := range contexts {
		if context.Name == name {
			return true
		}
	}
	return false
}
func appendContext(contexts []Context, value Context) []Context {
	if contextExists(contexts, value.Name) {
		return contexts
	}
	return append(contexts, value)
}

// appendAggregate records aggregateName under contextName, without duplicating existing entries.
func appendAggregate(contexts []Context, contextName, aggregateName string) []Context {
	for i := range contexts {
		if contexts[i].Name == contextName {
			for _, name := range contexts[i].Aggregates {
				if name == aggregateName {
					return contexts
				}
			}
			contexts[i].Aggregates = append(contexts[i].Aggregates, aggregateName)
		}
	}
	return contexts
}
