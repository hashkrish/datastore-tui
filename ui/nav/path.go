package nav

import (
	"strconv"
	"strings"
)

// PropSegment is one step into an entity's property tree: a property name,
// and (when stepping into an array element) that element's index.
type PropSegment struct {
	Name    string
	IsIndex bool
	Index   int
}

func (s PropSegment) String() string {
	if s.IsIndex {
		return "[" + strconv.Itoa(s.Index) + "]"
	}
	return s.Name
}

// DetailPath is the breadcrumb of property-tree nesting within a currently
// open entity's detail view (e.g. drilling into an array of embedded
// entities). It backs the "Esc steps out one level" behavior.
type DetailPath struct {
	segments []PropSegment
}

// Push descends one level into the property tree.
func (p *DetailPath) Push(seg PropSegment) {
	p.segments = append(p.segments, seg)
}

// Pop steps back out one level, reporting the segment removed (or the zero
// value and false if already at the root).
func (p *DetailPath) Pop() (PropSegment, bool) {
	if len(p.segments) == 0 {
		return PropSegment{}, false
	}
	last := p.segments[len(p.segments)-1]
	p.segments = p.segments[:len(p.segments)-1]
	return last, true
}

// Depth reports how many levels deep the path currently is.
func (p *DetailPath) Depth() int { return len(p.segments) }

// Segments returns the path's segments, root first.
func (p *DetailPath) Segments() []PropSegment { return p.segments }

// Reset clears the path back to the entity root.
func (p *DetailPath) Reset() { p.segments = nil }

// Breadcrumb renders root (e.g. "Namespace/Kind/entityKey") plus this
// path's segments as a single "/"-joined string.
func (p *DetailPath) Breadcrumb(root string) string {
	var b strings.Builder
	b.WriteString(root)
	for _, seg := range p.segments {
		if !seg.IsIndex {
			b.WriteByte('/')
		}
		b.WriteString(seg.String())
	}
	return b.String()
}
