package generator

import (
	"fmt"
	"strings"
)

// Relation describes a many-to-one reference from the entity currently being
// added to an already-existing target entity, declared via `nav:Entity`
// (optionally `nav:Entity?`) alongside the usual `name:type` property args.
type Relation struct {
	Name            string // navigation property name, e.g. "Customer"
	Target          string // referenced entity name, e.g. "Customer"
	FKProperty      string // synthesized foreign key property name, e.g. "CustomerId"
	DisplayProperty string // target property shown as a picker's label
	Optional        bool
}

// ManyToManyRelation describes a many-to-many reference declared via
// `nav:Entity[]`. It is materialized as a join entity (e.g. "PostTag") that
// carries a required many-to-one relation back to both the declaring entity
// and Target, reusing the same entity/nav/config rendering as any other
// two-relation entity instead of introducing bespoke join-table templates.
type ManyToManyRelation struct {
	Name            string // navigation property name, e.g. "Tags"
	Target          string // referenced entity name, e.g. "Tag"
	JoinEntity      string // synthesized join entity name, e.g. "PostTag"
	DisplayProperty string // Target's display property, for the join entity's picker
}

// splitRelationArgs pulls `nav:Entity`/`nav:Entity?`/`nav:Entity[]` tokens
// that reference an already-added entity out of args, returning the
// remaining tokens for parseProperties untouched, the parsed many-to-one
// relations, and the parsed many-to-many relations. declaring is the name of
// the entity being added (used to name synthesized join entities). context
// restricts matches to entities recorded with the same context (empty for
// simple/ddd entities; the bounded context name for Renoir aggregates), and
// a same-name entity recorded in a different context is a hard error rather
// than falling through to parseProperties.
func splitRelationArgs(declaring string, args []string, entities []EntityMeta, context string) (remaining []string, relations []Relation, manyToMany []ManyToManyRelation, err error) {
	seen := map[string]bool{}
	skipNext := false
	for _, arg := range args {
		if skipNext {
			remaining = append(remaining, arg)
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if isPropertySkipFlag(arg) {
				skipNext = true
			}
			remaining = append(remaining, arg)
			continue
		}
		name, typ, ok := strings.Cut(arg, ":")
		if !ok {
			remaining = append(remaining, arg)
			continue
		}
		if strings.HasSuffix(typ, "[]") {
			typeName := strings.TrimSuffix(typ, "[]")
			match, found := findEntityMetaAnyContext(entities, typeName)
			if !found {
				remaining = append(remaining, arg)
				continue
			}
			if match.Context != context {
				return nil, nil, nil, fmt.Errorf("relation target %q is in context %q, not %q; only same-context relations are supported", match.Name, match.Context, context)
			}
			if !validIdentifier(name) || seen[name] {
				return nil, nil, nil, fmt.Errorf("invalid or duplicate relation %q", name)
			}
			seen[name] = true
			manyToMany = append(manyToMany, ManyToManyRelation{
				Name:            pascal(name),
				Target:          match.Name,
				JoinEntity:      declaring + match.Name,
				DisplayProperty: resolveDisplayProperty(match),
			})
			continue
		}
		optional := strings.HasSuffix(typ, "?")
		typeName := strings.TrimSuffix(typ, "?")
		match, found := findEntityMetaAnyContext(entities, typeName)
		if !found {
			remaining = append(remaining, arg)
			continue
		}
		if match.Context != context {
			return nil, nil, nil, fmt.Errorf("relation target %q is in context %q, not %q; only same-context relations are supported", match.Name, match.Context, context)
		}
		if !validIdentifier(name) || seen[name] {
			return nil, nil, nil, fmt.Errorf("invalid or duplicate relation %q", name)
		}
		seen[name] = true
		navName := pascal(name)
		relations = append(relations, Relation{
			Name:            navName,
			Target:          match.Name,
			FKProperty:      navName + "Id",
			DisplayProperty: resolveDisplayProperty(match),
			Optional:        optional,
		})
	}
	return remaining, relations, manyToMany, nil
}

