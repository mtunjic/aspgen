package generator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Property struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	CSharpType  string `json:"csharpType"`
	UIControl   string `json:"uiControl"`
	// RelationTarget is the referenced entity name for a synthesized foreign
	// key property, empty for plain scalar properties.
	RelationTarget          string `json:"relationTarget,omitempty"`
	RelationDisplayProperty string `json:"relationDisplayProperty,omitempty"`
}

func parseProperties(args []string) ([]Property, error) {
	var result []Property
	seen := map[string]bool{}
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "--project" || arg == "--templates" || arg == "--framework" || arg == "--context" || arg == "--aggregate" ||
				arg == "-project" || arg == "-templates" || arg == "-framework" || arg == "-context" || arg == "-aggregate" {
				skipNext = true
			}
			continue
		}
		parts := strings.Split(arg, ":")
		if len(parts) < 2 {
			continue
		}
		name, typ := parts[0], parts[1]
		if !validIdentifier(name) || seen[name] {
			return nil, fmt.Errorf("invalid or duplicate property %q", name)
		}
		mapped, ok := mapType(typ)
		if !ok {
			return nil, fmt.Errorf("unsupported property type %q", typ)
		}
		seen[name] = true
		propertyName := pascal(name)
		result = append(result, Property{Name: propertyName, DisplayName: humanize(propertyName), CSharpType: mapped, UIControl: controlForType(mapped)})
	}
	if len(result) == 0 {
		return nil, errors.New("at least one property is required, e.g. name:string")
	}
	return result, nil
}

// csharpTypeInfo describes how a canonical C# type (as returned by mapType,
// without any "?" suffix) renders in the UI and in generated seed data.
type csharpTypeInfo struct {
	UIControl string
	Seed      func(property string, row int) string
}

// userTypeAliases maps the type names accepted on the `add ... name:type`
// command line to their canonical C# type.
var userTypeAliases = map[string]string{
	"string":   "string",
	"int":      "int",
	"long":     "long",
	"decimal":  "decimal",
	"float":    "float",
	"bool":     "bool",
	"date":     "DateOnly",
	"datetime": "DateTime",
	"guid":     "Guid",
	"uuid":     "Guid",
}

// csharpTypes is the single registry of per-canonical-type behavior driving
// both controlForType and seedLiteral.
var csharpTypes = map[string]csharpTypeInfo{
	"string":  {UIControl: "InputText", Seed: func(property string, row int) string { return fmt.Sprintf("\"%s sample %d\"", property, row+1) }},
	"int":     {UIControl: "InputNumber", Seed: func(_ string, row int) string { return fmt.Sprintf("%d", 20+row) }},
	"long":    {UIControl: "InputNumber", Seed: func(_ string, row int) string { return fmt.Sprintf("%dL", 1001+row) }},
	"decimal": {UIControl: "InputNumber", Seed: func(_ string, row int) string { return fmt.Sprintf("%d.50m", 100+row) }},
	"float":   {UIControl: "InputNumber", Seed: func(_ string, row int) string { return fmt.Sprintf("%d.5f", 4+row) }},
	"bool":    {UIControl: "InputCheckbox", Seed: func(_ string, row int) string { return strconv.FormatBool(row%2 == 0) }},
	"DateOnly": {UIControl: "InputDate", Seed: func(_ string, row int) string {
		return fmt.Sprintf("new DateOnly(%d, %d, %d)", 2000+row/336, row%12+1, row%28+1)
	}},
	"DateTime": {UIControl: "InputDate", Seed: func(_ string, row int) string {
		return fmt.Sprintf("new DateTime(%d, %d, %d, 10, 30, 0, DateTimeKind.Utc)", 2000+row/336, row%12+1, row%28+1)
	}},
	"Guid": {UIControl: "InputText", Seed: func(_ string, row int) string {
		return fmt.Sprintf("Guid.Parse(\"00000000-0000-0000-0000-%012d\")", row+1)
	}},
}

