package panes

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(accentColor).Padding(0, 1)
	inactiveTabStyle = dimStyle.Padding(0, 1)
)

// RenderTabBar renders one line listing every tab's label (1-indexed),
// highlighting active. When there isn't room for every tab, it drops tabs
// farthest from the active one first (an ellipsis marks whichever side(s)
// got elided) rather than hard-truncating the joined line from the right —
// which could otherwise clip the active tab's own segment off entirely,
// making the bar look like it isn't showing which tab is current. Only
// shown once a second tab exists — see Model.tabBarHeight.
func RenderTabBar(labels []string, active int, width int) string {
	lo, hi := 0, len(labels)
	for hi-lo > 1 && tabBarWidth(labels, lo, hi, lo > 0, hi < len(labels)) > width {
		if active-lo > hi-1-active {
			lo++
		} else {
			hi--
		}
	}

	segs := make([]string, 0, hi-lo+2)
	if lo > 0 {
		segs = append(segs, dimStyle.Render("…"))
	}
	for i := lo; i < hi; i++ {
		text := fmt.Sprintf("%d:%s", i+1, labels[i])
		style := inactiveTabStyle
		if i == active {
			style = activeTabStyle
		}
		segs = append(segs, style.Render(text))
	}
	if hi < len(labels) {
		segs = append(segs, dimStyle.Render("…"))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, segs...)
	return lipgloss.NewStyle().Width(width).MaxHeight(1).Render(truncate(line, width))
}

// tabBarWidth measures how wide RenderTabBar's rendered line would be for
// labels[lo:hi] plus ellipsis markers on whichever side(s) are elided —
// used to decide how far the visible window needs to shrink to fit width.
func tabBarWidth(labels []string, lo, hi int, leftEllipsis, rightEllipsis bool) int {
	w := 0
	if leftEllipsis {
		w++
	}
	if rightEllipsis {
		w++
	}
	for i := lo; i < hi; i++ {
		w += len(fmt.Sprintf("%d:%s", i+1, labels[i])) + 2 // +2 for Padding(0, 1)
	}
	return w
}
