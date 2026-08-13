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

// focusedStyle marks the currently-focused Miller column in the status-line
// breadcrumb (FormatBrowseBreadcrumb) — the role the column headers used to
// play before they were removed for eating a row that mattered once a
// column's item count exceeded the pane's height.
var (
	focusedStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)

	selectedRowStyle = lipgloss.NewStyle().
				Background(accentColor).
				Foreground(lipgloss.Color("15")).
				Bold(true)

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// RenderBrowse renders a sliding three-pane view — the column one level
// above focus ("parent"), the focused Miller column ("current"), and a
// preview of the column one level below ("preview") — sized to width/height
// as 20%/30%/50% of the space. Which nav.Column plays "parent" and
// "current" shifts with state.Focus (ranger-style drill-in): focusing Kind
// shows Namespace/Kind/Entity-preview, focusing Entity shows
// Kind/Entity/entity-property-preview. At the top of the hierarchy (Focus ==
// ColumnNamespace) there is no parent, so that pane renders blank; at the
// bottom (Focus == ColumnEntity) there is no column below, so the preview
// pane shows the highlighted entity's properties instead of a list. Panes
// have no header row — see FormatBrowseBreadcrumb for the "where am I"
// context shown in the status line instead.
func RenderBrowse(state *nav.State, width, height int) string {
	available := width - 2*columnGap
	parentWidth := available * 20 / 100
	currentWidth := available * 30 / 100
	previewWidth := available - parentWidth - currentWidth
	gap := gapColumn(height)

	var parentPane string
	if state.Focus == nav.ColumnNamespace {
		parentPane = renderEmptyColumn(parentWidth, height)
	} else {
		parentCol := state.Focus - 1
		parentPane = renderColumn(state.VisibleItems(parentCol), state.SelectedIndex(parentCol), parentWidth, height)
	}
	currentPane := renderColumn(state.VisibleItems(state.Focus), state.SelectedIndex(state.Focus), currentWidth, height)

	var previewPane string
	if state.Focus == nav.ColumnEntity {
		previewPane = renderPreviewColumn(state.SelectedEntity(), "(no entity selected)", previewWidth, height)
	} else {
		childCol := state.Focus + 1
		previewPane = renderColumn(state.VisibleItems(childCol), state.SelectedIndex(childCol), previewWidth, height)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parentPane, gap, currentPane, gap, previewPane)
}

// columnTitle returns the display label for a nav.Column, used by
// FormatBrowseBreadcrumb (the status line), not by the panes themselves.
func columnTitle(col nav.Column) string {
	switch col {
	case nav.ColumnNamespace:
		return "Namespace"
	case nav.ColumnKind:
		return "Kind"
	default:
		return "Entity"
	}
}

// FormatBrowseBreadcrumb renders the currently highlighted
// namespace/kind/entity as "namespace / kind / entity" for the bottom
// status line, with whichever segment is focused highlighted — replacing
// the "where am I" job the removed column headers used to do. A segment
// with nothing selected yet renders as "-".
func FormatBrowseBreadcrumb(state *nav.State) string {
	ns, _ := state.SelectedNamespace()
	kind, _ := state.SelectedKind()
	entity := ""
	if e := state.SelectedEntity(); e != nil {
		entity = e.Key.Last().String()
	}

	segs := []string{ns, kind, entity}
	cols := []nav.Column{nav.ColumnNamespace, nav.ColumnKind, nav.ColumnEntity}
	for i, col := range cols {
		text := segs[i]
		if text == "" {
			text = "-"
		}
		if state.Focus == col {
			text = focusedStyle.Render(columnTitle(col) + ": " + text)
		}
		segs[i] = text
	}
	return strings.Join(segs, " / ")
}

// renderEmptyColumn renders a blank parent pane for when focus is on the
// topmost Miller column (Namespace) and there is nothing above it to show.
func renderEmptyColumn(width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Render(dimStyle.Render("  (top level)"))
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

	listCol := renderColumn(labels, selected, colWidth, height)

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
		visibleRows := max(height-1, 1) // minus the key line
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

func renderColumn(items []string, selected int, width, height int) string {
	var b strings.Builder

	if len(items) == 0 {
		b.WriteString(dimStyle.Render("  (empty)"))
	} else {
		// Keep the selected row within view by windowing when the list is
		// taller than the available rows.
		visibleRows := max(height, 1)
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