// ReverseRelation describes a many-to-one relation declared on ANOTHER
// entity (SourceEntity) that points back at the entity currently being
// rendered, surfaced so a Details page can show a read-only child
// collection (e.g. a Customer's Details page lists its Orders). Because a
// relation target must already exist in the manifest before anything can
// reference it, the target is always rendered before its reverse relations
// exist - they can only ever be discovered later, when the referring entity
// is added (see patchReverseRelationUI in add_ddd.go).
type ReverseRelation struct {
	SourceEntity  string // entity holding the FK, e.g. "Order"
	SourceContext string
	FKProperty    string // e.g. "CustomerId"
	DisplayName   string // pluralized nav name shown in the UI, e.g. "Orders"
}

// computeReverseRelations scans every entity recorded in the same context as
// targetEntity for a many-to-one relation pointing back at it.
func computeReverseRelations(entities []EntityMeta, targetEntity, context string) []ReverseRelation {
	var out []ReverseRelation
	for _, e := range entities {
		if e.Context != context || e.Name == targetEntity {
			continue
		}
		for _, p := range e.Properties {
			if p.RelationTarget != targetEntity {
				continue
			}
			out = append(out, ReverseRelation{
				SourceEntity:  e.Name,
				SourceContext: e.Context,
				FKProperty:    p.Name,
				DisplayName:   e.Name + "s",
			})
		}
	}
	return out
}

// isPropertySkipFlag reports whether arg is a flag whose value token must
// also be skipped when scanning for `name:type` property/relation args.
func isPropertySkipFlag(arg string) bool {
	switch arg {
	case "--project", "--templates", "--framework", "--context", "--aggregate",
		"-project", "-templates", "-framework", "-context", "-aggregate":
		return true
	}
	return false
}

// hasScalarPropertyArgs reports whether args contains at least one plain
// `name:type` token (ignoring flags), used to decide whether parseProperties
// should run at all when an entity is declared with relations only.
func hasScalarPropertyArgs(args []string) bool {
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if isPropertySkipFlag(arg) {
				skipNext = true
			}
			continue
		}
		if strings.Contains(arg, ":") {
			return true
		}
	}
	return false
}

func findEntityMetaAnyContext(entities []EntityMeta, name string) (EntityMeta, bool) {
	for _, e := range entities {
		if strings.EqualFold(e.Name, name) {
			return e, true
		}
	}
	return EntityMeta{}, false
}

// resolveDisplayProperty picks the property shown as a relation picker's
// label: the target's first string property, falling back to Id.
func resolveDisplayProperty(target EntityMeta) string {
	for _, p := range target.Properties {
		if p.CSharpType == "string" {
			return p.Name
		}
	}
	return "Id"
}

// synthesizeRelationProperty builds the FK scalar Property for rel, carrying
// the display metadata templates need to render a picker control instead of
// a plain numeric input.
func synthesizeRelationProperty(rel Relation) Property {
	csharpType := "long"
	if rel.Optional {
		csharpType = "long?"
	}
	return Property{
		Name:                    rel.FKProperty,
		DisplayName:             humanize(rel.Name),
		CSharpType:              csharpType,
		UIControl:               "InputNumber",
		RelationTarget:          rel.Target,
		RelationDisplayProperty: rel.DisplayProperty,
	}
}

// reconstructRelations rebuilds the []Relation slice a manifest's persisted
// EntityMeta.Properties implies, for call sites (like retrofitting a UI onto
// aggregates that already existed) that only have Properties on hand, not
// the original *data.Relations from the `add` call that created them. The
// navigation name is recovered by trimming the FKProperty's "Id" suffix,
// which is always how synthesizeRelationProperty built it in the first
// place (FKProperty = navName + "Id"); Optional is recovered from whether
// the FK's CSharpType is nullable ("long?" vs "long").
func reconstructRelations(properties []Property) []Relation {
	var relations []Relation
	for _, p := range properties {
		if p.RelationTarget == "" {
			continue
		}
		relations = append(relations, Relation{
			Name:            strings.TrimSuffix(p.Name, "Id"),
			Target:          p.RelationTarget,
			FKProperty:      p.Name,
			DisplayProperty: p.RelationDisplayProperty,
			Optional:        strings.HasSuffix(p.CSharpType, "?"),
		})
	}
	return relations
}
