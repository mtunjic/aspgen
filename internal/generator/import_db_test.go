package generator

import (
	"reflect"
	"testing"

	"aspgen/internal/dbschema"
)

func TestRelationNavName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"customer_id", "customer"},
		{"customerId", "customer"},
		{"CustomerId", "Customer"},
		{"order_ref", "orderRef"},
		{"grid", "grid"},
		{"project", "project"},
	}
	for _, tt := range tests {
		if got := relationNavName(tt.in); got != tt.want {
			t.Errorf("relationNavName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSynthesizeProps(t *testing.T) {
	table := dbschema.Table{
		Name: "Orders",
		Columns: []dbschema.Column{
			{Name: "Id", RawType: "INTEGER", IsPrimaryKey: true},
			{Name: "CustomerId", RawType: "BIGINT", Nullable: false, ForeignKey: "Customers"},
			{Name: "ManagerId", RawType: "INTEGER", Nullable: true, ForeignKey: "Employees"},
			{Name: "Total", RawType: "DECIMAL(18,2)", Nullable: false},
			{Name: "CreatedOn", RawType: "DATETIME", Nullable: true},
		},
	}
	tests := []struct {
		name   string
		inSet  map[string]bool
		want   []string
		wantSk []string
	}{
		{
			name:   "referenced tables in set become relations",
			inSet:  map[string]bool{"customers": true, "employees": true},
			want:   []string{"Customer:Customer", "Manager:Employee?", "Total:decimal"},
			wantSk: nil,
		},
		{
			name:   "unselected referenced table falls back to scalar",
			inSet:  map[string]bool{"customers": true},
			want:   []string{"Customer:Customer", "ManagerId:long?", "Total:decimal"},
			wantSk: nil,
		},
		{
			name:   "no references at all",
			inSet:  map[string]bool{},
			want:   []string{"CustomerId:long", "ManagerId:long?", "Total:decimal"},
			wantSk: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skipped := synthesizeProps(table, dbschema.SQLite, "ddd", tt.inSet)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("synthesizeProps() args = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(skipped, tt.wantSk) {
				t.Errorf("synthesizeProps() skipped = %#v, want %#v", skipped, tt.wantSk)
			}
		})
	}
}

func TestSynthesizePropsJoinTableFKPK(t *testing.T) {
	table := dbschema.Table{
		Name: "PostTags",
		Columns: []dbschema.Column{
			{Name: "PostId", RawType: "INTEGER", IsPrimaryKey: true, ForeignKey: "Posts"},
			{Name: "TagId", RawType: "INTEGER", IsPrimaryKey: true, ForeignKey: "Tags"},
		},
	}
	got, _ := synthesizeProps(table, dbschema.SQLite, "ddd", map[string]bool{"posts": true, "tags": true})
	want := []string{"Post:Post", "Tag:Tag"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("synthesizeProps() = %#v, want %#v", got, want)
	}
}

func TestOrderTablesByDependency(t *testing.T) {
	tables := []dbschema.Table{
		{Name: "Orders", Columns: []dbschema.Column{{Name: "CustomerId", ForeignKey: "Customers"}}},
		{Name: "Customers"},
		{Name: "Products", Columns: []dbschema.Column{{Name: "CategoryId", ForeignKey: "Categories"}}},
		{Name: "Categories"},
	}
	got := orderTablesByDependency(tables)
	var names []string
	for _, tbl := range got {
		names = append(names, tbl.Name)
	}
	want := []string{"Customers", "Orders", "Categories", "Products"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("orderTablesByDependency() = %v, want %v", names, want)
	}
}

func TestOrderTablesByDependencyCycle(t *testing.T) {
	tables := []dbschema.Table{
		{Name: "A", Columns: []dbschema.Column{{Name: "BId", ForeignKey: "B"}}},
		{Name: "B", Columns: []dbschema.Column{{Name: "AId", ForeignKey: "A"}}},
		{Name: "C", Columns: []dbschema.Column{{Name: "BId", ForeignKey: "B"}}},
	}
	got := orderTablesByDependency(tables)
	if len(got) != 3 {
		t.Fatalf("orderTablesByDependency() len = %d, want 3", len(got))
	}
}
