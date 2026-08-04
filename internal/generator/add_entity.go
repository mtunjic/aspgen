package generator

import (
	"errors"
	"fmt"
	"path/filepath"
)

func addEntityCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid entity name %q", r.Name)
	}
	remainingArgs, relations, manyToMany, err := splitRelationArgs(r.Name, r.Args, m.Entities, "")
	if err != nil {
		return err
	}
	var props []Property
	if hasScalarPropertyArgs(remainingArgs) {
		if props, err = parseProperties(remainingArgs); err != nil {
			return err
		}
	}
	for _, rel := range relations {
		props = append(props, synthesizeRelationProperty(rel))
	}
	if len(props) == 0 {
		return errors.New("at least one property or relation is required, e.g. name:string or customer:Customer")
	}
	d.Properties = props
	d.Relations = relations
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
			if err := renderTree(r.Project, "simple-entity", data{Project: m.Project, Namespace: m.Project, Name: r.Name, Properties: props, Relations: relations, Backend: backend}, templateDir(r.Args), r.DryRun, r.Force); err != nil {
				return err
			}
			if err := updateSimpleDbContext(r.Project, m.Project, r.Name, r.DryRun); err != nil {
				return err
			}
			if err := updateSimpleDbContextRelations(r.Project, r.Name, relations, r.DryRun); err != nil {
				return err
			}
		} else if backend == "ddd" {
			if isWebAPI(*m) {
				if err := renderTree(r.Project, "webapi-ddd-entity", data{Project: m.Project, Namespace: m.Project, Name: r.Name, Properties: props, Relations: relations, Backend: backend}, templateDir(r.Args), r.DryRun, r.Force); err != nil {
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
	if err := applyInverseRelations(r, m, backend, relations); err != nil {
		return err
	}
	if err := applyManyToMany(r, m, backend, manyToMany, resolveDisplayProperty(EntityMeta{Properties: props})); err != nil {
		return err
	}
	m.Entities = appendEntityMeta(m.Entities, EntityMeta{Name: r.Name, Properties: props})
	m.Components = appendUnique(m.Components, "entity:"+r.Name)
	return nil
}

// applyInverseRelations adds a read-only inverse collection navigation for
// r.Name onto each relation target's already-generated model/domain class,
// and (for WPF projects) a related grid and store on the target's view/view
// model, so the "many" side of a many-to-one relation displays its children.
func applyInverseRelations(r addRequest, m *Manifest, backend string, relations []Relation) error {
	for _, rel := range relations {
		if isNonSimpleWebAPI(*m) || isLocalDDDWpf(*m, backend) {
			path := filepath.Join(r.Project, "src", "Domain", "Entities", rel.Target+".cs")
			if err := updateInverseNavigation(path, rel.Target+".cs", r.Name, r.DryRun); err != nil {
				return err
			}
		}
		if backend == "simple" {
			path := filepath.Join(r.Project, "src", "WebApi", "Models", rel.Target+".cs")
			if err := updateInverseNavigation(path, rel.Target+".cs", r.Name, r.DryRun); err != nil {
				return err
			}
		}
		if isWPFProject(*m) {
			if err := updateRelatedGrid(r.Project, rel.Target, r.Name, componentTheme(m.Components), r.DryRun); err != nil {
				return err
			}
			if err := updateRelatedStore(r.Project, m.Project, rel.Target, r.Name, r.DryRun); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyManyToMany materializes each many-to-many relation declared on r.Name
// as a join entity (e.g. PostTag) carrying a required many-to-one relation
// back to both r.Name and the target entity. It reuses the same rendering
// and marker helpers as an ordinary two-relation add entity call, so the
// join entity gets full CRUD, DbSet wiring, and inverse collections (via
// applyInverseRelations) on both r.Name's and the target's own views.
func applyManyToMany(r addRequest, m *Manifest, backend string, manyToMany []ManyToManyRelation, declaringDisplayProperty string) error {
	for _, rel := range manyToMany {
		leftRel := Relation{Name: r.Name, Target: r.Name, FKProperty: r.Name + "Id", DisplayProperty: declaringDisplayProperty}
		rightRel := Relation{Name: rel.Target, Target: rel.Target, FKProperty: rel.Target + "Id", DisplayProperty: rel.DisplayProperty}
		joinRelations := []Relation{leftRel, rightRel}
		joinProps := []Property{synthesizeRelationProperty(leftRel), synthesizeRelationProperty(rightRel)}
		jr := r
		jr.Name = rel.JoinEntity

		if isNonSimpleWebAPI(*m) || isLocalDDDWpf(*m, backend) {
			jd := data{Project: m.Project, Namespace: m.Project, Name: jr.Name, Properties: joinProps, Relations: joinRelations, Backend: backend}
			if err := renderTree(jr.Project, "entity", jd, templateDir(jr.Args), jr.DryRun, jr.Force); err != nil {
				return err
			}
		}
		if isWPFProject(*m) {
			jd := data{Project: m.Project, Namespace: m.Project + ".Desktop.Modules." + jr.Name, Name: jr.Name, Properties: joinProps, Relations: joinRelations, Backend: backend, Theme: r.Theme}
			if err := renderTree(jr.Project, "wpf-entity", jd, templateDir(jr.Args), jr.DryRun, jr.Force); err != nil {
				return err
			}
			if err := updateModuleCatalog(jr.Project, jd.Namespace, jr.Name, jr.DryRun); err != nil {
				return err
			}
		}
		if backend == "simple" {
			jd := data{Project: m.Project, Namespace: m.Project, Name: jr.Name, Properties: joinProps, Relations: joinRelations, Backend: backend}
			if err := renderTree(jr.Project, "simple-entity", jd, templateDir(jr.Args), jr.DryRun, jr.Force); err != nil {
				return err
			}
			if err := updateSimpleDbContext(jr.Project, m.Project, jr.Name, jr.DryRun); err != nil {
				return err
			}
			if err := updateSimpleDbContextRelations(jr.Project, jr.Name, joinRelations, jr.DryRun); err != nil {
				return err
			}
		} else if backend == "ddd" {
			if isWebAPI(*m) {
				jd := data{Project: m.Project, Namespace: m.Project, Name: jr.Name, Properties: joinProps, Relations: joinRelations, Backend: backend}
				if err := renderTree(jr.Project, "webapi-ddd-entity", jd, templateDir(jr.Args), jr.DryRun, jr.Force); err != nil {
					return err
				}
				if err := updateEntityDependencyInjection(jr.Project, m.Project, jr.Name, jr.DryRun); err != nil {
					return err
				}
				if err := updateEntityDbContext(jr.Project, m.Project, jr.Name, jr.DryRun); err != nil {
					return err
				}
			} else if err := updateEntityDbContextLocal(jr.Project, m.Project, jr.Name, joinProps, jr.DryRun); err != nil {
				return err
			}
		}
		if backend != "" && isWebAPI(*m) {
			if err := updateFeatureHost(jr.Project, m.Project, jr.Name, jr.DryRun); err != nil {
				return err
			}
		}
		if err := applyInverseRelations(jr, m, backend, joinRelations); err != nil {
			return err
		}
		m.Entities = appendEntityMeta(m.Entities, EntityMeta{Name: jr.Name, Properties: joinProps})
		m.Components = appendUnique(m.Components, "entity:"+jr.Name)
	}
	return nil
}
