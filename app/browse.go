package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
)

func (m *Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := keymap.DefaultBrowseKeyMap()

	switch {
	case key.Matches(msg, km.Quit):
		return m, tea.Quit

	case key.Matches(msg, km.Help):
		m.prevScreen = screenBrowse
		m.screen = screenHelp
		return m, nil

	case key.Matches(msg, km.Down):
		m.chordG.Reset()
		m.chordD.Reset()
		m.nav.MoveBy(1)
		return m, nil

	case key.Matches(msg, km.Up):
		m.chordG.Reset()
		m.chordD.Reset()
		m.nav.MoveBy(-1)
		return m, nil

	case key.Matches(msg, km.Top):
		if m.chordG.Complete('g') {
			m.nav.MoveToTop()
		} else {
			m.chordG.Arm('g')
		}
		return m, nil

	case key.Matches(msg, km.Bottom):
		m.nav.MoveToBottom()
		return m, nil

	case key.Matches(msg, km.HalfPageDown):
		m.nav.MoveBy(m.halfPage())
		return m, nil

	case key.Matches(msg, km.HalfPageUp):
		m.nav.MoveBy(-m.halfPage())
		return m, nil

	case key.Matches(msg, km.Filter):
		m.filterInput.SetValue(m.nav.Filter())
		m.filterInput.Focus()
		m.filterInput.CursorEnd()
		m.screen = screenFilterInput
		return m, nil

	case key.Matches(msg, km.Left):
		m.chordD.Reset()
		m.nav.FocusLeft()
		return m, nil

	case key.Matches(msg, km.Right), key.Matches(msg, km.Open):
		return m.drillIn()

	case key.Matches(msg, km.Refresh):
		return m.refreshFocused()

	case key.Matches(msg, km.ListBookmarks):
		return m.startBookmarkList()

	case key.Matches(msg, km.Add):
		if m.nav.Focus == nav.ColumnEntity {
			return m.startNewEntity()
		}
		return m, nil

	case key.Matches(msg, km.DeleteMark):
		if m.nav.Focus != nav.ColumnEntity {
			return m, nil
		}
		if m.chordD.Complete('d') {
			if m.nav.SelectedEntity() != nil {
				m.prevScreen = screenBrowse
				m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
					e := mm.nav.SelectedEntity()
					mm.screen = screenBrowse
					return mm, deleteEntityCmd(mm.client, e.Key)
				}
				m.screen = screenConfirmDeleteEntity
			}
		} else {
			m.chordD.Arm('d')
		}
		return m, nil

	default:
		m.chordG.Reset()
		m.chordD.Reset()
		return m, nil
	}
}

// drillIn implements "l"/"enter" in browse mode: moving focus one Miller
// column to the right, loading that column's data based on the selection to
// its left, or (from the entity column) opening the detail view.
func (m *Model) drillIn() (tea.Model, tea.Cmd) {
	switch m.nav.Focus {
	case nav.ColumnNamespace:
		ns, ok := m.nav.SelectedNamespace()
		if !ok || !m.nav.FocusRight() {
			return m, nil
		}
		m.namespace = ns
		return m, loadKindsCmd(m.client, ns)

	case nav.ColumnKind:
		kind, ok := m.nav.SelectedKind()
		if !ok || !m.nav.FocusRight() {
			return m, nil
		}
		return m, loadEntitiesCmd(m.client, m.namespace, kind, "", false)

	default: // ColumnEntity
		e := m.nav.SelectedEntity()
		if e == nil {
			return m, nil
		}
		m.currentEntity = e
		m.detailPath.Reset()
		m.detailSelected = 0
		m.detailFilter = ""
		m.dirty.Reset()
		m.status = ""
		m.screen = screenDetail
		return m, nil
	}
}

// refreshFocused re-fetches the focused column's data ("R").
func (m *Model) refreshFocused() (tea.Model, tea.Cmd) {
	switch m.nav.Focus {
	case nav.ColumnNamespace:
		return m, loadNamespacesCmd(m.client)
	case nav.ColumnKind:
		ns, ok := m.nav.SelectedNamespace()
		if !ok {
			return m, nil
		}
		return m, loadKindsCmd(m.client, ns)
	default:
		kind, ok := m.nav.SelectedKind()
		if !ok {
			return m, nil
		}
		return m, loadEntitiesCmd(m.client, m.namespace, kind, "", false)
	}
}

// startNewEntity opens the key-entry form for a brand new entity ("o" on
// the entity column); the entity itself is created (and its detail view
// opened) once the key form completes, see updateNewEntityKey.
func (m *Model) startNewEntity() (tea.Model, tea.Cmd) {
	kind, _ := m.nav.SelectedKind()
	m.newEntityKeyKind = kind
	m.newEntityKeyID = ""
	m.newEntityKeyName = ""
	m.newEntityKeyForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Kind").Value(&m.newEntityKeyKind),
		huh.NewInput().Title("ID (numeric; leave blank to use Name instead)").Value(&m.newEntityKeyID),
		huh.NewInput().Title("Name (leave blank to use ID instead)").Value(&m.newEntityKeyName),
	))
	m.screen = screenNewEntityKey
	return m, m.newEntityKeyForm.Init()
}

func (m *Model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.nav.SetFilter(m.filterInput.Value())
		m.screen = screenBrowse
		return m, nil
	case "esc":
		m.screen = screenBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirmYes
	m.confirmYes = nil
	if msg.String() == "y" && action != nil {
		return action(m)
	}
	m.screen = m.prevScreen
	return m, nil
}
