package generator

import (
	"errors"
	"fmt"
)

// addContextEntityCmd renders an `ar`-tier entity nested under its bounded
// context (Models/{Context}/{Name}.cs, Features/{Context}/{Name}/...),
// reusing the headless WebApi host and AppDbContext the --context/--arch
// engine's `new` command scaffolds. Higher arch tiers are not implemented
// yet; see /memories/session/plan.md phases 2-4.
func addContextEntityCmd(r addRequest, m *Manifest, ctx Context, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid entity name %q", r.Name)
	}
	if ctx.Arch != "ar" {
		return fmt.Errorf("arch tier %q is not implemented yet; only ar is currently supported", ctx.Arch)
	}
	remainingArgs, relations, _, err := splitRelationArgs(r.Name, r.Args, m.Entities, ctx.Name)
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
	d.Context, d.Properties, d.Relations = ctx.Name, props, relations
	if err := renderTree(r.Project, "ar-entity", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if err := updateContextDbContext(r.Project, m.Project, ctx.Name, r.Name, r.DryRun); err != nil {
		return err
	}
	if err := updateSimpleDbContextRelations(r.Project, r.Name, relations, r.DryRun); err != nil {
		return err
	}
	if err := updateContextFeatureHost(r.Project, m.Project, ctx.Name, r.Name, r.DryRun); err != nil {
		return err
	}
	m.Entities = appendEntityMeta(m.Entities, EntityMeta{Name: r.Name, Context: ctx.Name, Properties: props})
	m.Contexts = appendContextEntity(m.Contexts, ctx.Name, r.Name)
	m.Components = appendUnique(m.Components, "entity:"+ctx.Name+":"+r.Name)
	return nil
}
