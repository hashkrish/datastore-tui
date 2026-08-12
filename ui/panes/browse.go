package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// accentColor is the theme's single accent hue — the focused-column border,
// the selected-row highlight, and the detail-view breadcrumb all key off it,
// so changing the theme is a one-line edit here.
const accentColor = lipgloss.Color("25") // muted blue

var (
	columnTitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor)

	blurredBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	selectedRowStyle = lipgloss.NewStyle().
				Background(accentColor).
				Foreground(lipgloss.Color("15")).
				Bold(true)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// RenderBrowse renders the three Miller columns (namespace/kind/entity) plus
// a fourth preview pane showing the highlighted entity's properties (so its
// fields are visible before actually opening it for edit), side by side and
// sized to fit width/height, with the focused column highlighted.
func RenderBrowse(state *nav.State, width, height int) string {
	colWidth := width / 4
	previewWidth := width - 3*colWidth

	cols := []string{
		renderColumn("Namespace", state.VisibleItems(nav.ColumnNamespace), state.SelectedIndex(nav.ColumnNamespace), state.Focus == nav.ColumnNamespace, colWidth, height),
		renderColumn("Kind", state.VisibleItems(nav.ColumnKind), state.SelectedIndex(nav.ColumnKind), state.Focus == nav.ColumnKind, colWidth, height),
		renderColumn("Entity", state.VisibleItems(nav.ColumnEntity), state.SelectedIndex(nav.ColumnEntity), state.Focus == nav.ColumnEntity, colWidth, height),
		renderPreviewColumn(state.SelectedEntity(), previewWidth, height),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// renderPreviewColumn shows the highlighted entity's key and top-level
// properties, read-only — a ranger-style preview of what "enter" would open.
func renderPreviewColumn(e *model.Entity, width, height int) string {
	innerHeight := height - 2
	innerWidth := width - 2

	var b strings.Builder
	b.WriteString(columnTitleStyle.Render("Preview"))
	b.WriteString("\n")

	if e == nil {
		b.WriteString(dimStyle.Render("  (no entity selected)"))
		return blurredBorder.Width(innerWidth).Height(innerHeight).Render(b.String())
	}

	b.WriteString(truncate(" "+e.Key.Last().String(), innerWidth))
	b.WriteString("\n")
	rows, err := BuildRows(model.Value{Kind: model.KindEntity, EntityValue: e})
	switch {
	case err != nil:
		b.WriteString(dimStyle.Render("  " + err.Error()))
	case len(rows) == 0:
		b.WriteString(dimStyle.Render("  (no properties)"))
	default:
		visibleRows := max(innerHeight-2, 1) // minus title + key line
		for i, row := range rows {
			if i >= visibleRows {
				break
			}
			line := " " + formatRow(row, innerWidth-2)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return blurredBorder.Width(innerWidth).Height(innerHeight).Render(b.String())
}

func renderColumn(title string, items []string, selected int, focused bool, width, height int) string {
	border := blurredBorder
	if focused {
		border = focusedBorder
	}
	innerHeight := height - 2 // border top/bottom
	innerWidth := width - 2   // border left/right

	var b strings.Builder
	b.WriteString(columnTitleStyle.Render(title))
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  (empty)"))
	} else {
		// Keep the selected row within view by windowing when the list is
		// taller than the available rows.
		visibleRows := max(innerHeight-1, 1) // minus the title line
		start := 0
		if selected >= visibleRows {
			start = selected - visibleRows + 1
		}
		end := min(start+visibleRows, len(items))
		for i := start; i < end; i++ {
			row := truncate(items[i], innerWidth-2)
			line := " " + row
			if i == selected {
				line = selectedRowStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return border.Width(innerWidth).Height(innerHeight).Render(b.String())
}

// truncate clips s to width terminal columns, appending an ellipsis when it
// doesn't fit. It must go through ansi.Truncate rather than plain byte
// slicing: several callers pass strings already wrapped in lipgloss styling
// (e.g. formatRow's kind/container coloring), and slicing those by raw byte
// index can cut a string in the middle of an ANSI escape sequence or a
// multi-byte rune — the dropped reset code then lets that color bleed into
// whatever renders next (visible as a column's border looking shifted or
// doubled).
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}
