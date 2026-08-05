package dbschema

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseScript(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		want   []Table
		errMsg string
	}{
		{
			name: "simple sqlite table with inline primary key",
			sql: `-- customers table
CREATE TABLE Customers (
    Id INTEGER PRIMARY KEY,
    Name VARCHAR(255) NOT NULL,
    Age INT NULL
);`,
			want: []Table{{
				Name: "Customers",
				Columns: []Column{
					{Name: "Id", RawType: "INTEGER", Nullable: false, IsPrimaryKey: true},
					{Name: "Name", RawType: "VARCHAR(255)", Nullable: false},
					{Name: "Age", RawType: "INT", Nullable: true},
				},
			}},
		},
		{
			name: "table-level primary key constraint and quoted identifiers",
			sql: `CREATE TABLE "Orders" (
    "OrderId" BIGINT NOT NULL,
    "Total" DECIMAL(18,2) NOT NULL,
    PRIMARY KEY ("OrderId")
);`,
			want: []Table{{
				Name: "Orders",
				Columns: []Column{
					{Name: "OrderId", RawType: "BIGINT", Nullable: false, IsPrimaryKey: true},
					{Name: "Total", RawType: "DECIMAL(18,2)", Nullable: false},
				},
			}},
		},
		{
			name: "multiple tables and bracket-quoted identifiers ignoring FK/comments",
			sql: `/* schema */
CREATE TABLE [Products] (
    [Id] INT PRIMARY KEY,
    [CategoryId] INT NOT NULL,
    FOREIGN KEY ([CategoryId]) REFERENCES [Categories]([Id])
);
CREATE TABLE Categories (
    Id INT PRIMARY KEY,
    Name TEXT
);`,
			want: []Table{
				{Name: "Products", Columns: []Column{
					{Name: "Id", RawType: "INT", Nullable: false, IsPrimaryKey: true},
					{Name: "CategoryId", RawType: "INT", Nullable: false},
				}},
				{Name: "Categories", Columns: []Column{
					{Name: "Id", RawType: "INT", Nullable: false, IsPrimaryKey: true},
					{Name: "Name", RawType: "TEXT", Nullable: true},
				}},
			},
		},
		{
			name:   "no CREATE TABLE statements",
			sql:    `SELECT 1;`,
			errMsg: "no CREATE TABLE statements found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScript(SQLite, tt.sql)
			if tt.errMsg != "" {
				if err == nil || err.Error() != tt.errMsg {
					t.Fatalf("error = %v, want %q", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseScript() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseScript() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMapColumnType(t *testing.T) {
	tests := []struct {
		provider string
		rawType  string
		want     string
		wantOK   bool
	}{
		{SQLite, "VARCHAR(255)", "string", true},
		{SQLite, "INTEGER", "long", true},
		{SQLite, "BLOB", "", false},
		{Postgres, "character varying", "string", true},
		{Postgres, "timestamp without time zone", "datetime", true},
		{Postgres, "jsonb", "", false},
		{SQLServer, "uniqueidentifier", "guid", true},
		{SQLServer, "nvarchar", "string", true},
		{MySQL, "tinyint(1)", "bool", true},
		{MySQL, "tinyint(4)", "int", true},
		{MySQL, "varchar", "string", true},
		{"unknown", "int", "", false},
	}
	for _, tt := range tests {
		got, ok := MapColumnType(tt.provider, tt.rawType)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("MapColumnType(%q, %q) = (%q, %v), want (%q, %v)", tt.provider, tt.rawType, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestRenderSchemaSQL(t *testing.T) {
	tables := []Table{{
		Name: "Customers",
		Columns: []Column{
			{Name: "Id", RawType: "INTEGER", IsPrimaryKey: true},
			{Name: "Name", RawType: "VARCHAR(255)", Nullable: true},
		},
	}}
	got := RenderSchemaSQL(tables)
	if got == "" {
		t.Fatal("RenderSchemaSQL() returned empty string")
	}
	for _, want := range []string{"CREATE TABLE Customers", "Id INTEGER NOT NULL PRIMARY KEY,", "Name VARCHAR(255) NULL"} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderSchemaSQL() missing %q; got:\n%s", want, got)
		}
	}
}
