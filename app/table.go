package app

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// openTableView implements "T" in browse mode: enters the spreadsheet-like
// table view over the entity column's currently loaded (and possibly
// query-filtered/ordered) entities. No new fetch is issued — SelectedEntities
// already reflects any active filter/order, which is what makes a filtered
// list tabulate correctly.
func (m *Model) openTableView() (tea.Model, tea.Cmd) {
	entities := m.nav.SelectedEntities()
	m.tableState = &nav.TableState{Columns: nav.BuildTableColumns(entities)}
	m.screen = screenTable
	return m, nil
}

func (m *Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := keymap.DefaultTableKeyMap()
	entities := m.nav.SelectedEntities()
	cols := m.tableState.VisibleColumns()

	switch {
	case key.Matches(msg, km.Quit):
		return m.requestQuit()

	case key.Matches(msg, km.Help):
		m.prevScreen = screenTable
		m.screen = screenHelp
		return m, nil

	case key.Matches(msg, km.Toggle), key.Matches(msg, km.Back):
		m.screen = screenBrowse
		m.tableState = nil
		return m, nil

	case key.Matches(msg, km.FilterColumns):
		m.chordY.Reset()
		m.filterInput.SetValue(m.tableState.ColumnFilter)
		m.filterInput.Focus()
		m.filterInput.CursorEnd()
		m.screen = screenTableColumnFilter
		return m, nil

	case key.Matches(msg, km.Search):
		m.chordY.Reset()
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		m.filterInput.CursorEnd()
		m.screen = screenTableSearch
		return m, nil

	case key.Matches(msg, km.Down):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Row = clampInt(m.tableState.Row+1, 0, len(entities)-1)

	case key.Matches(msg, km.Up):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Row = clampInt(m.tableState.Row-1, 0, len(entities)-1)

	case key.Matches(msg, km.Right):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Col = clampInt(m.tableState.Col+1, 0, len(cols)-1)

	case key.Matches(msg, km.Left):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Col = clampInt(m.tableState.Col-1, 0, len(cols)-1)

	case key.Matches(msg, km.HalfPageDown):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Row = clampInt(m.tableState.Row+m.halfPage(), 0, len(entities)-1)

	case key.Matches(msg, km.HalfPageUp):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Row = clampInt(m.tableState.Row-m.halfPage(), 0, len(entities)-1)

	case key.Matches(msg, km.Top):
		m.chordY.Reset()
		if m.chordG.Complete('g') {
			m.tableState.Row = 0
		} else {
			m.chordG.Arm('g')
		}

	case key.Matches(msg, km.Bottom):
		m.chordG.Reset()
		m.chordY.Reset()
		m.tableState.Row = len(entities) - 1

	case key.Matches(msg, km.Open):
		m.chordG.Reset()
		m.chordY.Reset()
		return m.openEntityDetailFromTable(entities, cols)

	case key.Matches(msg, km.OpenWeb):
		m.chordG.Reset()
		m.chordY.Reset()
		return m.webForEntity(m.selectedTableEntity(entities), m.openWeb)

	case key.Matches(msg, km.YankURL):
		m.chordG.Reset()
		if m.chordY.Complete('y') {
			return m.webForEntity(m.selectedTableEntity(entities), m.copyWebURL)
		}
		return m, nil

	case key.Matches(msg, km.Yank):
		m.chordG.Reset()
		if m.chordY.Complete('y') {
			return m.yankSelectedTableCell(entities, cols)
		}
		m.chordY.Arm('y')
		return m, nil

	case key.Matches(msg, km.NextMatch):
		m.chordG.Reset()
		m.chordY.Reset()
		return m.findTableMatch(m.tableState.SearchTerm, true)

	case key.Matches(msg, km.PrevMatch):
		m.chordG.Reset()
		m.chordY.Reset()
		return m.findTableMatch(m.tableState.SearchTerm, false)

	default:
		m.chordG.Reset()
		m.chordY.Reset()
		return m, nil
	}

	m.clampTableScroll(len(entities))
	return m, nil
}

