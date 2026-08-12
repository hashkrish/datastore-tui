package nav

import "testing"

func TestPropSegment_String(t *testing.T) {
	if got, want := (PropSegment{Name: "address"}).String(), "address"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got, want := (PropSegment{IsIndex: true, Index: 3}).String(), "[3]"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestDetailPath_PushPopDepth(t *testing.T) {
	var p DetailPath
	if p.Depth() != 0 {
		t.Fatalf("Depth() = %d, want 0 for a fresh path", p.Depth())
	}

	p.Push(PropSegment{Name: "address"})
	p.Push(PropSegment{IsIndex: true, Index: 2})
	if p.Depth() != 2 {
		t.Fatalf("Depth() = %d, want 2", p.Depth())
	}

	seg, ok := p.Pop()
	if !ok || !seg.IsIndex || seg.Index != 2 {
		t.Fatalf("Pop() = %+v, %v, want index 2", seg, ok)
	}
	if p.Depth() != 1 {
		t.Fatalf("Depth() after Pop = %d, want 1", p.Depth())
	}

	seg, ok = p.Pop()
	if !ok || seg.Name != "address" {
		t.Fatalf("Pop() = %+v, %v, want address", seg, ok)
	}

	if _, ok := p.Pop(); ok {
		t.Fatalf("Pop() on an empty path returned ok=true, want false")
	}
}

func TestDetailPath_Reset(t *testing.T) {
	var p DetailPath
	p.Push(PropSegment{Name: "a"})
	p.Push(PropSegment{Name: "b"})
	p.Reset()
	if p.Depth() != 0 {
		t.Fatalf("Depth() after Reset = %d, want 0", p.Depth())
	}
}

func TestDetailPath_Breadcrumb(t *testing.T) {
	var p DetailPath
	if got, want := p.Breadcrumb("root"), "root"; got != want {
		t.Fatalf("Breadcrumb() = %q, want %q", got, want)
	}

	p.Push(PropSegment{Name: "tags"})
	p.Push(PropSegment{IsIndex: true, Index: 0})
	p.Push(PropSegment{Name: "label"})
	if got, want := p.Breadcrumb("root"), "root/tags[0]/label"; got != want {
		t.Fatalf("Breadcrumb() = %q, want %q", got, want)
	}
}

func TestDetailPath_Segments_RootFirst(t *testing.T) {
	var p DetailPath
	p.Push(PropSegment{Name: "a"})
	p.Push(PropSegment{Name: "b"})
	segs := p.Segments()
	if len(segs) != 2 || segs[0].Name != "a" || segs[1].Name != "b" {
		t.Fatalf("Segments() = %+v, want [a b]", segs)
	}
}
