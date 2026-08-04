package generator

import (
	"errors"
	"fmt"
)

// importDBCmd implements the `aspgen import-db` top-level verb: incrementally
// add entities to an existing project from a live DB connection or SQL
// script, mirroring add()'s project-discovery/manifest-save tail.
func importDBCmd(args []string) error {
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Print(importDBHelp)
		return nil
	}
	req, ok, err := parseDBImportFlags(args)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("usage: aspgen import-db --project PATH --connection CONN|--script PATH --provider PROVIDER [--tables all|A,B] [flags]; run 'aspgen import-db --help' for details")
	}
	project := value(args, "--project", "")
	if project == "" {
		project, err = discoverProjectRoot(".")
		if err != nil {
			return err
		}
	}
	m, err := loadManifest(project)
	if err != nil {
		return err
	}
	backend, err := backendOption(args)
	if err != nil {
		return err
	}
	if backend == "" {
		backend = componentBackend(m.Components)
	}
	dryRun := hasFlag(args, "--dry-run")
	force := hasFlag(args, "--force")
	if err := runDBImport(project, &m, backend, req, dryRun, force); err != nil {
		return err
	}
	if err := registerGeneratedProjectFiles(project, dryRun); err != nil {
		return err
	}
	if dryRun {
		fmt.Println("would update .aspgen/manifest.json")
		return nil
	}
	return saveManifest(project, m)
}
