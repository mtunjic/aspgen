package dbschema

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseScript extracts table/column definitions from a static SQL DDL script
// containing one or more CREATE TABLE statements. provider only affects
// identifier-quoting expectations (", `, [ ]) — column raw types are passed
// through untouched for MapColumnType to interpret per dialect.
//
// This is an intentionally minimal, non-grammar parser: it does not support
// ALTER TABLE, views, generated/computed columns, or nested subqueries.
// FOREIGN KEY constraints are recognized (inline `REFERENCES tbl(col)` and
// table-level `FOREIGN KEY (col) REFERENCES tbl(col)`, single-column only);
// CHECK/DEFAULT clauses are skipped over and never surfaced as columns.
func ParseScript(provider, sqlText string) ([]Table, error) {
	body := stripComments(sqlText)
	re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + identifierPattern + `)`)
	locs := re.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("no CREATE TABLE statements found")
	}
	var tables []Table
	for _, loc := range locs {
		name := unquoteIdentifier(body[loc[2]:loc[3]])
		rest := body[loc[1]:]
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			return nil, fmt.Errorf("table %q: missing column list", name)
		}
		start := loc[1] + open
		end, err := matchingParen(body, start)
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", name, err)
		}
		columns, err := parseColumns(body[start+1 : end])
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", name, err)
		}
		tables = append(tables, Table{Name: name, Columns: columns})
	}
	return tables, nil
}

// identifierPattern matches a quoted ("x", `x`, [x]) or bare identifier.
const identifierPattern = `"[^"]+"|` + "`[^`]+`" + `|\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_]*`

