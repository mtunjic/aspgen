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
	Name            string `json:"name"`            // navigation property name, e.g. "Tags"
	DisplayName     string `json:"displayName"`     // humanized label for the multi-select, e.g. "Tags"
	Target          string `json:"target"`          // referenced entity name, e.g. "Tag"
	JoinEntity      string `json:"joinEntity"`      // synthesized join entity name, e.g. "PostTag"
	DisplayProperty string `json:"displayProperty"` // Target's display property, for the join entity's picker
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
			navName := pascal(name)
			manyToMany = append(manyToMany, ManyToManyRelation{
				Name:            navName,
				DisplayName:     humanize(navName),
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

// reconstructManyToMany rebuilds the many-to-many relations a manifest's
// persisted EntityMeta implies, for call sites (like retrofitting a UI onto
// aggregates that already existed) that only have the recorded metadata on
// hand, not the original *data.ManyToMany from the `add` call. Prefers the
// explicitly-persisted declaring.ManyToMany; falls back to inferring a
// relation from any join entity (named {declaring}{Target}) recorded with
// exactly two relation FK properties pointing back at the declaring entity
// and at that Target, which is exactly the shape applyManyToManyRenoir
// produces.
func reconstructManyToMany(declaring EntityMeta, entities []EntityMeta) []ManyToManyRelation {
	if len(declaring.ManyToMany) > 0 {
		rels := make([]ManyToManyRelation, 0, len(declaring.ManyToMany))
		for _, rel := range declaring.ManyToMany {
			if rel.DisplayName == "" {
				rel.DisplayName = humanize(rel.Name)
			}
			rels = append(rels, rel)
		}
		return rels
	}
	var rels []ManyToManyRelation
	for _, join := range entities {
		if join.Context != declaring.Context || join.Name == declaring.Name || !strings.HasPrefix(join.Name, declaring.Name) {
			continue
		}
		target := strings.TrimPrefix(join.Name, declaring.Name)
		if target == "" {
			continue
		}
		var fks []Property
		for _, p := range join.Properties {
			if p.RelationTarget != "" {
				fks = append(fks, p)
			}
		}
		if len(fks) != 2 {
			continue
		}
		hasDeclaring, targetFK := false, Property{}
		for _, p := range fks {
			if p.RelationTarget == declaring.Name {
				hasDeclaring = true
			}
			if p.RelationTarget == target {
				targetFK = p
			}
		}
		if !hasDeclaring || targetFK.Name == "" {
			continue
		}
		navName := pascal(target + "s")
		rels = append(rels, ManyToManyRelation{
			Name:            navName,
			DisplayName:     humanize(navName),
			Target:          target,
			JoinEntity:      join.Name,
			DisplayProperty: targetFK.RelationDisplayProperty,
		})
	}
	return rels
}

// injectableTargets returns the deduplicated set of relation Target entity
// names whose store must be injected into a UI's constructor, combining the
// many-to-one relations' targets with the many-to-many relations' targets.
// Many-to-many pickers load their options from the same target Store a
// many-to-one dropdown uses, so an entity with both `tag:Tag` and
// `tags:Tag[]` must not inject I{{ .Target }}Store twice.
func injectableTargets(relations []Relation, manyToMany []ManyToManyRelation) []string {
	var result []string
	seen := map[string]bool{}
	for _, r := range relations {
		if !seen[r.Target] {
			seen[r.Target] = true
			result = append(result, r.Target)
		}
	}
	for _, m := range manyToMany {
		if !seen[m.Target] {
			seen[m.Target] = true
			result = append(result, m.Target)
		}
	}
	return result
}

// buildRelationQuickAdds computes, for every many-to-one relation target, the
// C# expression that constructs a new target Row (Id 0 + the typed display
// value + defaults for every other property) so the WPF edit form's inline
// "+" quick-add can create the related record without leaving the form. Only
// targets whose non-display properties are all safely defaultable qualify.
// The second map names, per target, the entity it must already have been
// created under (its first FK target), for the friendly constraint message.
func buildRelationQuickAdds(relations []Relation, entities []EntityMeta) (map[string]string, map[string]string) {
	rows := map[string]string{}
	missing := map[string]string{}
	for _, rel := range relations {
		target := findEntityMeta(entities, rel.Target)
		if target == nil {
			continue
		}
		if !quickAddFeasible(*target, rel.DisplayProperty) {
			continue
		}
		args := []string{"0"}
		for _, p := range target.Properties {
			if p.Name == rel.DisplayProperty {
				args = append(args, "name")
				continue
			}
			if p.RelationTarget != "" && missing[rel.Target] == "" {
				missing[rel.Target] = p.RelationTarget
			}
			args = append(args, quickAddDefault(p))
		}
		rows[rel.Target] = "new " + rel.Target + "Row(" + strings.Join(args, ", ") + ")"
	}
	return rows, missing
}

// buildBlazorQuickAdds computes, for every quick-addable relation target, the
// C# expression constructing its API Request from the typed display value
// (e.g. "new CustomerRequest(newCustomerName)") for the Blazor edit page's
// inline "+" quick-add. Only targets whose non-display properties default
// safely qualify (the same feasibility rule as WPF).
func buildBlazorQuickAdds(relations []Relation, entities []EntityMeta) map[string]string {
	result := map[string]string{}
	for _, rel := range relations {
		target := findEntityMeta(entities, rel.Target)
		if target == nil {
			continue
		}
		if !quickAddFeasible(*target, rel.DisplayProperty) {
			continue
		}
		var args []string
		for _, p := range target.Properties {
			if p.Name == rel.DisplayProperty {
				args = append(args, "new"+rel.Target+"Name")
				continue
			}
			args = append(args, quickAddDefault(p))
		}
		result[rel.Target] = "new " + rel.Target + "Request(" + strings.Join(args, ", ") + ")"
	}
	return result
}

// quickAddFeasible reports whether a relation target can be created inline
// from just its display property: the display property must be a string, and
// every other property must default safely (nullable, bool, numeric, Guid).
func quickAddFeasible(target EntityMeta, displayProperty string) bool {
	hasDisplay := false
	for _, p := range target.Properties {
		if p.Name == displayProperty {
			if strings.TrimSuffix(p.CSharpType, "?") != "string" {
				return false
			}
			hasDisplay = true
			continue
		}
		if quickAddDefault(p) == "" {
			return false
		}
	}
	return hasDisplay
}

// quickAddDefault renders a default C# literal for a target property used by
// the inline quick-add, or "" when the property can't be defaulted safely
// (required strings, dates, and unknown types are excluded). Nullable value
// types default to null -- a nullable FK must never become 0, which would
// violate its (optional) foreign-key constraint the moment it is saved.
func quickAddDefault(p Property) string {
	switch strings.TrimSuffix(p.CSharpType, "?") {
	case "bool":
		return "false"
	case "int", "long", "decimal", "float":
		if strings.HasSuffix(p.CSharpType, "?") {
			return "null"
		}
		return "0"
	case "Guid":
		if strings.HasSuffix(p.CSharpType, "?") {
			return "null"
		}
		return "Guid.Empty"
	case "string":
		if strings.HasSuffix(p.CSharpType, "?") {
			return "null"
		}
	}
	return ""
}