func mapType(t string) (string, bool) {
	nullable := strings.HasSuffix(t, "?")
	t = strings.TrimSuffix(t, "?")
	v, ok := userTypeAliases[t]
	if nullable && ok && v != "string" {
		v += "?"
	}
	if nullable && v == "string" {
		v += "?"
	}
	return v, ok
}

func controlForType(t string) string {
	info, ok := csharpTypes[strings.TrimSuffix(t, "?")]
	if !ok {
		return "InputText"
	}
	return info.UIControl
}

// filterKind classifies a canonical C# type (nullable "?" suffix allowed)
// into the filter-field shape templates should render: string (Contains),
// numeric/date (Min/Max range), bool (tri-state), or other (no filter).
func filterKind(csharpType string) string {
	switch strings.TrimSuffix(csharpType, "?") {
	case "string":
		return "string"
	case "int", "long", "decimal", "float":
		return "numeric"
	case "DateOnly", "DateTime":
		return "date"
	case "bool":
		return "bool"
	default:
		return "other"
	}
}

// quickSearchExpr renders a C# boolean expression ORing a null-safe
// .Contains(search) check over every plain string property (relation/FK
// properties are never string-typed, so they're implicitly excluded),
// e.g. `x.Name.Contains(search) || (x.Notes != null && x.Notes.Contains(search))`.
// Returns the literal "false" when there are no string properties to search.
func quickSearchExpr(properties []Property, varName string) string {
	var parts []string
	for _, p := range properties {
		if filterKind(p.CSharpType) != "string" {
			continue
		}
		if strings.HasSuffix(p.CSharpType, "?") {
			parts = append(parts, fmt.Sprintf("(%s.%s != null && %s.%s.Contains(search))", varName, p.Name, varName, p.Name))
		} else {
			parts = append(parts, fmt.Sprintf("%s.%s.Contains(search)", varName, p.Name))
		}
	}
	if len(parts) == 0 {
		return "false"
	}
	return strings.Join(parts, " || ")
}

// hasQuickSearchField reports whether any property is a plain string (the
// only kind quickSearchExpr searches); templates must gate the "if
// (!string.IsNullOrWhiteSpace(search)) query = query.Where(...)" line on
// this, since quickSearchExpr falls back to the literal "false" when there's
// nothing to search, which would otherwise zero out every result the moment
// a user types anything into the search box.
func hasQuickSearchField(properties []Property) bool {
	for _, p := range properties {
		if filterKind(p.CSharpType) == "string" {
			return true
		}
	}
	return false
}

// filterFieldNamesAndType returns the derived advanced-filter field name(s)
// (PascalCase, based on p.Name) and their shared nullable C# type for
// property p, e.g. Name(string) -> (["NameContains"], "string?"),
// Age(int) -> (["AgeMin", "AgeMax"], "int?"), a relation FK property ->
// ([p.Name], "long?"). Returns (nil, "") for unsupported/unfilterable types.
func filterFieldNamesAndType(p Property) ([]string, string) {
	if p.RelationTarget != "" {
		return []string{p.Name}, "long?"
	}
	switch filterKind(p.CSharpType) {
	case "string":
		return []string{p.Name + "Contains"}, "string?"
	case "numeric", "date":
		return []string{p.Name + "Min", p.Name + "Max"}, strings.TrimSuffix(p.CSharpType, "?") + "?"
	case "bool":
		return []string{p.Name}, "bool?"
	default:
		return nil, ""
	}
}

