package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/panes"
)

func (m *Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := keymap.DefaultDetailKeyMap()

	switch {
	case key.Matches(msg, km.Quit):
		return m.exitDetailToBrowse()

	case key.Matches(msg, km.Back):
		return m.detailBack()

	case key.Matches(msg, km.Up):
		m.chordD.Reset()
		if m.detailSelected > 0 {
			m.detailSelected--
		}
		return m, nil

	case key.Matches(msg, km.Down):
		m.chordD.Reset()
		_, rows, err := m.currentScope()
		if err == nil && m.detailSelected < len(rows)-1 {
			m.detailSelected++
		}
		return m, nil

	case key.Matches(msg, km.HalfPageDown):
		m.chordD.Reset()
		_, rows, err := m.currentScope()
		if err == nil {
			m.detailSelected = min(m.detailSelected+m.halfPage(), max(0, len(rows)-1))
		}
		return m, nil

	case key.Matches(msg, km.HalfPageUp):
		m.chordD.Reset()
		m.detailSelected = max(0, m.detailSelected-m.halfPage())
		return m, nil

	case key.Matches(msg, km.Expand):
		m.chordD.Reset()
		return m.detailExpand()

	case key.Matches(msg, km.Retype):
		m.chordD.Reset()
		return m.startRetype()

	case key.Matches(msg, km.Filter):
		m.filterInput.SetValue(m.detailFilter)
		m.filterInput.Focus()
		m.filterInput.CursorEnd()
		m.screen = screenDetailFilterInput
		return m, nil

	case key.Matches(msg, km.Save):
		if m.currentEntity == nil {
			return m, nil
		}
		m.status = "saving..."
		return m, saveEntityCmd(m.client, m.currentEntity)

	case key.Matches(msg, km.AddItem):
		return m.startAddItem()

	case key.Matches(msg, km.DeleteItem):
		if m.chordD.Complete('d') {
			return m.confirmDeleteItem()
		}
		m.chordD.Arm('d')
		return m, nil

	default:
		m.chordD.Reset()
		return m, nil
	}
}

// detailExpand implements "l"/"enter" in detail mode: drilling into a
// container row, or opening an edit form for a scalar leaf row.
func (m *Model) detailExpand() (tea.Model, tea.Cmd) {
	scope, rows, err := m.currentScope()
	if err != nil {
		m.err = err
		return m, nil
	}
	if len(rows) == 0 {
		return m, nil
	}
	row := rows[m.detailSelected]
	if row.IsContainer {
		m.detailPath.Push(row.Segment)
		m.detailSelected = 0
		m.detailFilter = ""
		return m, nil
	}

	leaf, err := panes.LeafRowValue(scope, row)
	if err != nil {
		m.err = err
		return m, nil
	}

	// A null property has no scalar form of its own to edit — that's
	// expected; editing it with "enter" is a no-op (see the Note huh
	// renders for KindNull). Changing its type is a deliberate, separate
	// action bound to "t" (updateDetail's Retype case), not something a
	// plain edit should do implicitly.
	fe, ok := edit.NewFieldEditor(leaf)
	if !ok {
		return m, nil
	}
	m.fieldEditor = fe
	m.editingSegment = row.Segment
	m.screen = screenEditLeaf
	return m, fe.Form().Init()
}

// detailBack implements "h"/"esc" in detail mode: stepping out one nesting
// level, or (at the entity root) leaving the detail view for browse mode.
func (m *Model) detailBack() (tea.Model, tea.Cmd) {
	if m.detailPath.Depth() > 0 {
		m.detailPath.Pop()
		m.detailSelected = 0
		m.detailFilter = ""
		return m, nil
	}
	return m.exitDetailToBrowse()
}

func (m *Model) exitDetailToBrowse() (tea.Model, tea.Cmd) {
	if !m.dirty.Dirty() {
		m.currentEntity = nil
		m.screen = screenBrowse
		return m, nil
	}
	m.prevScreen = screenDetail
	m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
		mm.currentEntity = nil
		mm.dirty.Reset()
		mm.screen = screenBrowse
		return mm, nil
	}
	m.screen = screenConfirmQuit
	return m, nil
}

// startAddItem implements "o" in detail mode: only meaningful when the
// current scope is an array, it opens the two-step "pick a type, then edit
// its value" flow (see updateNewItemType / updateEditLeaf).
func (m *Model) startAddItem() (tea.Model, tea.Cmd) {
	scope, _, err := m.currentScope()
	if err != nil {
		m.err = err
		return m, nil
	}
	if scope.Kind != model.KindArray {
		return m, nil
	}
	m.newItemTarget = targetArrayAppend
	m.newItemKind = model.KindString
	m.newItemTypeForm = edit.TypeSelectForm(&m.newItemKind)
	m.screen = screenNewItemType
	return m, m.newItemTypeForm.Init()
}

// startRetype implements "t" in detail mode: changing the data type of
// whichever row is currently selected (leaf or container), via the same
// "pick a type, then fill in its value" flow "o" uses for new array items.
// If the user picks the same type the property already had, its existing
// value is kept (pre-filled into the value form) instead of being reset to
// a zero value.
func (m *Model) startRetype() (tea.Model, tea.Cmd) {
	_, rows, err := m.currentScope()
	if err != nil {
		m.err = err
		return m, nil
	}
	if len(rows) == 0 {
		return m, nil
	}
	row := rows[m.detailSelected]
	segs := append(m.detailPath.Segments(), row.Segment)
	current, err := panes.GetValueAtPath(m.currentEntity, segs)
	if err != nil {
		m.err = err
		return m, nil
	}
	m.editingSegment = row.Segment
	m.newItemTarget = targetLeafRetype
	m.retypeOriginal = current
	m.newItemKind = current.Kind
	m.newItemTypeForm = edit.TypeSelectForm(&m.newItemKind)
	m.screen = screenNewItemType
	return m, m.newItemTypeForm.Init()
}

