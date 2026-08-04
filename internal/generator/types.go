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
