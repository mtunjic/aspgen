package generator

import (
	"errors"
	"fmt"
)

func addModuleCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid module name %q", r.Name)
	}
	if !isWPFProject(*m) {
		return errors.New("module generation requires a WPF UI; add ui first")
	}
	d.Namespace = m.Project + ".Modules." + r.Name
	if err := renderTree(r.Project, "module", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if err := updateModuleCatalog(r.Project, d.Namespace, r.Name, r.DryRun); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "module:"+r.Name)
	return nil
}