func (m *Model) updateNewItemType(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.newItemTypeForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.newItemTypeForm = f
	}

	switch m.newItemTypeForm.State {
	case huh.StateCompleted:
		zero := edit.ZeroValue(m.newItemKind)
		if m.newItemTarget == targetLeafRetype && m.newItemKind == m.retypeOriginal.Kind {
			zero = m.retypeOriginal // same type re-picked: keep the existing value
		}
		if fe, ok := edit.NewFieldEditor(zero); ok {
			m.fieldEditor = fe
			m.screen = screenNewItemValue
			return m, fe.Form().Init()
		}
		// Array/Entity items have no scalar form to fill in; write the
		// (empty) container as-is.
		if err := m.writeNewItemValue(zero); err != nil {
			m.err = err
		} else {
			m.dirty.MarkDirty()
		}
		m.screen = screenDetail
		return m, nil
	case huh.StateAborted:
		m.screen = screenDetail
		return m, nil
	default:
		return m, cmd
	}
}

// writeNewItemValue completes the "pick a type, then fill in its value"
// flow: appending value as a new array item, or (when the flow was started
// by opening a null leaf for retyping) writing value onto that property.
func (m *Model) writeNewItemValue(value model.Value) error {
	if m.newItemTarget == targetArrayAppend {
		return panes.AppendArrayItem(m.currentEntity, m.detailPath.Segments(), value)
	}
	segs := append(m.detailPath.Segments(), m.editingSegment)
	return panes.SetValueAtPath(m.currentEntity, segs, value)
}

// updateEditLeaf drives whichever huh form is active for a scalar value:
// editing an existing leaf (screenEditLeaf) or filling in a freshly-added
// array item's value (screenNewItemValue).
func (m *Model) updateEditLeaf(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.fieldEditor.Form().Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		_ = f // huh.Form mutates in place; state check below reads through m.fieldEditor
	}

	switch m.fieldEditor.Form().State {
	case huh.StateCompleted:
		value, err := m.fieldEditor.Result()
		if err != nil {
			m.err = err
			m.screen = screenDetail
			m.fieldEditor = nil
			return m, nil
		}
		if m.screen == screenNewItemValue {
			err = m.writeNewItemValue(value)
		} else {
			segs := append(m.detailPath.Segments(), m.editingSegment)
			err = panes.SetValueAtPath(m.currentEntity, segs, value)
		}
		if err != nil {
			m.err = err
		} else {
			m.dirty.MarkDirty()
		}
		m.fieldEditor = nil
		m.screen = screenDetail
		return m, nil
	case huh.StateAborted:
		m.fieldEditor = nil
		m.screen = screenDetail
		return m, nil
	default:
		return m, cmd
	}
}

func (m *Model) confirmDeleteItem() (tea.Model, tea.Cmd) {
	scope, rows, err := m.currentScope()
	if err != nil {
		m.err = err
		return m, nil
	}
	if scope.Kind != model.KindArray || len(rows) == 0 {
		return m, nil
	}
	m.prevScreen = screenDetail
	m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
		_, rows, err := mm.currentScope()
		if err != nil || mm.detailSelected >= len(rows) {
			mm.screen = screenDetail
			return mm, nil
		}
		row := rows[mm.detailSelected]
		if err := panes.RemoveArrayItem(mm.currentEntity, mm.detailPath.Segments(), row.Segment.Index); err != nil {
			mm.err = err
		} else {
			mm.dirty.MarkDirty()
			if mm.detailSelected > 0 {
				mm.detailSelected--
			}
		}
		mm.screen = screenDetail
		return mm, nil
	}
	m.screen = screenConfirmDeleteItem
	return m, nil
}

func (m *Model) updateDetailFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.detailFilter = m.filterInput.Value()
		m.detailSelected = 0
		m.screen = screenDetail
		return m, nil
	case "esc":
		m.screen = screenDetail
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m *Model) updateNewEntityKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.newEntityKeyForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.newEntityKeyForm = f
	}

	switch m.newEntityKeyForm.State {
	case huh.StateCompleted:
		return m.finishNewEntity()
	case huh.StateAborted:
		m.screen = screenBrowse
		return m, nil
	default:
		return m, cmd
	}
}

func (m *Model) finishNewEntity() (tea.Model, tea.Cmd) {
	if m.newEntityKeyKind == "" {
		m.err = errNewEntityKindRequired
		m.screen = screenBrowse
		return m, nil
	}
	pe := model.PathElement{Kind: m.newEntityKeyKind}
	if m.newEntityKeyID != "" {
		id, err := parseInt64(m.newEntityKeyID)
		if err != nil {
			m.err = err
			m.screen = screenBrowse
			return m, nil
		}
		pe.ID = id
	} else {
		pe.Name = m.newEntityKeyName
	}

	e := &model.Entity{
		Key:        &model.Key{NamespaceID: resolveNamespaceID(m.namespace), Path: []model.PathElement{pe}},
		Properties: map[string]model.Value{},
	}
	m.currentEntity = e
	m.detailPath.Reset()
	m.detailSelected = 0
	m.detailFilter = ""
	m.dirty.MarkDirty() // a brand new entity always needs an initial save
	m.screen = screenDetail
	return m, nil
}
