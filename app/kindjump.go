package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/ui/nav"
)

// startKindJump implements ":" in browse mode: loads the current namespace's
// kinds for a filterable picker, so the user can jump straight to any kind's
// entities without stepping through the Kind column.
func (m *Model) startKindJump() (tea.Model, tea.Cmd) {
	if _, ok := m.nav.SelectedNamespace(); !ok {
		return m, nil
	}
	m.kindJumpKind = ""
	m.prevScreen = screenBrowse
	m.status = "loading kinds..."
	return m, loadKindJumpKindsCmd(m.client, m.id, m.namespace)
}

// openKindJumpForm builds kindJumpForm once loadKindJumpKindsCmd resolves —
// a single-field filterable pick-list of every kind in the namespace,
// mirroring openRefKindForm.
func (m *Model) openKindJumpForm(kinds []string) (tea.Model, tea.Cmd) {
	m.status = ""
	if len(kinds) == 0 {
		m.err = errRefNoKinds
		m.screen = screenBrowse
		return m, nil
	}
	m.kindJumpKinds = kinds
	m.kindJumpKind = kinds[0]

	opts := make([]huh.Option[string], len(kinds))
	for i, k := range kinds {
		opts[i] = huh.NewOption(k, k)
	}
	m.kindJumpForm = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Jump to kind").Options(opts...).Value(&m.kindJumpKind).Filtering(true),
	))
	if m.width > 0 {
		m.kindJumpForm = m.kindJumpForm.WithWidth(m.width)
	}
	if h := formHeight(m.height); h > 0 {
		m.kindJumpForm = m.kindJumpForm.WithHeight(h)
	}
	m.screen = screenKindJump
	return m, m.kindJumpForm.Init()
}

// updateKindJumpForm drives kindJumpForm and, on completion, commits the
// jump: switches the Kind/Entity columns to the picked kind and loads its
// entities, the same landing drillIn's ColumnKind case produces.
func (m *Model) updateKindJumpForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.kindJumpForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.kindJumpForm = f
	}

	switch m.kindJumpForm.State {
	case huh.StateCompleted:
		m.nav.SetKinds(m.kindJumpKinds)
		m.nav.SelectKind(m.kindJumpKind)
		m.nav.Focus = nav.ColumnEntity
		m.activeFilters = nil
		m.activeOrder = nil

		m.screen = screenBrowse
		m.status = "loading entities..."
		return m, loadEntitiesCmd(m.client, m.id, m.namespace, m.kindJumpKind, "", false, nil)
	case huh.StateAborted:
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}
