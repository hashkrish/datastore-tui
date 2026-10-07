package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/krishnan/datastore-tui/datastore/model"
)

var (
	breadcrumbStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(0, 1)
	kindLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	containerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
)

// RenderDetail renders rows (as built by BuildRows) as a full-width list
// under breadcrumb, highlighting the row at selected. When hasPreview is
// set, the list shrinks to the left third and preview (see BlobPreview) is
// shown read-only in a pane to its right.
func RenderDetail(breadcrumb string, rows []DetailRow, selected int, preview string, hasPreview bool, width, height int) string {
	var b strings.Builder
	// Unlike every row below it, breadcrumb has no natural upper bound on
	// length (namespace/kind/id, plus a nested-path segment per level
	// drilled into) — truncate it to width so a long one can't wrap in the
	// terminal itself and silently push everything below (and, via the
	// resulting scroll, the tab bar above it) down by a row.
	b.WriteString(breadcrumbStyle.Render(truncate(breadcrumb, width-2)))
	b.WriteString("\n\n")

	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("  (no properties)"))
		return b.String()
	}

	if hasPreview {
		listHeight := max(height-2, 1)
		listWidth := (width - columnGap) / 3
		list := renderDetailRows(rows, selected, listWidth, listHeight)
		previewCol := renderTextPreview(preview, width-listWidth-columnGap, listHeight)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, list, gapColumn(listHeight), previewCol))
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

// renderDetailRows is RenderDetail's row list confined to a width×height
// column, for when it shares the screen with a preview pane.
func renderDetailRows(rows []DetailRow, selected int, width, height int) string {
	var b strings.Builder
	start := 0
	if selected >= height {
		start = selected - height + 1
	}
	end := min(start+height, len(rows))
	for i := start; i < end; i++ {
		if i > start {
			b.WriteString("\n")
		}
		line := " " + formatRow(rows[i], width)
		if i == selected {
			line = selectedRowStyle.Render(line)
		}
		b.WriteString(line)
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

// renderTextPreview shows text read-only in a width×height pane, word
// wrapping long lines (see wrapPreviewLine) and clipping to height. Tabs are
// expanded and carriage returns dropped so neither can throw off the
// terminal's own column accounting.
func renderTextPreview(text string, width, height int) string {
	if text == "" {
		return lipgloss.NewStyle().Width(width).Height(height).Render(dimStyle.Render("  (empty)"))
	}
	text = strings.NewReplacer("\t", "    ", "\r", "").Replace(text)
	limit := max(width-1, 1)
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, wrapPreviewLine(line, limit)...)
		if len(lines) >= height {
			lines = lines[:height]
			break
		}
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

// wrapPreviewLine word wraps line to limit columns (hard breaking words
// longer than that), repeating line's leading indentation on each
// continuation so wrapped pretty-printed JSON stays aligned under its key.
// The indent is dropped when it would leave less than half the width.
func wrapPreviewLine(line string, limit int) []string {
	body := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(body)]
	if len(indent) > limit/2 {
		indent = ""
		body = line
	}
	wrapped := strings.Split(ansi.Wrap(body, limit-len(indent), ""), "\n")
	for i := range wrapped {
		wrapped[i] = indent + wrapped[i]
	}
	return wrapped
}

// BlobPreview returns the decoded text of the selected row when scope is an
// array and that row is a blob holding displayable text (see
// model.DecodeBlobText); ok is false otherwise, including for binary blobs.
func BlobPreview(scope model.Value, rows []DetailRow, selected int) (text string, ok bool) {
	if scope.Kind != model.KindArray || selected < 0 || selected >= len(rows) {
		return "", false
	}
	row := rows[selected]
	if row.Kind != model.KindBlob {
		return "", false
	}
	v, err := LeafRowValue(scope, row)
	if err != nil {
		return "", false
	}
	return model.DecodeBlobText(v.BlobValue)
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
