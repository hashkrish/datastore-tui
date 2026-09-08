package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// drainN feeds cmd's resulting message back into m.Update, and so on for up
// to n hops, simulating what a running tea.Program's event loop does with
// the commands a huh.Form returns internally (e.g. the nextFieldMsg/
// nextGroupMsg chain huh.Form.Update emits when a field is confirmed) — the
// harness has no real event loop, so tests that press "enter" on a huh
// field must drain that chain themselves or the form will appear to just
// eat the keystroke. n bounds how many hops are drained, so a test can stop
// short of a command that would make a real network call (nil m.client).
func drainN(t *testing.T, m *Model, cmd tea.Cmd, n int) *Model {
	t.Helper()
	for i := 0; i < n && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			return m
		}
		if bm, ok := msg.(tea.BatchMsg); ok {
			for _, c := range bm {
				m = drainN(t, m, c, n-i)
			}
			return m
		}
		var mdl tea.Model
		mdl, cmd = m.Update(msg)
		m = mdl.(*Model)
	}
	return m
}

// TestRefKindForm_EnterCompletesForm guards against a regression where
// pressing enter on the (single-field) kind picker let you move the
// highlight between options ("focus") but never actually completed the
// form ("select"): huh.Form.Update's internal nextFieldMsg/nextGroupMsg
// chain isn't a tea.KeyMsg, so it only reaches the form if Model.Update
// explicitly forwards the current screen's messages to it — screenRefKind/
// screenRefProperty were missing from that forwarding switch when first
// added (see the switch above handleKey in model.go).
func TestRefKindForm_EnterCompletesForm(t *testing.T) {
	m := New(nil, false)
	m.width, m.height = 80, 24

	mdl, initCmd := m.openRefKindForm([]string{"Alpha", "Beta", "Gamma"})
	m = mdl.(*Model)
	m = drainN(t, m, initCmd, 3)
	if m.screen != screenRefKind {
		t.Fatalf("screen = %v, want screenRefKind", m.screen)
	}

	mdl, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mdl.(*Model)
	m = drainN(t, m, cmd, 3)
	if m.refTargetKind != "Beta" {
		t.Fatalf("refTargetKind after down = %q, want Beta", m.refTargetKind)
	}

	// 2 hops: nextFieldMsg then nextGroupMsg (form completion) — stop short
	// of the resulting loadRefPropertiesCmd, which would fire a real
	// network call against the nil client.
	mdl, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(*Model)
	m = drainN(t, m, cmd, 2)

	if m.refKindForm.State != huh.StateCompleted {
		t.Fatalf("refKindForm.State = %v, want StateCompleted (enter did not complete the form)", m.refKindForm.State)
	}
	if m.status != "loading properties..." {
		t.Fatalf("status = %q, want %q (form completion should have moved on to the property step)", m.status, "loading properties...")
	}
	if m.refTargetKind != "Beta" {
		t.Fatalf("refTargetKind = %q, want Beta", m.refTargetKind)
	}
}
