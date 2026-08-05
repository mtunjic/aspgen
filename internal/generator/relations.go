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
