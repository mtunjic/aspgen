package dbschema

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"
)

var driverNames = map[string]string{
	SQLite:    "sqlite",
	Postgres:  "pgx",
	SQLServer: "sqlserver",
	MySQL:     "mysql",
}

// Introspect connects to the given database and returns schema for the
// requested tables. tableFilter is treated as an explicit allow-list: every
// requested name is checked against the database's real table list before
// being used in any further query, and an unknown name is a hard error —
// this keeps a caller-supplied --tables value from being used to probe or
// inject into catalog queries. A nil/empty tableFilter means "all tables".
func Introspect(provider, connection string, tableFilter []string) ([]Table, error) {
	driverName, ok := driverNames[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
	db, err := sql.Open(driverName, connection)
	if err != nil {
		return nil, fmt.Errorf("open %s connection: %w", provider, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connect to %s database: %w", provider, err)
	}
	allTables, err := listTables(db, provider)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	selected, err := allowListTables(allTables, tableFilter)
	if err != nil {
		return nil, err
	}
	tables := make([]Table, 0, len(selected))
	for _, name := range selected {
		columns, err := listColumns(db, provider, name)
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", name, err)
		}
		tables = append(tables, Table{Name: name, Columns: columns})
	}
	return tables, nil
}

// allowListTables validates requested against available (case-sensitive)
// and returns available unchanged, sorted, when requested is empty ("all").
func allowListTables(available, requested []string) ([]string, error) {
	if len(requested) == 0 {
		sorted := append([]string(nil), available...)
		sort.Strings(sorted)
		return sorted, nil
	}
	known := map[string]bool{}
	for _, t := range available {
		known[t] = true
	}
	for _, want := range requested {
		if !known[want] {
			return nil, fmt.Errorf("table %q not found in database", want)
		}
	}
	return requested, nil
}

func listTables(db *sql.DB, provider string) ([]string, error) {
	query, ok := listTablesQueries[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

var listTablesQueries = map[string]string{
	SQLite:    `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`,
	Postgres:  `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`,
	SQLServer: `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`,
	MySQL:     `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME`,
}

// safeIdentifier is only used for sqlite's PRAGMA table_info(<name>), which
// (unlike the information_schema queries used by the other three providers)
// cannot take a bound parameter for the table name.
var safeIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func listColumns(db *sql.DB, provider, table string) ([]Column, error) {
	if provider == SQLite {
		return listSQLiteColumns(db, table)
	}
	query, ok := listColumnsQueries[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
	rows, err := db.Query(query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []Column
	for rows.Next() {
		var name, rawType string
		var nullableRaw, pkRaw any
		if err := rows.Scan(&name, &rawType, &nullableRaw, &pkRaw); err != nil {
			return nil, err
		}
		columns = append(columns, Column{
			Name:         name,
			RawType:      rawType,
			Nullable:     truthy(nullableRaw),
			IsPrimaryKey: truthy(pkRaw),
		})
	}
	return columns, rows.Err()
}

func listSQLiteColumns(db *sql.DB, table string) ([]Column, error) {
	if !safeIdentifier.MatchString(table) {
		return nil, fmt.Errorf("unsupported table name %q", table)
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []Column
	for rows.Next() {
		var cid int
		var name, rawType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &rawType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		// SQLite reports notnull=0 for INTEGER PRIMARY KEY (rowid alias)
		// columns even though they can never hold NULL; pk always wins.
		nullable := notNull == 0 && pk == 0
		columns = append(columns, Column{
			Name:         name,
			RawType:      rawType,
			Nullable:     nullable,
			IsPrimaryKey: pk != 0,
		})
	}
	return columns, rows.Err()
}

// listColumnsQueries use information_schema (or MySQL's near-identical
// variant), parameterized by table name — no string interpolation needed
// since these are ordinary WHERE-clause values, not identifiers.
var listColumnsQueries = map[string]string{
	Postgres: `
SELECT c.column_name, c.data_type, (c.is_nullable = 'YES'),
       EXISTS (
         SELECT 1 FROM information_schema.table_constraints tc
         JOIN information_schema.key_column_usage kcu
           ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
         WHERE tc.constraint_type = 'PRIMARY KEY'
           AND kcu.table_name = c.table_name
           AND kcu.column_name = c.column_name
       )
FROM information_schema.columns c
WHERE c.table_name = $1
ORDER BY c.ordinal_position`,
	SQLServer: `
SELECT c.COLUMN_NAME, c.DATA_TYPE, IIF(c.IS_NULLABLE = 'YES', 1, 0), IIF(pk.COLUMN_NAME IS NOT NULL, 1, 0)
FROM INFORMATION_SCHEMA.COLUMNS c
LEFT JOIN (
    SELECT ku.TABLE_NAME, ku.COLUMN_NAME
    FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
    JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
      ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = ku.TABLE_SCHEMA
    WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
) pk ON pk.TABLE_NAME = c.TABLE_NAME AND pk.COLUMN_NAME = c.COLUMN_NAME
WHERE c.TABLE_NAME = ?
ORDER BY c.ORDINAL_POSITION`,
	MySQL: `
SELECT c.COLUMN_NAME, c.COLUMN_TYPE, (c.IS_NULLABLE = 'YES'), (c.COLUMN_KEY = 'PRI')
FROM INFORMATION_SCHEMA.COLUMNS c
WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = ?
ORDER BY c.ORDINAL_POSITION`,
}

// truthy normalizes the many wire representations drivers use for boolean-ish
// expressions (bool, int64, []byte, string) into a Go bool.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case []byte:
		s := strings.TrimSpace(string(t))
		return s == "1" || strings.EqualFold(s, "true") || strings.EqualFold(s, "yes")
	case string:
		s := strings.TrimSpace(t)
		return s == "1" || strings.EqualFold(s, "true") || strings.EqualFold(s, "yes")
	default:
		return false
	}
}