// filterParamDecls renders a leading-comma list of "Type name" declarations
// (one or two per filterable property) for a method signature or record,
// e.g. ", string? nameContains, int? ageMin, int? ageMax". casing is "camel"
// for method parameters or "pascal" for record properties.
func filterParamDecls(properties []Property, casing string) string {
	var parts []string
	for _, p := range properties {
		names, typ := filterFieldNamesAndType(p)
		for _, name := range names {
			if casing == "camel" {
				name = camel(name)
			}
			parts = append(parts, typ+" "+name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// filterParamNames renders a leading-comma list of value references matching
// filterParamDecls's fields, each written as prefix+name, e.g. prefix
// "request." + casing "pascal" -> ", request.NameContains, request.AgeMin,
// request.AgeMax" (forwarding a Query's fields), or prefix "" + casing
// "camel" -> ", nameContains, ageMin, ageMax" (referencing local parameters).
func filterParamNames(properties []Property, prefix, casing string) string {
	var parts []string
	for _, p := range properties {
		names, _ := filterFieldNamesAndType(p)
		for _, name := range names {
			if casing == "camel" {
				name = camel(name)
			}
			parts = append(parts, prefix+name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// filterWhereClauses renders one `if (...) query = query.Where(varName =>
// ...);` C# statement per filter field, conditionally narrowing `query` by
// each supplied (non-null) filter value. When valuePrefix is "" each filter
// value is referenced as a local camelCase parameter (e.g. "nameContains");
// otherwise it's referenced as valuePrefix+PascalCaseName (e.g.
// "criteria.NameContains"). Statements are newline-joined for direct
// insertion into a template's method body.
func filterWhereClauses(properties []Property, varName, valuePrefix string) string {
	ref := func(name string) string {
		if valuePrefix == "" {
			return camel(name)
		}
		return valuePrefix + name
	}
	var lines []string
	appendRange := func(minRef, maxRef, propName string) {
		lines = append(lines, fmt.Sprintf("if (%s.HasValue) query = query.Where(%s => %s.%s >= %s.Value);", minRef, varName, varName, propName, minRef))
		lines = append(lines, fmt.Sprintf("if (%s.HasValue) query = query.Where(%s => %s.%s <= %s.Value);", maxRef, varName, varName, propName, maxRef))
	}
	for _, p := range properties {
		if p.RelationTarget != "" {
			v := ref(p.Name)
			lines = append(lines, fmt.Sprintf("if (%s.HasValue) query = query.Where(%s => %s.%s == %s.Value);", v, varName, varName, p.Name, v))
			continue
		}
		switch filterKind(p.CSharpType) {
		case "string":
			v := ref(p.Name + "Contains")
			condition := fmt.Sprintf("%s.%s.Contains(%s)", varName, p.Name, v)
			if strings.HasSuffix(p.CSharpType, "?") {
				condition = fmt.Sprintf("%s.%s != null && %s", varName, p.Name, condition)
			}
			lines = append(lines, fmt.Sprintf("if (!string.IsNullOrWhiteSpace(%s)) query = query.Where(%s => %s);", v, varName, condition))
		case "numeric", "date":
			appendRange(ref(p.Name+"Min"), ref(p.Name+"Max"), p.Name)
		case "bool":
			v := ref(p.Name)
			lines = append(lines, fmt.Sprintf("if (%s.HasValue) query = query.Where(%s => %s.%s == %s.Value);", v, varName, varName, p.Name, v))
		}
	}
	return strings.Join(lines, "\n        ")
}

func humanize(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	runes := []rune(value)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			previous := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && unicode.IsLower(next)) {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	words := strings.Fields(b.String())
	for i, word := range words {
		if word == "" {
			continue
		}
		wordRunes := []rune(strings.ToLower(word))
		wordRunes[0] = unicode.ToUpper(wordRunes[0])
		words[i] = string(wordRunes)
	}
	return strings.Join(words, " ")
}

func validIdentifier(s string) bool {
	if s == "" || (!unicode.IsLetter(rune(s[0])) && s[0] != '_') {
		return false
	}
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func validProjectName(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return false
	}
	for _, part := range parts {
		if !validIdentifier(part) {
			return false
		}
	}
	return true
}
func pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func camel(s string) string { p := pascal(s); return strings.ToLower(p[:1]) + p[1:] }
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
