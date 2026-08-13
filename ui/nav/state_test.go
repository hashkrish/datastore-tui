package nav

import (
	"testing"

	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

func entityWithName(name string) *model.Entity {
	return &model.Entity{Key: &model.Key{Path: []model.PathElement{{Kind: "Person", Name: name}}}}
}

func TestState_NewState_FocusedOnNamespace(t *testing.T) {
	s := NewState()
	if s.Focus != ColumnNamespace {
		t.Fatalf("Focus = %v, want ColumnNamespace", s.Focus)
	}
}

func TestState_SelectedNamespace_EmptyList(t *testing.T) {
	s := NewState()
	if _, ok := s.SelectedNamespace(); ok {
		t.Fatalf("SelectedNamespace() ok = true on an empty list, want false")
	}
}

func TestState_MoveBy_ClampsWithinBounds(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"a", "b", "c"})

	s.MoveBy(-1) // clamp at 0
	if idx := s.SelectedIndex(ColumnNamespace); idx != 0 {
		t.Fatalf("SelectedIndex = %d, want 0", idx)
	}

	s.MoveBy(10) // clamp at len-1
	if idx := s.SelectedIndex(ColumnNamespace); idx != 2 {
		t.Fatalf("SelectedIndex = %d, want 2", idx)
	}

	s.MoveBy(-1)
	if idx := s.SelectedIndex(ColumnNamespace); idx != 1 {
		t.Fatalf("SelectedIndex = %d, want 1", idx)
	}
}

func TestState_MoveToTopAndBottom(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"a", "b", "c"})
	s.MoveBy(1)

	s.MoveToBottom()
	if idx := s.SelectedIndex(ColumnNamespace); idx != 2 {
		t.Fatalf("after MoveToBottom, SelectedIndex = %d, want 2", idx)
	}
	s.MoveToTop()
	if idx := s.SelectedIndex(ColumnNamespace); idx != 0 {
		t.Fatalf("after MoveToTop, SelectedIndex = %d, want 0", idx)
	}
}

func TestState_MoveOperatesOnFocusedColumnOnly(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"a", "b"})
	s.SetKinds([]string{"x", "y", "z"})
	s.Focus = ColumnKind

	s.MoveBy(1)
	if idx := s.SelectedIndex(ColumnKind); idx != 1 {
		t.Fatalf("SelectedIndex(Kind) = %d, want 1", idx)
	}
	if idx := s.SelectedIndex(ColumnNamespace); idx != 0 {
		t.Fatalf("SelectedIndex(Namespace) = %d, want unchanged 0", idx)
	}
}

func TestState_FocusRightAndLeft_StopsAtEnds(t *testing.T) {
	s := NewState()
	if !s.FocusRight() || s.Focus != ColumnKind {
		t.Fatalf("FocusRight() from Namespace: Focus = %v, want ColumnKind", s.Focus)
	}
	if !s.FocusRight() || s.Focus != ColumnEntity {
		t.Fatalf("FocusRight() from Kind: Focus = %v, want ColumnEntity", s.Focus)
	}
	if s.FocusRight() {
		t.Fatalf("FocusRight() from Entity returned true, want false (already at the end)")
	}
	if s.Focus != ColumnEntity {
		t.Fatalf("Focus after a no-op FocusRight() = %v, want unchanged ColumnEntity", s.Focus)
	}

	if !s.FocusLeft() || s.Focus != ColumnKind {
		t.Fatalf("FocusLeft() from Entity: Focus = %v, want ColumnKind", s.Focus)
	}
	if !s.FocusLeft() || s.Focus != ColumnNamespace {
		t.Fatalf("FocusLeft() from Kind: Focus = %v, want ColumnNamespace", s.Focus)
	}
	if s.FocusLeft() {
		t.Fatalf("FocusLeft() from Namespace returned true, want false (already at the start)")
	}
}

