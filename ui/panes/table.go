package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// keyColumnWidth and cellWidth are fixed for a predictable grid; a real
// spreadsheet would auto-size, but fixed widths keep this a straightforward
// hand-rolled renderer consistent with RenderBrowse/RenderDetail's approach.
const (
	keyColumnWidth = 24
	cellWidth      = 20
)

// VisibleTableColumns reports how many of a TableState's Columns fit in
// width once the pinned Key column is subtracted — shared by RenderTable
// (to know how much to render) and the caller's scroll-clamping (to know
// when ColOff needs to change to keep the selected column in view), so the
// column-width layout math lives in exactly one place.
func VisibleTableColumns(width int) int {
	n := (width - keyColumnWidth) / (cellWidth + 1)
	if n < 0 {
		return 0
	}
	return n
}

// RenderTable renders entities as a spreadsheet-like grid: one pinned Key
// column plus ts.VisibleColumns() as scrollable columns, one row per
// entity. ts.Row selects the highlighted row; ts.RowOff/ts.ColOff are the
// current scroll position, both already clamped by the caller before each
// render.
func RenderTable(entities []*model.Entity, ts *nav.TableState, width, height int) string {
	if len(entities) == 0 {
		return dimStyle.Render("  (no entities)")
	}
	if height < 1 {
		height = 1
	}
	cols := ts.VisibleColumns()

	var b strings.Builder
	b.WriteString(renderTableHeaderRow(ts, cols, width))
	b.WriteString("\n")

	visibleRows := height - 1 // header consumes one row
	if visibleRows < 0 {
		visibleRows = 0
	}
	end := ts.RowOff + visibleRows
	if end > len(entities) {
		end = len(entities)
	}
	for i := ts.RowOff; i < end; i++ {
		if i > ts.RowOff {
			b.WriteString("\n")
		}
		b.WriteString(renderTableRow(entities[i], ts, cols, i == ts.Row, width))
	}
	return b.String()
}

// padCell truncates s to width display columns (ANSI/rune-safe, via
// truncate) and then right-pads it with spaces back out to exactly width —
// truncate alone only clips overlong content, it never pads shorter
// content, which is what let each cell's width drift with its own value's
// length and threw off column alignment between rows.
func padCell(s string, width int) string {
	s = truncate(s, width)
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// renderTableHeaderRow renders the pinned "Key" header plus as many of
// cols[ts.ColOff:] as fit in width.
func renderTableHeaderRow(ts *nav.TableState, cols []string, width int) string {
	var b strings.Builder
	b.WriteString(focusedStyle.Render(padCell(" Key", keyColumnWidth)))
	n := VisibleTableColumns(width)
	end := ts.ColOff + n
	if end > len(cols) {
		end = len(cols)
	}
	for c := ts.ColOff; c < end; c++ {
		b.WriteString(" ")
		b.WriteString(focusedStyle.Render(padCell(cols[c], cellWidth)))
	}
	return b.String()
}

// renderTableRow renders one entity's Key cell plus its visible property
// cells, formatted via preview() (the same Value->string function detail.go
// uses), truncated and padded to a fixed width so columns stay aligned
// across rows regardless of each cell's content length. A property absent
// on this entity (sparse schema) renders as a dim placeholder rather than
// an empty gap, so it reads as "no value" rather than looking like a
// rendering glitch.
//
// Only the selected cell (isSelectedRow && its column == ts.Col) gets the
// full background highlight; the rest of a selected row just bolds its Key
// cell as a row indicator, so the highlighted background always marks
// exactly one cell instead of the whole row swallowing which column is
// selected.
func renderTableRow(e *model.Entity, ts *nav.TableState, cols []string, isSelectedRow bool, width int) string {
	var b strings.Builder
	keyCell := padCell(" "+entityLabelForTable(e), keyColumnWidth)
	if isSelectedRow {
		keyCell = focusedStyle.Render(keyCell)
	}
	b.WriteString(keyCell)

	n := VisibleTableColumns(width)
	end := ts.ColOff + n
	if end > len(cols) {
		end = len(cols)
	}
	for c := ts.ColOff; c < end; c++ {
		b.WriteString(" ")
		name := cols[c]
		v, present := e.Properties[name]

		var cell string
		if present {
			cell = padCell(preview(v), cellWidth)
		} else {
			cell = padCell("·", cellWidth)
		}

		switch {
		case isSelectedRow && c == ts.Col:
			cell = selectedRowStyle.Render(cell)
		case !present:
			cell = dimStyle.Render(cell)
		}
		b.WriteString(cell)
	}
	return b.String()
}

// PreviewValue renders v exactly as a table/detail cell displays it —
// exported so table search logic in app/table.go (outside this package) can
// match "/" search terms against the same text a cell shows, rather than
// re-deriving its own formatting.
func PreviewValue(v model.Value) string { return preview(v) }

// entityLabelForTable renders an Entity's Key-column label — its ID/Name,
// same as the browse Entity column's rows.
func entityLabelForTable(e *model.Entity) string {
	if e == nil || e.Key == nil {
		return ""
	}
	return e.Key.Last().IDOrName()
}
