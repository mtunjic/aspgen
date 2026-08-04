package generator

import (
	"errors"
	"fmt"
)

func addEntityCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid entity name %q", r.Name)
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	d.Properties = props
	backend := r.Backend
	if isNonSimpleWebAPI(*m) || isLocalDDDWpf(*m, backend) {
		if err := renderTree(r.Project, "entity", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
	}
	if isWPFProject(*m) {
		d.Namespace = m.Project + ".Desktop.Modules." + r.Name
		if err := renderTree(r.Project, "wpf-entity", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		if err := updateModuleCatalog(r.Project, d.Namespace, r.Name, r.DryRun); err != nil {
			return err
		}
	}
	if backend != "" {
		if !isWebAPI(*m) && !isLocalDDDWpf(*m, backend) {
			return errors.New("a backend profile requires a webapi, fullstack, or local DDD wpf project")
		}
		if backend == "simple" {
			if err := renderTree(r.Project, "simple-entity", data{Project: m.Project, Namespace: m.Project, Name: r.Name, Properties: props, Backend: backend}, templateDir(r.Args), r.DryRun, r.Force); err != nil {
				return err
			}
			if err := updateSimpleDbContext(r.Project, m.Project, r.Name, r.DryRun); err != nil {
				return err
			}
		} else if backend == "ddd" {
			if isWebAPI(*m) {
				if err := renderTree(r.Project, "webapi-ddd-entity", data{Project: m.Project, Namespace: m.Project, Name: r.Name, Properties: props, Backend: backend}, templateDir(r.Args), r.DryRun, r.Force); err != nil {
					return err
				}
				if err := updateEntityDependencyInjection(r.Project, m.Project, r.Name, r.DryRun); err != nil {
					return err
				}
				if err := updateEntityDbContext(r.Project, m.Project, r.Name, r.DryRun); err != nil {
					return err
				}
			} else if err := updateEntityDbContextLocal(r.Project, m.Project, r.Name, props, r.DryRun); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported backend %q", backend)
		}
		if isWebAPI(*m) {
			if err := updateFeatureHost(r.Project, m.Project, r.Name, r.DryRun); err != nil {
				return err
			}
		}
		m.Components = appendUnique(m.Components, "backend:"+backend)
	}
	if componentSeed(m.Components) == "dummy" {
		if backend == "" {
			return errors.New("dummy seeding requires a simple or DDD backend")
		}
		seedBackend := backend
		if backend == "ddd" && !isWebAPI(*m) {
			seedBackend = "ddd-local"
		}
		if err := updateSeedFile(r.Project, m.Project, seedBackend, r.Name, props, componentSeedCount(m.Components), r.DryRun); err != nil {
			return err
		}
	}
	m.Components = appendUnique(m.Components, "entity:"+r.Name)
	return nil
}
