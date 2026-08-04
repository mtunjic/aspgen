package generator

import (
	"errors"
	"fmt"
	"path/filepath"
)

func addContextCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid context name %q", r.Name)
	}
	m.Contexts = appendContext(m.Contexts, Context{Name: r.Name})
	m.Components = appendUnique(m.Components, "context:"+r.Name)
	return nil
}

func addAggregateCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid aggregate name %q", r.Name)
	}
	contextName := value(r.Args, "--context", "")
	if contextName == "" || !validIdentifier(contextName) {
		return errors.New("aggregate requires --context ContextName")
	}
	if !contextExists(m.Contexts, contextName) {
		return fmt.Errorf("bounded context %q does not exist; add it first", contextName)
	}
	remainingArgs, relations, manyToMany, err := splitRelationArgs(r.Name, r.Args, m.Entities, contextName)
	if err != nil {
		return err
	}
	var props []Property
	if hasScalarPropertyArgs(remainingArgs) {
		if props, err = parseProperties(remainingArgs); err != nil {
			return err
		}
	}
	if err := rejectAggregateReservedProperties(props); err != nil {
		return err
	}
	for _, rel := range relations {
		props = append(props, synthesizeRelationProperty(rel))
	}
	if len(props) == 0 {
		return errors.New("at least one property or relation is required, e.g. name:string or customer:Customer")
	}
	d.Context, d.Aggregate, d.Properties, d.Relations, d.Crud = contextName, r.Name, props, relations, !hasFlag(r.Args, "--no-crud")
	if !isRenoir(*m) {
		return errors.New("DDD aggregate generation currently requires the blazor/Renoir profile")
	}
	if err := renderTree(r.Project, "renoir-aggregate", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if d.Crud {
		if err := renderTree(r.Project, "renoir-crud", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		appBlazorDir := m.Project + ".AppBlazor"
		using := "using " + m.Project + ".Application;\n"
		registration := "        builder.Services.AddScoped<" + r.Name + "CrudService>();"
		if err := updateBlazorServiceHost(r.Project, appBlazorDir, []string{using}, registration, r.DryRun); err != nil {
			return err
		}
	}
	for _, rel := range relations {
		path := filepath.Join(r.Project, "src", m.Project+".DomainModel", contextName, rel.Target+".cs")
		if err := updateInverseNavigation(path, rel.Target+".cs", r.Name, r.DryRun); err != nil {
			return err
		}
	}
	m.Entities = appendEntityMeta(m.Entities, EntityMeta{Name: r.Name, Context: contextName, Properties: props})
	m.Contexts = appendAggregate(m.Contexts, contextName, r.Name)
	m.Components = appendUnique(m.Components, "aggregate:"+contextName+":"+r.Name)
	if err := applyManyToManyRenoir(r, m, d, contextName, manyToMany, resolveDisplayProperty(EntityMeta{Properties: props})); err != nil {
		return err
	}
	return nil
}

// applyManyToManyRenoir materializes each many-to-many relation declared on
// a Renoir aggregate as its own CRUD-default join aggregate (e.g.
// PostTag) in the same bounded context, carrying a required many-to-one
// relation back to both r.Name and the target aggregate. It reuses the same
// renoir-aggregate/renoir-crud rendering as an ordinary two-relation
// "add aggregate" call, so the join aggregate gets full CRUD, DI wiring, and
// inverse navigations (via the same updateInverseNavigation helper) on both
// r.Name's and the target's own domain classes.
func applyManyToManyRenoir(r addRequest, m *Manifest, d *data, contextName string, manyToMany []ManyToManyRelation, declaringDisplayProperty string) error {
	for _, rel := range manyToMany {
		leftRel := Relation{Name: r.Name, Target: r.Name, FKProperty: r.Name + "Id", DisplayProperty: declaringDisplayProperty}
		rightRel := Relation{Name: rel.Target, Target: rel.Target, FKProperty: rel.Target + "Id", DisplayProperty: rel.DisplayProperty}
		joinRelations := []Relation{leftRel, rightRel}
		joinProps := []Property{synthesizeRelationProperty(leftRel), synthesizeRelationProperty(rightRel)}

		jd := *d
		jd.Aggregate = rel.JoinEntity
		jd.Properties = joinProps
		jd.Relations = joinRelations
		jd.Crud = true

		if err := renderTree(r.Project, "renoir-aggregate", jd, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		appBlazorDir := m.Project + ".AppBlazor"
		using := "using " + m.Project + ".Application;\n"
		registration := "        builder.Services.AddScoped<" + rel.JoinEntity + "CrudService>();"
		if err := renderTree(r.Project, "renoir-crud", jd, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		if err := updateBlazorServiceHost(r.Project, appBlazorDir, []string{using}, registration, r.DryRun); err != nil {
			return err
		}

		for _, jrel := range joinRelations {
			path := filepath.Join(r.Project, "src", m.Project+".DomainModel", contextName, jrel.Target+".cs")
			if err := updateInverseNavigation(path, jrel.Target+".cs", rel.JoinEntity, r.DryRun); err != nil {
				return err
			}
		}
		m.Entities = appendEntityMeta(m.Entities, EntityMeta{Name: rel.JoinEntity, Context: contextName, Properties: joinProps})
		m.Contexts = appendAggregate(m.Contexts, contextName, rel.JoinEntity)
		m.Components = appendUnique(m.Components, "aggregate:"+contextName+":"+rel.JoinEntity)
	}
	return nil
}

func addValueObjectCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid value object name %q", r.Name)
	}
	contextName := value(r.Args, "--context", "")
	if contextName == "" || !contextExists(m.Contexts, contextName) {
		return errors.New("value-object requires an existing --context ContextName")
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	d.Context, d.Properties = contextName, props
	if !isRenoir(*m) {
		return errors.New("DDD value-object generation currently requires the blazor/Renoir profile")
	}
	if err := renderTree(r.Project, "renoir-value-object", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "value-object:"+contextName+":"+r.Name)
	return nil
}

func addDomainServiceCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid domain service name %q", r.Name)
	}
	contextName := value(r.Args, "--context", "")
	if contextName == "" || !contextExists(m.Contexts, contextName) {
		return errors.New("domain-service requires an existing --context ContextName")
	}
	d.Context = contextName
	if !isRenoir(*m) {
		return errors.New("DDD domain-service generation currently requires the blazor/Renoir profile")
	}
	if err := renderTree(r.Project, "renoir-domain-service", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "domain-service:"+contextName+":"+r.Name)
	return nil
}

func addRepositoryCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid repository name %q", r.Name)
	}
	contextName := value(r.Args, "--context", "")
	aggregateName := value(r.Args, "--aggregate", "")
	if contextName == "" || !contextExists(m.Contexts, contextName) || aggregateName == "" || !validIdentifier(aggregateName) {
		return errors.New("repository requires an existing --context ContextName and --aggregate AggregateName")
	}
	d.Context, d.Aggregate = contextName, aggregateName
	if !isRenoir(*m) {
		return errors.New("DDD repository generation currently requires the blazor/Renoir profile")
	}
	if err := renderTree(r.Project, "renoir-repository", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	appBlazorDir := m.Project + ".AppBlazor"
	usings := []string{
		"using " + m.Project + ".DomainModel." + contextName + ";\n",
		"using " + m.Project + ".Persistence.Repositories;\n",
	}
	registration := "        builder.Services.AddScoped<I" + r.Name + ", " + r.Name + ">();"
	if err := updateBlazorServiceHost(r.Project, appBlazorDir, usings, registration, r.DryRun); err != nil {
		return err
	}
	if err := wireCrudServiceToRepository(r, m, aggregateName, r.Name); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "repository:"+contextName+":"+aggregateName)
	return nil
}

func addEventCmd(r addRequest, m *Manifest, d *data) error {
	if !validIdentifier(r.Name) {
		return fmt.Errorf("invalid domain event name %q", r.Name)
	}
	contextName := value(r.Args, "--context", "")
	if contextName == "" || !contextExists(m.Contexts, contextName) {
		return errors.New("event requires an existing --context ContextName")
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	d.Context, d.Properties = contextName, props
	if !isRenoir(*m) {
		return errors.New("DDD event generation currently requires the blazor/Renoir profile")
	}
	if err := renderTree(r.Project, "renoir-event", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "event:"+contextName+":"+r.Name)
	return nil
}
