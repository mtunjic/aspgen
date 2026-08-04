// Package dbschema discovers table/column schema from a static SQL DDL
// script, for aspgen's import-db command.
package dbschema

// Column describes one column of an introspected or parsed table. IsPrimaryKey
// covers both single-column and composite primary keys; callers decide what
// to do with primary-key/reserved columns (dbschema only reports facts).
type Column struct {
	Name         string
	RawType      string
	Nullable     bool
	IsPrimaryKey bool
}

// Table describes one table's columns, in declaration/introspection order.
type Table struct {
	Name    string
	Columns []Column
}

// Providers supported by script parsing.
const (
	SQLite    = "sqlite"
	Postgres  = "postgres"
	SQLServer = "sqlserver"
	MySQL     = "mysql"
)

// Providers lists the supported provider names, for validation/help text.
var Providers = []string{SQLite, Postgres, SQLServer, MySQL}
