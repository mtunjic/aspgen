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
	arch, err := archOption(r.Args)
	if err != nil {
		return err
	}
	if arch != "" && arch != "ar" && arch != "dm" && arch != "cqrs" && arch != "es" {
		return fmt.Errorf("arch tier %q is not implemented yet; only ar, dm, cqrs, and es are currently supported", arch)
	}
	if existing, ok := findContext(m.Contexts, r.Name); ok && arch != "" && existing.Arch != "" && existing.Arch != arch {
		return fmt.Errorf("context %q already exists with arch %q", r.Name, existing.Arch)
	}
	m.Contexts = appendContextWithArch(m.Contexts, r.Name, arch)
	m.Components = appendUnique(m.Components, "context:"+r.Name)
	if arch != "" {
		m.Components = appendUnique(m.Components, "context-engine")
	}
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
	ctx, err := requireDDDContext(m, contextName, "aggregate")
	if err != nil {
		return err
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
	if err := rejectAggregateReservedProperties(props, ctx.Arch); err != nil {
		return err
	}
	for _, rel := range relations {
		props = append(props, synthesizeRelationProperty(rel))
	}
	if len(props) == 0 {
		return errors.New("at least one property or relation is required, e.g. name:string or customer:Customer")
	}
	d.Context, d.Aggregate, d.Properties, d.Relations, d.Crud = contextName, r.Name, props, relations, !hasFlag(r.Args, "--no-crud")
	if err := renderTree(r.Project, aggregateTemplateGroup(ctx.Arch), *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if d.Crud {
		if err := renderAggregateCrud(r, m, *d, ctx.Arch); err != nil {
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
	if err := applyManyToManyRenoir(r, m, d, contextName, manyToMany, resolveDisplayProperty(EntityMeta{Properties: props}), ctx.Arch); err != nil {
		return err
	}
	return nil
}

// aggregateTemplateGroup picks the template group that renders an
// aggregate's DomainModel (and, for es, Persistence read-model) shape:
// es-tier aggregates are event-sourced (see es-aggregate/EventSourcedAggregate),
// every other tier (including legacy Renoir) uses the plain BaseEntity shape.
func aggregateTemplateGroup(arch string) string {
	if arch == "es" {
		return "es-aggregate"
	}
	return "renoir-aggregate"
}

// requireDDDContext validates that contextName exists and is capable of
// dm-tier (or higher) constructs (aggregates/value-objects/domain-services/
// repositories/events). New-engine contexts (Arch != "") must be dm or
// higher; legacy Renoir contexts (Arch == "") require the blazor/Renoir
// profile, exactly as before this engine existed.
func requireDDDContext(m *Manifest, contextName, kind string) (Context, error) {
	ctx, ok := findContext(m.Contexts, contextName)
	if !ok {
		return Context{}, fmt.Errorf("bounded context %q does not exist; add it first", contextName)
	}
	if ctx.Arch != "" {
		if !archAtLeast(ctx.Arch, "dm") {
			return Context{}, fmt.Errorf("%s requires a dm/cqrs/es context; %q is arch tier %q (use add entity instead)", kind, contextName, ctx.Arch)
		}
		return ctx, nil
	}
	if !isRenoir(*m) {
		// Deprecated: this legacy blazor/Renoir path (Arch == "") is kept
		// working but superseded by --context/--arch dm+ contexts above.
		return Context{}, fmt.Errorf("DDD %s generation currently requires the blazor/Renoir profile", kind)
	}
	return ctx, nil
}

// renderAggregateCrud renders the aggregate's Application-layer CRUD service
// and validator, additionally wiring it into a host for tiers that have one:
// legacy Renoir contexts (arch == "") get Blazor DI + Razor CRUD pages;
// cqrs-tier contexts get the CrudService registered with the WebApi host's
// own DI plus a full vertical-slice Command/Query/Handler + Minimal API
// endpoints layer mounted on that host; es-tier contexts get the same
// vertical-slice/endpoint treatment but backed by a generated
// {Aggregate}EventStoreRepository instead of a CrudService (no CrudService
// is rendered for es). dm-tier contexts get the Application-layer files
// only; there is no host to wire into yet (Phase 5, -ui is not implemented).
func renderAggregateCrud(r addRequest, m *Manifest, d data, arch string) error {
	switch arch {
	case "":
		if err := renderTree(r.Project, "renoir-crud", d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		appBlazorDir := m.Project + ".AppBlazor"
		using := "using " + m.Project + ".Application;\n"
		registration := "        builder.Services.AddScoped<" + d.Aggregate + "CrudService>();"
		return updateBlazorServiceHost(r.Project, appBlazorDir, []string{using}, registration, r.DryRun)
	case "cqrs":
		if err := renderTree(r.Project, "dm-crud", d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		applicationDir := m.Project + ".Application"
		registration := "        services.AddScoped<" + d.Aggregate + "CrudService>();"
		if err := updateApplicationServiceHost(r.Project, applicationDir, registration, r.DryRun); err != nil {
			return err
		}
		if err := renderTree(r.Project, "cqrs-feature", d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		return updateContextFeatureHost(r.Project, m.Project, d.Context, d.Aggregate, r.DryRun)
	case "es":
		applicationDir := m.Project + ".Application"
		registration := "        services.AddScoped<" + d.Aggregate + "EventStoreRepository>();"
		if err := updateApplicationServiceHost(r.Project, applicationDir, registration, r.DryRun); err != nil {
			return err
		}
		if err := renderTree(r.Project, "es-feature", d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		return updateContextFeatureHost(r.Project, m.Project, d.Context, d.Aggregate, r.DryRun)
	default:
		// dm tier (and any future tier without its own case) gets the
		// CrudService-only rendering; there is no host to wire into yet.
		return renderTree(r.Project, "dm-crud", d, templateDir(r.Args), r.DryRun, r.Force)
	}
}

// applyManyToManyRenoir materializes each many-to-many relation declared on
// a Renoir aggregate as its own CRUD-default join aggregate (e.g.
// PostTag) in the same bounded context, carrying a required many-to-one
// relation back to both r.Name and the target aggregate. It reuses the same
// renoir-aggregate/renoir-crud rendering as an ordinary two-relation
// "add aggregate" call, so the join aggregate gets full CRUD, DI wiring, and
// inverse navigations (via the same updateInverseNavigation helper) on both
// r.Name's and the target's own domain classes.
func applyManyToManyRenoir(r addRequest, m *Manifest, d *data, contextName string, manyToMany []ManyToManyRelation, declaringDisplayProperty string, arch string) error {
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

		if err := renderTree(r.Project, aggregateTemplateGroup(arch), jd, templateDir(r.Args), r.DryRun, r.Force); err != nil {
			return err
		}
		if err := renderAggregateCrud(r, m, jd, arch); err != nil {
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
	if contextName == "" {
		return errors.New("value-object requires an existing --context ContextName")
	}
	if _, err := requireDDDContext(m, contextName, "value-object"); err != nil {
		return err
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	d.Context, d.Properties = contextName, props
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
	if contextName == "" {
		return errors.New("domain-service requires an existing --context ContextName")
	}
	if _, err := requireDDDContext(m, contextName, "domain-service"); err != nil {
		return err
	}
	d.Context = contextName
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
	if contextName == "" || aggregateName == "" || !validIdentifier(aggregateName) {
		return errors.New("repository requires an existing --context ContextName and --aggregate AggregateName")
	}
	ctx, err := requireDDDContext(m, contextName, "repository")
	if err != nil {
		return err
	}
	if ctx.Arch == "es" {
		return fmt.Errorf("es-tier aggregates already have a generated %sEventStoreRepository; add repository is not applicable", aggregateName)
	}
	d.Context, d.Aggregate = contextName, aggregateName
	if err := renderTree(r.Project, "renoir-repository", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	if ctx.Arch == "" {
		appBlazorDir := m.Project + ".AppBlazor"
		usings := []string{
			"using " + m.Project + ".DomainModel." + contextName + ";\n",
			"using " + m.Project + ".Persistence.Repositories;\n",
		}
		registration := "        builder.Services.AddScoped<I" + r.Name + ", " + r.Name + ">();"
		if err := updateBlazorServiceHost(r.Project, appBlazorDir, usings, registration, r.DryRun); err != nil {
			return err
		}
	} else if ctx.Arch == "cqrs" {
		infrastructureDir := m.Project + ".Infrastructure"
		usings := []string{
			"using " + m.Project + ".DomainModel." + contextName + ";\n",
			"using " + m.Project + ".Persistence.Repositories;\n",
		}
		registration := "        services.AddScoped<I" + r.Name + ", " + r.Name + ">();"
		if err := updateInfrastructureRepositoryHost(r.Project, infrastructureDir, usings, registration, r.DryRun); err != nil {
			return err
		}
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
	if contextName == "" {
		return errors.New("event requires an existing --context ContextName")
	}
	if _, err := requireDDDContext(m, contextName, "event"); err != nil {
		return err
	}
	props, err := parseProperties(r.Args)
	if err != nil {
		return err
	}
	d.Context, d.Properties = contextName, props
	if err := renderTree(r.Project, "renoir-event", *d, templateDir(r.Args), r.DryRun, r.Force); err != nil {
		return err
	}
	m.Components = appendUnique(m.Components, "event:"+contextName+":"+r.Name)
	return nil
}