var tableConstraintPrefix = regexp.MustCompile(`(?i)^(PRIMARY\s+KEY|FOREIGN\s+KEY|CONSTRAINT|UNIQUE|CHECK|KEY|INDEX)\b`)
var primaryKeyPrefix = regexp.MustCompile(`(?i)^PRIMARY\s+KEY`)
var foreignKeyAnyPattern = regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b`)
var notNullPattern = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
var inlinePKPattern = regexp.MustCompile(`(?i)\bPRIMARY\s+KEY\b`)
var identifierRe = regexp.MustCompile(identifierPattern)
var fkTargetPattern = regexp.MustCompile(`(?i)\bREFERENCES\s+(` + identifierPattern + `)`)

func parseColumns(body string) ([]Column, error) {
	defs := splitTopLevel(body, ',')
	var columns []Column
	pkNames := map[string]bool{}
	fkByCol := map[string]string{}
	for _, def := range defs {
		def = strings.TrimSpace(def)
		if def == "" {
			continue
		}
		if tableConstraintPrefix.MatchString(def) {
			// Covers bare and CONSTRAINT-named PRIMARY/FOREIGN KEY clauses.
			if primaryKeyPrefix.MatchString(def) || inlinePKPattern.MatchString(def) {
				for _, col := range extractParenIdentifiers(def) {
					pkNames[strings.ToLower(col)] = true
				}
			} else if foreignKeyAnyPattern.MatchString(def) {
				// FOREIGN KEY (local) REFERENCES tbl(col) — single-column only.
				local := extractParenIdentifiers(def)
				if len(local) > 0 {
					if m := fkTargetPattern.FindStringSubmatch(def); len(m) > 1 {
						fkByCol[strings.ToLower(local[0])] = unquoteIdentifier(m[1])
					}
				}
			}
			continue
		}
		col, err := parseColumnDef(def)
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}
	for i := range columns {
		if pkNames[strings.ToLower(columns[i].Name)] {
			columns[i].IsPrimaryKey = true
			columns[i].Nullable = false
		}
		if ref := fkByCol[strings.ToLower(columns[i].Name)]; ref != "" {
			columns[i].ForeignKey = ref
		}
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("no columns found")
	}
	return columns, nil
}

// parseColumnDef parses one column entry such as `"Name" VARCHAR(255) NOT NULL`.
func parseColumnDef(def string) (Column, error) {
	nameLoc := identifierRe.FindStringIndex(def)
	if nameLoc == nil {
		return Column{}, fmt.Errorf("could not parse column definition %q", def)
	}
	name := unquoteIdentifier(def[nameLoc[0]:nameLoc[1]])
	remainder := strings.TrimSpace(def[nameLoc[1]:])
	rawType, tail := splitTypeAndConstraints(remainder)
	col := Column{Name: name, RawType: rawType, Nullable: true}
	if notNullPattern.MatchString(tail) || inlinePKPattern.MatchString(tail) {
		col.Nullable = false
	}
	if inlinePKPattern.MatchString(tail) {
		col.IsPrimaryKey = true
	}
	if m := fkTargetPattern.FindStringSubmatch(tail); len(m) > 1 {
		col.ForeignKey = unquoteIdentifier(m[1])
	}
	return col, nil
}

// splitTypeAndConstraints separates the leading type token (optionally
// followed by a parenthesized precision/length, e.g. "decimal(18,2)") from
// the trailing constraint clauses (NOT NULL, PRIMARY KEY, DEFAULT ...).
func splitTypeAndConstraints(s string) (rawType, tail string) {
	i := 0
	for i < len(s) && !isSpace(s[i]) && s[i] != '(' {
		i++
	}
	typeEnd := i
	if i < len(s) && s[i] == '(' {
		if end, err := matchingParen(s, i); err == nil {
			typeEnd = end + 1
		} else {
			typeEnd = len(s)
		}
	}
	return strings.TrimSpace(s[:typeEnd]), strings.TrimSpace(s[typeEnd:])
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// extractParenIdentifiers pulls the comma-separated column names out of a
// table-level constraint's parenthesized list, e.g. "PRIMARY KEY (A, B)".
func extractParenIdentifiers(def string) []string {
	open := strings.IndexByte(def, '(')
	if open < 0 {
		return nil
	}
	end, err := matchingParen(def, open)
	if err != nil {
		return nil
	}
	var names []string
	for _, part := range splitTopLevel(def[open+1:end], ',') {
		part = strings.TrimSpace(part)
		if loc := identifierRe.FindStringIndex(part); loc != nil {
			names = append(names, unquoteIdentifier(part[loc[0]:loc[1]]))
		}
	}
	return names
}

// matchingParen returns the index of the ')' matching the '(' at openIdx,
// ignoring parens inside single-quoted string literals.
func matchingParen(s string, openIdx int) (int, error) {
	depth := 0
	inString := false
	for i := openIdx; i < len(s); i++ {
		switch {
		case s[i] == '\'' && !inString:
			inString = true
		case s[i] == '\'' && inString:
			inString = false
		case inString:
			// skip characters inside string literals
		case s[i] == '(':
			depth++
		case s[i] == ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unbalanced parentheses")
}

// splitTopLevel splits s on sep, ignoring separators nested inside
// parentheses or single-quoted string literals.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	inString := false
	last := 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\'' && !inString:
			inString = true
		case s[i] == '\'' && inString:
			inString = false
		case inString:
		case s[i] == '(':
			depth++
		case s[i] == ')':
			depth--
		case s[i] == sep && depth == 0:
			parts = append(parts, s[last:i])
			last = i + 1
		}
	}
	parts = append(parts, s[last:])
	return parts
}

func unquoteIdentifier(s string) string {
	if len(s) >= 2 {
		switch {
		case s[0] == '"' && s[len(s)-1] == '"':
			return s[1 : len(s)-1]
		case s[0] == '`' && s[len(s)-1] == '`':
			return s[1 : len(s)-1]
		case s[0] == '[' && s[len(s)-1] == ']':
			return s[1 : len(s)-1]
		}
	}
	return s
}

var commentBlock = regexp.MustCompile(`(?s)/\*.*?\*/`)
var lineComment = regexp.MustCompile(`--[^\n]*`)

func stripComments(s string) string {
	s = commentBlock.ReplaceAllString(s, " ")
	s = lineComment.ReplaceAllString(s, "")
	return s
}
