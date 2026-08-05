package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

// databaseSeederPath returns the generated DatabaseSeeder.cs location for the
// given seed backend ("" -> simple webapi, "ddd" -> DDD webapi, "ddd-local"
// -> local DDD wpf).
func databaseSeederPath(project, seedBackend string) string {
	switch seedBackend {
	case "ddd":
		return filepath.Join(project, "src", "WebApi", "Seeding", "DatabaseSeeder.cs")
	case "ddd-local":
		return filepath.Join(project, "src", "Infrastructure", "Seeding", "DatabaseSeeder.cs")
	default:
		return filepath.Join(project, "src", "WebApi", "Data", "DatabaseSeeder.cs")
	}
}

func renderSeedBlock(backend, entity string, properties []Property, count int) string {
	var b strings.Builder
	if backend == "ddd-local" {
		fmt.Fprintf(&b, "        if (!db.%ss.Any())\n        {\n", entity)
		fmt.Fprintf(&b, "            db.%ss.AddRange(\n", entity)
		for row := 0; row < count; row++ {
			fmt.Fprintf(&b, "                new %s(", entity)
			for i, property := range properties {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(seedValueFor(property, row, count))
			}
			b.WriteString(")")
			if row < count-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("            );\n        }\n")
		return b.String()
	}
	fmt.Fprintf(&b, "        if (!await db.%ss.AnyAsync(cancellationToken))\n        {\n", entity)
	fmt.Fprintf(&b, "            db.%ss.AddRange(\n", entity)
	for row := 0; row < count; row++ {
		fmt.Fprintf(&b, "                new %s", entity)
		if backend == "ddd" {
			b.WriteString("(")
		} else {
			b.WriteString(" {")
		}
		for i, property := range properties {
			if i > 0 {
				b.WriteString(",")
			}
			value := seedValueFor(property, row, count)
			if backend == "ddd" {
				b.WriteString(" ")
				b.WriteString(value)
			} else {
				fmt.Fprintf(&b, " %s = %s", property.Name, value)
			}
		}
		if backend == "ddd" {
			b.WriteString(")")
		} else {
			b.WriteString(" }")
		}
		if row < count-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("            );\n        }\n")
	return b.String()
}

// seedValueFor returns the seed literal for property at row. A relation's
// foreign key cycles through the already-seeded target rows (1..count)
// instead of using the generic numeric literal, which could reference a row
// that was never seeded.
func seedValueFor(property Property, row, count int) string {
	if property.RelationTarget != "" {
		return fmt.Sprintf("%dL", (row%count)+1)
	}
	return seedLiteral(property.CSharpType, property.Name, row)
}

func seedLiteral(csharpType, property string, row int) string {
	info, ok := csharpTypes[strings.TrimSuffix(csharpType, "?")]
	if !ok {
		return "default!"
	}
	return info.Seed(property, row)
}
