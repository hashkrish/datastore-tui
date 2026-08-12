package panes

import (
	"testing"

	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

func TestBuildRows_Entity_SortedByName(t *testing.T) {
	rows, err := BuildRows(model.Value{Kind: model.KindEntity, EntityValue: fixtureEntity()})
	if err != nil {
		t.Fatalf("BuildRows: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("len(rows) = %d, want 5", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Segment.Name > rows[i].Segment.Name {
			t.Fatalf("rows not sorted: %q before %q", rows[i-1].Segment.Name, rows[i].Segment.Name)
		}
	}
}

func TestBuildRows_Entity_ContainerFlags(t *testing.T) {
	rows, err := BuildRows(model.Value{Kind: model.KindEntity, EntityValue: fixtureEntity()})
	if err != nil {
		t.Fatalf("BuildRows: %v", err)
	}
	byName := map[string]DetailRow{}
	for _, r := range rows {
		byName[r.Segment.Name] = r
	}
	if !byName["tags"].IsContainer || byName["tags"].Kind != model.KindArray {
		t.Fatalf("tags row = %+v, want IsContainer array", byName["tags"])
	}
	if !byName["address"].IsContainer || byName["address"].Kind != model.KindEntity {
		t.Fatalf("address row = %+v, want IsContainer entity", byName["address"])
	}
	if byName["name"].IsContainer {
		t.Fatalf("name row = %+v, want a scalar leaf", byName["name"])
	}
	if byName["name"].Preview != `"Alice"` {
		t.Fatalf("name preview = %q, want quoted Alice", byName["name"].Preview)
	}
}

func TestBuildRows_Array(t *testing.T) {
	rows, err := BuildRows(model.Value{Kind: model.KindArray, ArrayValue: []model.Value{
		model.StringValue("a"), model.IntegerValue(1),
	}})
	if err != nil {
		t.Fatalf("BuildRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if !rows[0].Segment.IsIndex || rows[0].Segment.Index != 0 {
		t.Fatalf("rows[0].Segment = %+v, want index 0", rows[0].Segment)
	}
	if !rows[1].Segment.IsIndex || rows[1].Segment.Index != 1 {
		t.Fatalf("rows[1].Segment = %+v, want index 1", rows[1].Segment)
	}
}

func TestBuildRows_LeafErrors(t *testing.T) {
	if _, err := BuildRows(model.StringValue("x")); err == nil {
		t.Fatalf("BuildRows on a scalar leaf succeeded, want an error")
	}
}

func TestBuildRows_EmptyEntity(t *testing.T) {
	rows, err := BuildRows(model.Value{Kind: model.KindEntity, EntityValue: &model.Entity{}})
	if err != nil {
		t.Fatalf("BuildRows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("len(rows) = %d, want 0", len(rows))
	}
}

func TestLeafRowValue(t *testing.T) {
	scope := model.Value{Kind: model.KindEntity, EntityValue: fixtureEntity()}
	rows, err := BuildRows(scope)
	if err != nil {
		t.Fatalf("BuildRows: %v", err)
	}
	var nameRow DetailRow
	for _, r := range rows {
		if r.Segment.Name == "name" {
			nameRow = r
		}
	}
	v, err := LeafRowValue(scope, nameRow)
	if err != nil {
		t.Fatalf("LeafRowValue: %v", err)
	}
	if v.StringValue != "Alice" {
		t.Fatalf("LeafRowValue = %+v, want StringValue Alice", v)
	}
}

func TestFormatBreadcrumb(t *testing.T) {
	key := &model.Key{Path: []model.PathElement{{Kind: "Person", Name: "alice"}}}
	var path nav.DetailPath
	if got, want := FormatBreadcrumb("[default]", key, &path), "[default]/Person/alice"; got != want {
		t.Fatalf("FormatBreadcrumb = %q, want %q", got, want)
	}

	path.Push(nav.PropSegment{Name: "address"})
	path.Push(nav.PropSegment{Name: "tags"})
	path.Push(nav.PropSegment{IsIndex: true, Index: 2})
	want := "[default]/Person/alice/address/tags[2]"
	if got := FormatBreadcrumb("[default]", key, &path); got != want {
		t.Fatalf("FormatBreadcrumb (nested) = %q, want %q", got, want)
	}
}