// updateTableColumnFilter drives the "*" column-name filter input, following
// the same textinput.Model + screen-switch shape as updateFilterInput
// (app/browse.go) and updateDetailFilterInput (app/detail.go): "enter"
// commits the filter and resets the column selection (the previously
// selected column may no longer be visible, or may now sit at a different
// index), "esc" discards the edit and leaves the previously applied filter
// untouched.
func (m *Model) updateTableColumnFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.tableState.ColumnFilter = m.filterInput.Value()
		m.tableState.Col, m.tableState.ColOff = 0, 0
		m.screen = screenTable
		m.clampTableScroll(len(m.nav.SelectedEntities()))
		return m, nil
	case "esc":
		m.screen = screenTable
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

// updateTableSearch drives the "/" cell-content search input: "enter" runs
// the search and jumps the selection to the first match (see
// searchTableCells), "esc" cancels without searching.
func (m *Model) updateTableSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		term := m.filterInput.Value()
		m.screen = screenTable
		return m.searchTableCells(term)
	case "esc":
		m.screen = screenTable
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

// yankSelectedTableCell implements "yy" in table mode: copies the
// highlighted cell's raw value to the system clipboard. A no-op when the
// entity is missing that property (a sparse "·" cell — nothing to copy) or
// there are no columns to select at all.
func (m *Model) yankSelectedTableCell(entities []*model.Entity, cols []string) (tea.Model, tea.Cmd) {
	if m.tableState.Row < 0 || m.tableState.Row >= len(entities) {
		return m, nil
	}
	if m.tableState.Col < 0 || m.tableState.Col >= len(cols) {
		return m, nil
	}
	e := entities[m.tableState.Row]
	if e == nil {
		return m, nil
	}
	v, ok := e.Properties[cols[m.tableState.Col]]
	if !ok {
		return m, nil
	}
	if err := clipboard.WriteAll(yankText(v)); err != nil {
		m.err = err
		return m, nil
	}
	m.status = "copied value"
	return m, nil
}

// searchTableCells implements "/" in table mode: records term as the
// current search (so "n"/"N" can repeat it later without re-prompting) and
// jumps to its first match forward from the selected cell — see
// findTableMatch.
func (m *Model) searchTableCells(term string) (tea.Model, tea.Cmd) {
	m.tableState.SearchTerm = term
	return m.findTableMatch(term, true)
}

// findTableMatch implements "/"'s initial jump as well as "n"/"N" (repeating
// the last search forward/backward, vim-style): scans every entity's
// visible-column values (formatted via panes.PreviewValue, the same text a
// cell displays) in row-major order starting one cell past (forward) or
// before (backward) the currently selected cell, wrapping around, and jumps
// the selection to the first case-insensitive substring match. Leaves the
// selection untouched (with a "no match" status, or silently if term is
// empty — e.g. "n" pressed before any search has run) otherwise.
func (m *Model) findTableMatch(term string, forward bool) (tea.Model, tea.Cmd) {
	if term == "" {
		return m, nil
	}
	ts := m.tableState
	entities := m.nav.SelectedEntities()
	cols := ts.VisibleColumns()
	if len(entities) == 0 || len(cols) == 0 {
		m.status = "no match"
		return m, nil
	}

	needle := strings.ToLower(term)
	total := len(entities) * len(cols)
	cur := ts.Row*len(cols) + ts.Col
	for offset := 1; offset <= total; offset++ {
		step := offset
		if !forward {
			step = -offset
		}
		idx := ((cur+step)%total + total) % total
		row, col := idx/len(cols), idx%len(cols)
		e := entities[row]
		if e == nil {
			continue
		}
		v, ok := e.Properties[cols[col]]
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(panes.PreviewValue(v)), needle) {
			ts.Row, ts.Col = row, col
			m.clampTableScroll(len(entities))
			m.status = "match found"
			return m, nil
		}
	}
	m.status = "no match"
	return m, nil
}

