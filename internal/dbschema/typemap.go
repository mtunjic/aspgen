package dbschema

import "strings"

// MapColumnType maps a provider-specific raw SQL type name (as reported by
// PRAGMA table_info / information_schema.columns, or parsed from a CREATE
// TABLE script) to the alias accepted by aspgen's `name:type` property
// syntax (string, int, long, decimal, float, bool, date, datetime, guid).
// ok is false for types with no known mapping (e.g. json, blob, xml,
// arrays) — callers should skip such columns rather than fail the whole
// table.
func MapColumnType(provider, rawType string) (string, bool) {
	switch provider {
	case SQLite:
		alias, ok := sqliteTypes[normalizeRawType(rawType)]
		return alias, ok
	case Postgres:
		alias, ok := postgresTypes[normalizeRawType(rawType)]
		return alias, ok
	case SQLServer:
		alias, ok := sqlserverTypes[normalizeRawType(rawType)]
		return alias, ok
	case MySQL:
		return mapMySQLType(rawType)
	default:
		return "", false
	}
}

// normalizeRawType strips length/precision annotations (e.g. "varchar(255)"
// -> "varchar"), collapses whitespace, and lower-cases the type name so it
// matches the lookup tables below.
func normalizeRawType(rawType string) string {
	t := strings.ToLower(strings.TrimSpace(rawType))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	return strings.Join(strings.Fields(t), " ")
}

// sqliteTypes covers common declared-type affinities; sqlite is dynamically
// typed so this is necessarily a best-effort mapping of whatever the CREATE
// TABLE/PRAGMA declared.
var sqliteTypes = map[string]string{
	"text": "string", "varchar": "string", "nvarchar": "string", "char": "string",
	"nchar": "string", "clob": "string",
	"integer": "long", "int": "long", "bigint": "long",
	"smallint": "int", "tinyint": "int", "mediumint": "int",
	"real": "float", "double": "float", "double precision": "float", "float": "float",
	"numeric": "decimal", "decimal": "decimal",
	"boolean": "bool", "bool": "bool",
	"date": "date", "datetime": "datetime", "timestamp": "datetime",
	"guid": "guid", "uuid": "guid",
}

// postgresTypes keys are information_schema.columns.data_type values.
var postgresTypes = map[string]string{
	"character varying": "string", "character": "string", "text": "string",
	"varchar": "string", "char": "string",
	"integer": "int", "smallint": "int",
	"bigint":  "long",
	"numeric": "decimal", "decimal": "decimal", "money": "decimal",
	"real": "float", "double precision": "float",
	"boolean":                     "bool",
	"date":                        "date",
	"timestamp without time zone": "datetime", "timestamp with time zone": "datetime",
	"uuid": "guid",
}

// sqlserverTypes keys are INFORMATION_SCHEMA.COLUMNS.DATA_TYPE values.
var sqlserverTypes = map[string]string{
	"varchar": "string", "nvarchar": "string", "char": "string", "nchar": "string",
	"text": "string", "ntext": "string",
	"int": "int", "smallint": "int", "tinyint": "int",
	"bigint":  "long",
	"decimal": "decimal", "numeric": "decimal", "money": "decimal", "smallmoney": "decimal",
	"float": "float", "real": "float",
	"bit":      "bool",
	"date":     "date",
	"datetime": "datetime", "datetime2": "datetime", "smalldatetime": "datetime",
	"uniqueidentifier": "guid",
}

// mysqlTypes keys are INFORMATION_SCHEMA.COLUMNS.DATA_TYPE values (without
// the length/unsigned suffix mysqlColumnType strips before lookup).
var mysqlTypes = map[string]string{
	"varchar": "string", "char": "string",
	"text": "string", "tinytext": "string", "mediumtext": "string", "longtext": "string",
	"int": "int", "smallint": "int", "mediumint": "int",
	"bigint":  "long",
	"decimal": "decimal", "numeric": "decimal",
	"float": "float", "double": "float",
	"date":     "date",
	"datetime": "datetime", "timestamp": "datetime",
}

// mapMySQLType special-cases MySQL's tinyint(1)-as-boolean convention, which
// requires the length annotation that a plain normalizeRawType lookup
// discards; every other type falls back to the generic mysqlTypes table.
func mapMySQLType(rawType string) (string, bool) {
	if normalizeRawType(rawType) == "tinyint" {
		if strings.Contains(rawType, "(1)") {
			return "bool", true
		}
		return "int", true
	}
	alias, ok := mysqlTypes[normalizeRawType(rawType)]
	return alias, ok
}
