package nav

import (
	"reflect"
	"testing"

	"github.com/krishnan/datastore-tui/datastore/model"
)

func entityWithProps(name string, props map[string]model.Value) *model.Entity {
	e := entityWithName(name)
	e.Properties = props
	return e
}

func TestBuildTableColumns_EmptySlice(t *testing.T) {
	cols := BuildTableColumns(nil)
	if len(cols) != 0 {
		t.Fatalf("BuildTableColumns(nil) = %v, want empty", cols)
	}
}

func TestBuildTableColumns_SingleEntity_Alphabetized(t *testing.T) {
	e := entityWithProps("alice", map[string]model.Value{
		"zeta":  {Kind: model.KindString, StringValue: "z"},
		"alpha": {Kind: model.KindString, StringValue: "a"},
	})
	cols := BuildTableColumns([]*model.Entity{e})
	want := []string{"alpha", "zeta"}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("BuildTableColumns = %v, want %v", cols, want)
	}
}

func TestBuildTableColumns_IdenticalPropertySets_Dedups(t *testing.T) {
	props := map[string]model.Value{
		"name": {Kind: model.KindString, StringValue: "a"},
		"age":  {Kind: model.KindInteger, IntegerValue: 1},
	}
	e1 := entityWithProps("alice", props)
	e2 := entityWithProps("bob", props)
	cols := BuildTableColumns([]*model.Entity{e1, e2})
	want := []string{"age", "name"}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("BuildTableColumns = %v, want %v", cols, want)
	}
}

func TestBuildTableColumns_SparseEntities_UnionInFirstSeenOrder(t *testing.T) {
	e1 := entityWithProps("alice", map[string]model.Value{
		"name": {Kind: model.KindString, StringValue: "alice"},
		"age":  {Kind: model.KindInteger, IntegerValue: 30},
	})
	e2 := entityWithProps("bob", map[string]model.Value{
		"name":  {Kind: model.KindString, StringValue: "bob"},
		"age":   {Kind: model.KindInteger, IntegerValue: 40},
		"email": {Kind: model.KindString, StringValue: "bob@example.com"},
	})
	cols := BuildTableColumns([]*model.Entity{e1, e2})
	want := []string{"age", "name", "email"}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("BuildTableColumns = %v, want %v", cols, want)
	}
}

func TestTableState_VisibleColumns_NoFilter(t *testing.T) {
	ts := &TableState{Columns: []string{"age", "email", "name"}}
	if got := ts.VisibleColumns(); !reflect.DeepEqual(got, ts.Columns) {
		t.Fatalf("VisibleColumns() = %v, want %v", got, ts.Columns)
	}
}

func TestTableState_VisibleColumns_FiltersCaseInsensitiveSubstring(t *testing.T) {
	ts := &TableState{Columns: []string{"age", "email", "name"}, ColumnFilter: "AM"}
	want := []string{"name"}
	if got := ts.VisibleColumns(); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisibleColumns() = %v, want %v", got, want)
	}
}

func TestTableState_VisibleColumns_NoMatches(t *testing.T) {
	ts := &TableState{Columns: []string{"age", "email", "name"}, ColumnFilter: "zzz"}
	if got := ts.VisibleColumns(); len(got) != 0 {
		t.Fatalf("VisibleColumns() = %v, want empty", got)
	}
}
