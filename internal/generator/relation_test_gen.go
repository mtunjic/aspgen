package generator

import (
	"strconv"
	"strings"
)

// RelationTest carries everything the generated relation unit test
// (tests-relations template group) needs to exercise an aggregate's
// many-to-one and many-to-many EF relationships end-to-end: create the
// related entities and join rows through the DbContext, then query them back
// by foreign key. It is resolved from the manifest at `add` time so the
// template never has to guess target property shapes.
type RelationTest struct {
	Project    string
	Aggregate  string
	Var        string   // variable name for the aggregate instance
	CtorArgs   []string // `new Aggregate(...)` argument expressions
	Relations  []RelationTestTarget
	ManyToMany []RelationTestTarget
	// FormFields are the HTML form field name/value pairs the aggregate's
	// Create/Edit POST binds to its Request record, used by the MVC
	// integration test to drive the real controller flow.
	FormFields []RelationTestFormField
	// HasStringProp/FirstStringProp let the MVC integration test submit an
	// invalid form (empty first string property) to exercise validation.
	HasStringProp    bool
	FirstStringProp  string
}

// RelationTestFormField is one `name = value` pair posted to the MVC
// controller; Value is a C# expression evaluating to the raw form value.
type RelationTestFormField struct {
	Name  string
	Value string
}

// RelationTestTarget describes one related entity the test must create (and,
// for many-to-many, the join entity whose rows link it to the aggregate).
type RelationTestTarget struct {
	Target     string
	Var        string
	SecondVar  string // second instance for many-to-many
	JoinEntity string // many-to-many join aggregate name
	CtorArgs   []string
	// RequestArgs are the positional arguments for the target's API Request
	// record, used by the WebApi integration test to create the target over
	// HTTP (FK properties fall back to a numeric seed rather than an id
	// reference, since the target is created before any principal exists).
	RequestArgs []string
	// DisplayProperty is the target's display property (e.g. "Name"), used to
	// name the nested relation filter (e.g. CustomerNameContains) in tests.
	DisplayProperty string
}

// buildRelationTest resolves the aggregate and its relation targets from the
// manifest into a RelationTest. Targets must already exist (relation targets
// are validated to exist at parse time).
func buildRelationTest(m *Manifest, contextName, aggregate string, props []Property, relations []Relation, manyToMany []ManyToManyRelation) *RelationTest {
	rt := &RelationTest{
		Project:   m.Project,
		Aggregate: aggregate,
		Var:       camel(aggregate),
		CtorArgs:  testCtorArgs(props),
	}
	for _, rel := range relations {
		target := findEntityMeta(m.Entities, rel.Target)
		rt.Relations = append(rt.Relations, RelationTestTarget{
			Target:          rel.Target,
			Var:             camel(rel.Target),
			CtorArgs:        testCtorArgs(target.Properties),
			RequestArgs:     testRequestArgs(target.Properties),
			DisplayProperty: rel.DisplayProperty,
		})
	}
	for _, rel := range manyToMany {
		target := findEntityMeta(m.Entities, rel.Target)
		rt.ManyToMany = append(rt.ManyToMany, RelationTestTarget{
			Target:          rel.Target,
			Var:             camel(rel.Target) + "1",
			SecondVar:       camel(rel.Target) + "2",
			JoinEntity:      rel.JoinEntity,
			CtorArgs:        testCtorArgs(target.Properties),
			RequestArgs:     testRequestArgs(target.Properties),
			DisplayProperty: rel.DisplayProperty,
		})
	}
	for _, p := range props {
		if p.RelationTarget != "" {
			rt.FormFields = append(rt.FormFields, RelationTestFormField{Name: p.Name, Value: camel(p.RelationTarget) + ".ToString()"})
			continue
		}
		rt.FormFields = append(rt.FormFields, RelationTestFormField{Name: p.Name, Value: strconv.Quote(testFormValue(p))})
		if strings.TrimSuffix(p.CSharpType, "?") == "string" {
			rt.HasStringProp = true
			if rt.FirstStringProp == "" {
				rt.FirstStringProp = p.Name
			}
		}
	}
	return rt
}

// testCtorArgs renders the positional constructor-argument expressions for
// an entity: a foreign-key property becomes `<targetVar>.Id` (the created
// related instance's id), anything else becomes a deterministic seed literal
// for its C# type.
func testCtorArgs(properties []Property) []string {
	var args []string
	for _, p := range properties {
		if p.RelationTarget != "" {
			args = append(args, camel(p.RelationTarget)+".Id")
			continue
		}
		args = append(args, testSeedLiteral(p))
	}
	return args
}

// testRequestArgs renders positional seed literals for every property
// (including foreign keys, which get a plain numeric seed) to construct an
// API Request record that does not reference other test entities.
func testRequestArgs(properties []Property) []string {
	var args []string
	for _, p := range properties {
		args = append(args, testSeedLiteral(p))
	}
	return args
}

// testFormValue renders the raw (culture-independent) form value an MVC input
// posts for a scalar property.
func testFormValue(p Property) string {
	switch strings.TrimSuffix(p.CSharpType, "?") {
	case "string":
		return p.Name + " sample"
	case "int":
		return "20"
	case "long":
		return "1001"
	case "decimal":
		return "100"
	case "float":
		return "4"
	case "bool":
		return "true"
	case "DateOnly":
		return "2020-01-01"
	case "DateTime":
		return "2020-01-01T10:30:00"
	case "Guid":
		return "00000000-0000-0000-0000-000000000001"
	}
	return "0"
}

// testSeedLiteral renders a C# literal for property p's canonical type using
// the same per-type registry the dummy seed generator uses.
func testSeedLiteral(p Property) string {
	info, ok := csharpTypes[strings.TrimSuffix(p.CSharpType, "?")]
	if !ok {
		return "default"
	}
	return info.Seed(p.Name, 0)
}
