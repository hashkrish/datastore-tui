package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// accentColor is the theme's single accent hue — the focused-column title,
// the selected-row highlight, and the detail-view breadcrumb all key off it,
// so changing the theme is a one-line edit here.
const accentColor = lipgloss.Color("25") // muted blue

// columnGap is the blank-column width separating adjacent panes, standing
// in for the border that used to mark their edges.
const columnGap = 1

var (
	columnTitleStyle        = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	focusedColumnTitleStyle = columnTitleStyle.Foreground(accentColor)

	selectedRowStyle = lipgloss.NewStyle().
				Background(accentColor).
				Foreground(lipgloss.Color("15")).
				Bold(true)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// RenderBrowse renders the three Miller columns (namespace/kind/entity) plus
// a fourth preview pane showing the highlighted entity's properties (so its
// fields are visible before actually opening it for edit), side by side and
// sized to fit width/height, with the focused column's title highlighted.
func RenderBrowse(state *nav.State, width, height int) string {
	colWidth := (width - 3*columnGap) / 4
	previewWidth := width - 3*colWidth - 3*columnGap
	gap := gapColumn(height)

	cols := []string{
		renderColumn("Namespace", state.VisibleItems(nav.ColumnNamespace), state.SelectedIndex(nav.ColumnNamespace), state.Focus == nav.ColumnNamespace, colWidth, height),
		gap,
		renderColumn("Kind", state.VisibleItems(nav.ColumnKind), state.SelectedIndex(nav.ColumnKind), state.Focus == nav.ColumnKind, colWidth, height),
		gap,
		renderColumn("Entity", state.VisibleItems(nav.ColumnEntity), state.SelectedIndex(nav.ColumnEntity), state.Focus == nav.ColumnEntity, colWidth, height),
		gap,
		renderPreviewColumn(state.SelectedEntity(), "(no entity selected)", previewWidth, height),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// RenderBookmarks renders the bookmark picker (ctrl+l): a scrollable list of
// bookmark labels on the left, highlighting selected, plus a preview pane on
// the right showing the highlighted bookmark's entity properties — the same
// live preview RenderBrowse gives the Miller-column entity list. preview is
// nil either while its entity is still being fetched (loading) or if the
// lookup came back without it (the bookmarked entity no longer exists).
func RenderBookmarks(labels []string, selected int, preview *model.Entity, loading bool, width, height int) string {
	colWidth := (width - columnGap) / 3
	previewWidth := width - colWidth - columnGap

	listCol := renderColumn("Bookmarks", labels, selected, true, colWidth, height)

	emptyMessage := "(entity not found)"
	if loading {
		emptyMessage = "(loading...)"
	}
	previewCol := renderPreviewColumn(preview, emptyMessage, previewWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, listCol, gapColumn(height), previewCol)
}

// gapColumn is a blank spacer of columnGap width and height rows tall, used
// to separate adjacent panes now that they no longer have their own border.
func gapColumn(height int) string {
	return lipgloss.NewStyle().Width(columnGap).Height(height).Render("")
}

// renderPreviewColumn shows the highlighted entity's key and top-level
// properties, read-only — a ranger-style preview of what "enter" would open.
// emptyMessage is shown in place of the property list when e is nil.
func renderPreviewColumn(e *model.Entity, emptyMessage string, width, height int) string {
	var b strings.Builder
	b.WriteString(columnTitleStyle.Render("Preview"))
	b.WriteString("\n")

	if e == nil {
		b.WriteString(dimStyle.Render("  " + emptyMessage))
		return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
	}

	b.WriteString(truncate(" "+e.Key.Last().String(), width))
	b.WriteString("\n")
	rows, err := BuildRows(model.Value{Kind: model.KindEntity, EntityValue: e})
	switch {
	case err != nil:
		b.WriteString(dimStyle.Render("  " + err.Error()))
	case len(rows) == 0:
		b.WriteString(dimStyle.Render("  (no properties)"))
	default:
		visibleRows := max(height-2, 1) // minus title + key line
		for i, row := range rows {
			if i >= visibleRows {
				break
			}
			line := " " + formatRow(row, width-2)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

func renderColumn(title string, items []string, selected int, focused bool, width, height int) string {
	titleStyle := columnTitleStyle
	if focused {
		titleStyle = focusedColumnTitleStyle
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  (empty)"))
	} else {
		// Keep the selected row within view by windowing when the list is
		// taller than the available rows.
		visibleRows := max(height-1, 1) // minus the title line
		start := 0
		if selected >= visibleRows {
			start = selected - visibleRows + 1
		}
		end := min(start+visibleRows, len(items))
		for i := start; i < end; i++ {
			row := truncate(items[i], width-2)
			line := " " + row
			if i == selected {
				line = selectedRowStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

// truncate clips s to width terminal columns, appending an ellipsis when it
// doesn't fit. It must go through ansi.Truncate rather than plain byte
// slicing: several callers pass strings already wrapped in lipgloss styling
// (e.g. formatRow's kind/container coloring), and slicing those by raw byte
// index can cut a string in the middle of an ANSI escape sequence or a
// multi-byte rune — the dropped reset code then lets that color bleed into
// whatever renders next (visible as neighboring columns' text picking up
// the wrong color).
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}
