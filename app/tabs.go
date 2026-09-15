package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/krishnan/datastore-tui/ui/keymap"
)

// isStableScreen reports whether s is one of the screens tab-switching keys
// are allowed to interrupt: the three "resting" screens a user can be on
// between actions, as opposed to a form, confirm prompt, or other modal
// overlay mid-flight. Restricting tab switches to these keeps a switch from
// silently abandoning in-progress input.
func isStableScreen(s screen) bool {
	switch s {
	case screenBrowse, screenDetail, screenTable:
		return true
	default:
		return false
	}
}

// handleTabKey intercepts the tab-management keys ("ctrl+t"/"ctrl+w" open/
// close, "tab"/"shift+tab" cycle, "1"-"9" jump), reporting whether msg was
// one of them so handleKey can fall through to its normal per-screen
// dispatch otherwise. Only called while the active tab is on a stable
// screen — see isStableScreen.
func (m *Model) handleTabKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	km := keymap.DefaultTabKeyMap()
	switch {
	case key.Matches(msg, km.NewTab):
		mm, cmd := m.newTabAction()
		return true, mm, cmd
	case key.Matches(msg, km.CloseTab):
		mm, cmd := m.closeTabAction()
		return true, mm, cmd
	case key.Matches(msg, km.NextTab):
		m.nextTab()
		return true, m, nil
	case key.Matches(msg, km.PrevTab):
		m.prevTab()
		return true, m, nil
	}
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		m.gotoTab(int(s[0] - '0'))
		return true, m, nil
	}
	return false, m, nil
}

// newTabAction implements "ctrl+t": opens a brand new tab, starting fresh at
// the namespace column, and makes it the active one.
func (m *Model) newTabAction() (tea.Model, tea.Cmd) {
	t := newTab(m.nextTabID)
	m.nextTabID++
	m.tabs = append(m.tabs, t)
	m.active = len(m.tabs) - 1
	m.tab = t
	return m, loadNamespacesCmd(m.client, t.id)
}

// closeTabAction implements "ctrl+w": closes the active tab, confirming
// first if it has unsaved detail edits (reusing screenConfirmQuit, the same
// overlay/wording used elsewhere for discarding unsaved edits). A no-op on
// the last remaining tab — "q" quits the app instead.
func (m *Model) closeTabAction() (tea.Model, tea.Cmd) {
	if len(m.tabs) == 1 {
		return m, nil
	}
	if m.dirty.Dirty() {
		m.prevScreen = m.screen
		closing := m.id
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			mm.removeTab(closing)
			return mm, nil
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	m.removeTab(m.id)
	return m, nil
}

// removeTab splices the tab with the given id out of m.tabs, adjusting
// m.active and the embedded m.tab so the active tab stays valid — the
// nearest remaining tab (preferring the one now at the same index, or the
// last tab if the closed one was rightmost).
func (m *Model) removeTab(id int) {
	idx := -1
	for i, t := range m.tabs {
		if t.id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if len(m.tabs) == 0 {
		return
	}
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	} else if idx < m.active {
		m.active--
	}
	m.tab = m.tabs[m.active]
}

// nextTab/prevTab implement "tab"/"shift+tab": cycling the active tab,
// wrapping around at either end.
func (m *Model) nextTab() {
	m.active = (m.active + 1) % len(m.tabs)
	m.tab = m.tabs[m.active]
}

func (m *Model) prevTab() {
	m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
	m.tab = m.tabs[m.active]
}

// gotoTab implements "1"-"9": jumps directly to the n'th tab (1-indexed), a
// no-op if there's no such tab.
func (m *Model) gotoTab(n int) {
	if n >= 1 && n <= len(m.tabs) {
		m.active = n - 1
		m.tab = m.tabs[m.active]
	}
}

// requestQuit implements every "quit the app" key (browse mode's "q"/
// "ctrl+c", table mode's "ctrl+c"). It replaces an unconditional tea.Quit:
// tabs let a background tab sit mid-edit while a different tab is active,
// so quitting from browse/table no longer guarantees every tab is clean the
// way it did before tabs existed (a dirty detail view always had to be
// resolved, via screenConfirmQuit, before you could reach browse/table in
// that same tab) — see Model.confirmingQuit.
func (m *Model) requestQuit() (tea.Model, tea.Cmd) {
	for _, t := range m.tabs {
		if t.dirty.Dirty() {
			m.confirmingQuit = true
			return m, nil
		}
	}
	return m, tea.Quit
}

// updateConfirmingQuit drives the Model.confirmingQuit overlay: "y" quits
// for real, anything else cancels back to whatever was on screen.
func (m *Model) updateConfirmingQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmingQuit = false
	if msg.String() == "y" {
		return m, tea.Quit
	}
	return m, nil
}

// confirmingQuitMessage renders the Model.confirmingQuit overlay's text.
func (m *Model) confirmingQuitMessage() string {
	n := 0
	for _, t := range m.tabs {
		if t.dirty.Dirty() {
			n++
		}
	}
	if n == 1 {
		return "1 tab has unsaved changes. Quit anyway? (y/n)"
	}
	return fmt.Sprintf("%d tabs have unsaved changes. Quit anyway? (y/n)", n)
}
