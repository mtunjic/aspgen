package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest struct {
	Project     string       `json:"project"`
	Components  []string     `json:"components"`
	UI          string       `json:"ui,omitempty"`
	Persistence string       `json:"persistence,omitempty"`
	Contexts    []Context    `json:"contexts,omitempty"`
	Entities    []EntityMeta `json:"entities,omitempty"`
}

type Context struct {
	Name string `json:"name"`
	// Arch is the bounded context's --arch tier: ar, dm, cqrs, or es. Empty
	// only for a context declared via `add context NAME` without --arch yet
	// (a two-step workflow; --arch can be backfilled with a later `add
	// context NAME --arch TIER` call).
	Arch       string   `json:"arch,omitempty"`
	Aggregates []string `json:"aggregates,omitempty"`
	Entities   []string `json:"entities,omitempty"`
}

// archTiers is the ordinal ladder from least to most capable, each tier a
// superset of the previous tier's concepts (ar < dm < cqrs < es).
var archTiers = []string{"ar", "dm", "cqrs", "es"}

func archIndex(arch string) int {
	for i, tier := range archTiers {
		if tier == arch {
			return i
		}
	}
	return -1
}

// archAtLeast reports whether arch is at least as capable as min on the
// archTiers ladder; false if either value isn't a recognized tier.
func archAtLeast(arch, min string) bool {
	a, m := archIndex(arch), archIndex(min)
	return a >= 0 && m >= 0 && a >= m
}

// archBackend maps a --context/--arch engine tier to the `data.Backend`
// value its own template family expects (used to pick the right Store
// branch for UI generation): "ar" reuses the legacy "simple" identifier
// since ar-tier entities render through the same HTTP-CRUD Store branch;
// dm/cqrs/es already match their own backend component naming.
func archBackend(arch string) string {
	if arch == "ar" {
		return "simple"
	}
	return arch
}

// projectHasDmContext reports whether any bounded context recorded in m is
// at the dm arch tier (used to decide whether a shared shell project, e.g.
// Desktop, needs a project reference it wouldn't otherwise get from
// componentBackend, which only reflects the FIRST context created via `new`).
func projectHasDmContext(m Manifest) bool {
	for _, ctx := range m.Contexts {
		if ctx.Arch == "dm" {
			return true
		}
	}
	return false
}

// isContextEngine reports whether m was created via the --context/--arch
// engine (true for every project; the generator has no other bootstrap path).
func isContextEngine(m Manifest) bool { return hasComponent(m.Components, "context-engine") }

// findContext returns the context named name, if any.
func findContext(contexts []Context, name string) (Context, bool) {
	for _, context := range contexts {
		if context.Name == name {
			return context, true
		}
	}
	return Context{}, false
}

// appendContextWithArch records a context created via the --context/--arch
// engine, or backfills Arch onto an existing arch-less (legacy) entry.
func appendContextWithArch(contexts []Context, name, arch string) []Context {
	for i := range contexts {
		if contexts[i].Name == name {
			if arch != "" && contexts[i].Arch == "" {
				contexts[i].Arch = arch
			}
			return contexts
		}
	}
	return append(contexts, Context{Name: name, Arch: arch})
}

// appendContextEntity records entityName under contextName, without
// duplicating existing entries.
func appendContextEntity(contexts []Context, contextName, entityName string) []Context {
	for i := range contexts {
		if contexts[i].Name == contextName {
			for _, name := range contexts[i].Entities {
				if name == entityName {
					return contexts
				}
			}
			contexts[i].Entities = append(contexts[i].Entities, entityName)
		}
	}
	return contexts
}

// EntityMeta records enough about a previously-added entity/aggregate to let
// a later `add entity`/`add aggregate` declare a relation to it (validate it
// exists and pick a display property for picker controls).
type EntityMeta struct {
	Name       string     `json:"name"`
	Context    string     `json:"context,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	// ManyToMany records the `nav:Entity[]` relations declared on this
	// entity, so a later `add ui` (retrofitting a UI onto aggregates that
	// already existed) can render the multi-select pickers without having
	// the original add-call args on hand.
	ManyToMany []ManyToManyRelation `json:"manyToMany,omitempty"`
}

// appendEntityMeta records name/context/properties in entities, replacing
// any existing entry for the same name so re-running `add entity` (--force)
// keeps the recorded properties current.
func appendEntityMeta(entities []EntityMeta, meta EntityMeta) []EntityMeta {
	for i := range entities {
		if entities[i].Name == meta.Name {
			entities[i] = meta
			return entities
		}
	}
	return append(entities, meta)
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

func componentThemeMode(components []string) string {
	for _, component := range components {
		if strings.HasPrefix(component, "theme-mode:") {
			return strings.TrimPrefix(component, "theme-mode:")
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

// rejectAggregateReservedProperties rejects property names that would clash
// with a base class member. "Id" is reserved for every tier; es-tier
// aggregates additionally reserve "Version"/"Deleted", which are members of
// EventSourcedAggregate.
func rejectAggregateReservedProperties(properties []Property, arch string) error {
	for _, property := range properties {
		if property.Name == "Id" {
			return errors.New("aggregate property \"id\" is reserved for the aggregate identity")
		}
		if arch == "es" && (property.Name == "Version" || property.Name == "Deleted") {
			return fmt.Errorf("aggregate property %q is reserved by EventSourcedAggregate on es-tier contexts", property.Name)
		}
	}
	return nil
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