func TestState_Filter_NarrowsVisibleItemsAndClampsSelection(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"alpha", "beta", "gamma"})
	s.MoveToBottom() // select "gamma", index 2

	s.SetFilter("a") // matches alpha, beta, gamma (all contain "a")
	if got := s.VisibleItems(ColumnNamespace); len(got) != 3 {
		t.Fatalf("VisibleItems = %v, want all 3 to match \"a\"", got)
	}

	s.SetFilter("beta")
	visible := s.VisibleItems(ColumnNamespace)
	if len(visible) != 1 || visible[0] != "beta" {
		t.Fatalf("VisibleItems = %v, want [beta]", visible)
	}
	// The selection index (2, from before filtering) now points past the
	// filtered list's end; SelectedNamespace must not return a stale item.
	if _, ok := s.SelectedNamespace(); ok {
		t.Fatalf("SelectedNamespace() ok = true with a stale out-of-range index, want false")
	}
}

func TestState_Filter_IsCaseInsensitive(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"Production", "staging"})
	s.SetFilter("PROD")
	visible := s.VisibleItems(ColumnNamespace)
	if len(visible) != 1 || visible[0] != "Production" {
		t.Fatalf("VisibleItems = %v, want [Production]", visible)
	}
}

func TestState_SetKinds_ResetsFilterAndSelection(t *testing.T) {
	s := NewState()
	s.SetNamespaces([]string{"a"})
	s.Focus = ColumnKind
	s.SetKinds([]string{"Kind1", "Kind2"})
	s.SetFilter("Kind2")
	s.MoveToTop()

	// Loading a fresh kind list (e.g. after the namespace selection
	// changes) must drop the old filter/selection, not carry it over.
	s.SetKinds([]string{"Other1", "Other2"})
	if got := s.Filter(); got != "" {
		t.Fatalf("Filter() after SetKinds = %q, want empty", got)
	}
	if got := s.VisibleItems(ColumnKind); len(got) != 2 {
		t.Fatalf("VisibleItems after SetKinds = %v, want both new items", got)
	}
}

func TestState_SelectedEntity(t *testing.T) {
	s := NewState()
	page := &client.QueryPage{Entities: []*model.Entity{entityWithName("alice"), entityWithName("bob")}}
	s.SetEntitiesPage(page, false)
	s.Focus = ColumnEntity

	e := s.SelectedEntity()
	if e == nil || e.Key.Last().Name != "alice" {
		t.Fatalf("SelectedEntity() = %v, want alice", e)
	}

	s.MoveBy(1)
	e = s.SelectedEntity()
	if e == nil || e.Key.Last().Name != "bob" {
		t.Fatalf("SelectedEntity() after move = %v, want bob", e)
	}
}

func TestState_SelectedEntity_NoneLoaded(t *testing.T) {
	s := NewState()
	if e := s.SelectedEntity(); e != nil {
		t.Fatalf("SelectedEntity() = %v, want nil when nothing is loaded", e)
	}
}

func TestState_SetEntitiesPage_Append(t *testing.T) {
	s := NewState()
	s.SetEntitiesPage(&client.QueryPage{
		Entities:  []*model.Entity{entityWithName("alice")},
		EndCursor: "cursor1",
		HasMore:   true,
	}, false)
	s.SetEntitiesPage(&client.QueryPage{
		Entities:  []*model.Entity{entityWithName("bob")},
		EndCursor: "cursor2",
		HasMore:   false,
	}, true)

	if got := s.VisibleItems(ColumnEntity); len(got) != 2 {
		t.Fatalf("VisibleItems = %v, want 2 entities after append", got)
	}
	if s.EntityCursor() != "cursor2" {
		t.Fatalf("EntityCursor() = %q, want cursor2 (latest page)", s.EntityCursor())
	}
	if s.EntityHasMore() {
		t.Fatalf("EntityHasMore() = true, want false (latest page said no)")
	}
}

func TestState_SetEntitiesPage_ReplaceDropsOldEntities(t *testing.T) {
	s := NewState()
	s.SetEntitiesPage(&client.QueryPage{Entities: []*model.Entity{entityWithName("alice")}}, false)
	s.SetEntitiesPage(&client.QueryPage{Entities: []*model.Entity{entityWithName("carol")}}, false)

	if got := s.VisibleItems(ColumnEntity); len(got) != 1 || got[0] != "carol" {
		t.Fatalf("VisibleItems = %v, want just [carol]", got)
	}
}
