package generator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"aspgen/internal/templates"
)

type data struct {
	Project    string
	Namespace  string
	Name       string
	Properties []Property
	// Relations holds many-to-one references declared on this entity;
	// the corresponding foreign key is also present in Properties.
	Relations []Relation
	Context   string
	Aggregate string
	Crud      bool
	Theme     string
	Backend   string
	Database  string
	Seed      string
	SeedCount int
}

func renderTree(root, group string, d data, override string, dryRun, force bool) error {
	var source fs.FS = templates.FS
	prefix := filepath.ToSlash(filepath.Join("files", group))
	if override != "" {
		candidate := filepath.Join(override, group)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			source = os.DirFS(override)
			prefix = filepath.ToSlash(group)
		}
	}
	return fs.WalkDir(source, prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(path, prefix+"/"), ".tmpl")
		rel, err := renderString(filepath.ToSlash(rel), d)
		if err != nil {
			return fmt.Errorf("render path %s: %w", path, err)
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if exists(target) && !force {
			if group == "entity" && filepath.Base(target) == "AuditableEntity.cs" {
				return nil
			}
			return fmt.Errorf("refusing to overwrite %s", target)
		}
		if dryRun {
			fmt.Println("would create", target)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		content, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		body, err := renderString(string(content), d)
		if err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		return os.WriteFile(target, []byte(body), 0o644)
	})
}

func renderString(raw string, d data) (string, error) {
	t, err := template.New("file").Funcs(template.FuncMap{
		"pascal": pascal, "camel": camel, "kebab": kebab, "trimSuffix": strings.TrimSuffix,
	}).Parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}

func templateCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: aspgen templates export PATH | list")
	}
	switch args[0] {
	case "list":
		return fs.WalkDir(templates.FS, "files", func(path string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				fmt.Println(strings.TrimPrefix(path, "files/"))
			}
			return err
		})
	case "export":
		if len(args) < 2 {
			return errors.New("template export path is required")
		}
		return fs.WalkDir(templates.FS, "files", func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			target := filepath.Join(args[1], strings.TrimPrefix(path, "files/"))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			b, err := fs.ReadFile(templates.FS, path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, 0o644)
		})
	case "validate":
		if len(args) < 2 {
			return errors.New("template validation path is required")
		}
		return validateTemplateTree(os.DirFS(args[1]), ".")
	default:
		return fmt.Errorf("unknown templates command %q", args[0])
	}
}

func validateTemplateTree(source fs.FS, root string) error {
	return fs.WalkDir(source, root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return walkErr
		}
		content, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		_, err = template.New(filepath.Base(path)).Funcs(template.FuncMap{
			"pascal": pascal, "camel": camel, "kebab": kebab, "trimSuffix": strings.TrimSuffix,
		}).Parse(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	})
}
