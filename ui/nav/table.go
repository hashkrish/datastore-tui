package nav

import (
	"sort"
	"strings"

	"github.com/krishnan/datastore-tui/datastore/model"
)

// TableState holds the spreadsheet-grid view's cursor/scroll position. It is
// rebuilt (columns re-derived) every time the table screen is entered, since
// the entity list backing it may have changed since the last visit (a query
// filter, an order, a refresh).
type TableState struct {
	Columns      []string // derived property-name union; the "Key" column is a separate pinned synthetic column, not included here
	ColumnFilter string   // "*": substring filter over Columns, by name — see VisibleColumns
	Row, Col     int      // selected cell: Row indexes the entity slice, Col indexes VisibleColumns()
	RowOff       int      // first visible entity row (vertical scroll)
	ColOff       int      // first visible column (within VisibleColumns()) (horizontal scroll)
	SearchTerm   string   // "/": last cell-content search, so "n"/"N" can repeat it without re-prompting
}

// VisibleColumns returns Columns filtered by ColumnFilter (case-insensitive
// substring match on the column/property name), or all of Columns when no
// filter is set. Col/ColOff index into this slice, not the raw Columns —
// mirroring how columnList.visible() is what a Miller column's selection
// indexes into, not its unfiltered items.
func (ts *TableState) VisibleColumns() []string {
	if ts.ColumnFilter == "" {
		return ts.Columns
	}
	needle := strings.ToLower(ts.ColumnFilter)
	out := make([]string, 0, len(ts.Columns))
	for _, c := range ts.Columns {
		if strings.Contains(strings.ToLower(c), needle) {
			out = append(out, c)
		}
	}
	return out
}

// BuildTableColumns computes the union of property names across entities,
// for use as the table view's data columns (the synthetic "Key" column is
// handled separately by the renderer, not included here).
//
// Ordering: within each entity, names are sorted alphabetically before being
// folded into the running union, and an entity's names are folded in
// entity-list order (first entity's names first). This is deterministic
// (unlike raw map iteration, which Go randomizes) and, for the common case
// of homogeneous entities of one kind, is indistinguishable from a plain
// global alphabetical order. It only diverges for sparse/mixed-schema
// kinds, where it keeps each entity's own properties clustered together
// (the first entity's properties occupy the leftmost columns; any property
// introduced later by a different entity is appended after).
func BuildTableColumns(entities []*model.Entity) []string {
	seen := map[string]bool{}
	var cols []string
	for _, e := range entities {
		if e == nil {
			continue
		}
		names := make([]string, 0, len(e.Properties))
		for name := range e.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				cols = append(cols, name)
			}
		}
	}
	return cols
}
