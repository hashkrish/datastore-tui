package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/krishnan/datastore-tui/datastore/model"
)

var (
	breadcrumbStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(0, 1)
	kindLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	containerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
)

// RenderDetail renders rows (as built by BuildRows) as a full-width list
// under breadcrumb, highlighting the row at selected.
func RenderDetail(breadcrumb string, rows []DetailRow, selected int, width, height int) string {
	var b strings.Builder
	b.WriteString(breadcrumbStyle.Render(breadcrumb))
	b.WriteString("\n\n")

	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("  (no properties)"))
		return b.String()
	}

	visibleRows := max(height-2, 1)
	start := 0
	if selected >= visibleRows {
		start = selected - visibleRows + 1
	}
	end := min(start+visibleRows, len(rows))

	for i := start; i < end; i++ {
		line := " " + formatRow(rows[i], width)
		if i == selected {
			line = selectedRowStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func formatRow(row DetailRow, width int) string {
	label := row.Segment.String()
	kindLabel := kindLabelStyle.Render(row.Kind.String())
	preview := row.Preview
	if row.IsContainer {
		preview = containerStyle.Render(preview)
	}
	return truncate(label+"  "+kindLabel+"  "+preview, width-2)
}

// LeafRowValue resolves the actual model.Value for a leaf DetailRow given
// its parent scope value, for handing off to an edit form.
func LeafRowValue(scope model.Value, row DetailRow) (model.Value, error) {
	return getChild(scope, row.Segment)
}
