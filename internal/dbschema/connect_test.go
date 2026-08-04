package dbschema

import (
	"database/sql"
	"sort"
	"testing"

	_ "modernc.org/sqlite"
)

func TestIntrospectSQLite(t *testing.T) {
	ddl := []string{
		`CREATE TABLE Customers (Id INTEGER PRIMARY KEY, Name TEXT NOT NULL, Age INTEGER)`,
		`CREATE TABLE Orders (Id INTEGER PRIMARY KEY, Total REAL NOT NULL)`,
	}

	// Introspect opens its own connection, so use a shared in-memory DSN
	// that keeps the database alive across connections for this test.
	const dsn = "file::memory:?cache=shared"
	setup, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open shared: %v", err)
	}
	defer setup.Close()
	for _, stmt := range ddl {
		if _, err := setup.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	tables, err := Introspect(SQLite, dsn, nil)
	if err != nil {
		t.Fatalf("Introspect() error = %v", err)
	}
	names := make([]string, len(tables))
	for i, tbl := range tables {
		names[i] = tbl.Name
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "Customers" || names[1] != "Orders" {
		t.Fatalf("unexpected tables: %v", names)
	}

	filtered, err := Introspect(SQLite, dsn, []string{"Customers"})
	if err != nil {
		t.Fatalf("Introspect(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "Customers" {
		t.Fatalf("unexpected filtered tables: %+v", filtered)
	}
	cols := filtered[0].Columns
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d: %+v", len(cols), cols)
	}
	if cols[0].Name != "Id" || !cols[0].IsPrimaryKey || cols[0].Nullable {
		t.Errorf("Id column = %+v, want primary key non-nullable", cols[0])
	}
	if cols[1].Name != "Name" || cols[1].Nullable {
		t.Errorf("Name column = %+v, want non-nullable", cols[1])
	}
	if cols[2].Name != "Age" || !cols[2].Nullable {
		t.Errorf("Age column = %+v, want nullable", cols[2])
	}

	if _, err := Introspect(SQLite, dsn, []string{"NoSuchTable"}); err == nil {
		t.Fatal("expected error for unknown requested table")
	}
}
