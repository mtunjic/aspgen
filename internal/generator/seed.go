package generator

import (
	"fmt"
	"strings"
)

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
				b.WriteString(seedLiteral(property.CSharpType, property.Name, row))
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
			value := seedLiteral(property.CSharpType, property.Name, row)
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

func seedLiteral(csharpType, property string, row int) string {
	info, ok := csharpTypes[strings.TrimSuffix(csharpType, "?")]
	if !ok {
		return "default!"
	}
	return info.Seed(property, row)
}