// clampTableScroll adjusts tableState.RowOff/ColOff so the selected Row/Col
// stay within the currently visible window — the two-axis analogue of how
// browse's columnList keeps its cursor visible, made explicit here since the
// grid scrolls independently on two axes rather than one.
func (m *Model) clampTableScroll(entityCount int) {
	ts := m.tableState
	if ts == nil {
		return
	}
	if entityCount == 0 {
		ts.Row, ts.RowOff = 0, 0
	}

	visibleRows := (m.height - 1 /* status line */) - 1 /* header row */
	if visibleRows < 1 {
		visibleRows = 1
	}
	if ts.Row < ts.RowOff {
		ts.RowOff = ts.Row
	} else if ts.Row >= ts.RowOff+visibleRows {
		ts.RowOff = ts.Row - visibleRows + 1
	}

	visibleCols := panes.VisibleTableColumns(m.width)
	if visibleCols < 1 {
		visibleCols = 1
	}
	if ts.Col < ts.ColOff {
		ts.ColOff = ts.Col
	} else if ts.Col >= ts.ColOff+visibleCols {
		ts.ColOff = ts.Col - visibleCols + 1
	}
}

// openEntityDetailFromTable implements "enter" on a table-view cell: opens
// that row's entity in the detail view, mirroring drillIn()'s ColumnEntity
// branch in app/browse.go exactly (so detail-view behavior doesn't differ
// depending on which screen it was entered from), plus recording
// detailOrigin so backing out returns to table view instead of browse. The
// detail view's selection lands on the property the highlighted cell's
// column belongs to, not row 0, so "enter" on a cell reads as "inspect this
// value" rather than dropping the user back at the top of the property
// list.
// selectedTableEntity returns the table view's highlighted row's entity, or
// nil if there's none.
func (m *Model) selectedTableEntity(entities []*model.Entity) *model.Entity {
	if m.tableState == nil || m.tableState.Row < 0 || m.tableState.Row >= len(entities) {
		return nil
	}
	return entities[m.tableState.Row]
}

func (m *Model) openEntityDetailFromTable(entities []*model.Entity, cols []string) (tea.Model, tea.Cmd) {
	if m.tableState == nil || m.tableState.Row < 0 || m.tableState.Row >= len(entities) {
		return m, nil
	}
	e := entities[m.tableState.Row]
	if e == nil {
		return m, nil
	}
	m.currentEntity = e
	m.entityStack = nil
	m.detailPath.Reset()
	m.detailSelected = detailRowIndexForColumn(e, cols, m.tableState.Col)
	m.detailFilter = ""
	m.dirty.Reset()
	m.status = ""
	m.detailOrigin = screenTable
	m.screen = screenDetail
	return m, nil
}

// detailRowIndexForColumn finds which row BuildRows would give e's property
// named columns[col] (BuildRows sorts an entity's properties alphabetically
// — see entityRows in ui/panes/detail.go), so the detail view can open with
// that property already selected. Falls back to 0 (the top of the list)
// when col is out of range, the entity has no such property (a sparse
// column, showing "·" for this row), or its rows can't be built at all.
func detailRowIndexForColumn(e *model.Entity, columns []string, col int) int {
	if col < 0 || col >= len(columns) {
		return 0
	}
	rows, err := panes.BuildRows(model.Value{Kind: model.KindEntity, EntityValue: e})
	if err != nil {
		return 0
	}
	name := columns[col]
	for i, row := range rows {
		if !row.Segment.IsIndex && row.Segment.Name == name {
			return i
		}
	}
	return 0
}

// clampInt is a small local helper matching the clamping nav's columnList
// does internally, since tableState's Row/Col live outside that machinery.
func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
