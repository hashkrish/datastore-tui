package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/datastore/query"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/panes"
)

func (m *Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := keymap.DefaultDetailKeyMap()

	switch {
	case key.Matches(msg, km.Quit):
		return m.exitDetailToBrowse()

	case key.Matches(msg, km.Help):
		m.prevScreen = screenDetail
		m.screen = screenHelp
		return m, nil

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

	case key.Matches(msg, km.Refresh):
		m.chordD.Reset()
		return m.startRefreshEntity()

	case key.Matches(msg, km.GoToKey):
		m.chordD.Reset()
		return m.followKeyProperty()

	case key.Matches(msg, km.ToggleBookmark):
		m.chordD.Reset()
		return m.toggleBookmark()

	case key.Matches(msg, km.ListBookmarks):
		m.chordD.Reset()
		return m.startBookmarkList()

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
	fe, ok := edit.NewFieldEditor(leaf, m.width)
	if !ok {
		return m, nil
	}
	m.fieldEditor = fe
	m.editingSegment = row.Segment
	m.screen = screenEditLeaf
	return m, fe.Form().Init()
}

// followKeyProperty implements "ctrl+]" in detail mode: if the selected row
// is a Key-typed scalar property, looks up and opens the entity it points
// at.
func (m *Model) followKeyProperty() (tea.Model, tea.Cmd) {
	scope, rows, err := m.currentScope()
	if err != nil || len(rows) == 0 {
		return m, nil
	}
	row := rows[m.detailSelected]
	if row.IsContainer {
		return m, nil
	}
	leaf, err := panes.LeafRowValue(scope, row)
	if err != nil || leaf.Kind != model.KindKey || leaf.KeyValue == nil {
		return m, nil
	}
	return m.goToKey(leaf.KeyValue)
}

// goToKey opens the entity identified by key (fetching it first), deferring
// to the unsaved-edit confirmation used elsewhere in detail mode if the
// current entity is dirty.
func (m *Model) goToKey(key *model.Key) (tea.Model, tea.Cmd) {
	if key == nil {
		return m, nil
	}
	if m.dirty.Dirty() {
		m.prevScreen = m.screen
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			mm.dirty.Reset()
			mm.status = "loading..."
			return mm, lookupKeyCmd(mm.client, key)
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	m.status = "loading..."
	return m, lookupKeyCmd(m.client, key)
}

// startRefreshEntity implements "r" in detail mode: reloads the current
// entity from the database. Local edits, if any, would otherwise be
// silently overwritten by whatever the reload fetches, so a dirty entity is
// held behind a confirm (screenConfirmRefresh) instead of reloading
// straight away; a clean one reloads immediately.
func (m *Model) startRefreshEntity() (tea.Model, tea.Cmd) {
	if m.currentEntity == nil {
		return m, nil
	}
	key := m.currentEntity.Key
	if m.dirty.Dirty() {
		m.prevScreen = screenDetail
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			mm.status = "refreshing..."
			return mm, lookupKeyCmd(mm.client, key)
		}
		m.screen = screenConfirmRefresh
		return m, nil
	}
	m.status = "refreshing..."
	return m, lookupKeyCmd(m.client, key)
}

// openEntity opens e in the detail view directly, without a Lookup round
// trip — used once an entity has already been fetched (a keyLookupMsg
// result, or a bookmark preview picked from the picker).
func (m *Model) openEntity(e *model.Entity) (tea.Model, tea.Cmd) {
	m.status = ""
	m.currentEntity = e
	m.namespace = namespaceLabel(e.Key.NamespaceID)
	m.detailPath.Reset()
	m.detailSelected = 0
	m.detailFilter = ""
	m.dirty.Reset()
	m.screen = screenDetail
	return m, nil
}

// toggleBookmark implements "ctrl+b" in detail mode: bookmarking or
// unbookmarking the entity currently open, persisted to disk.
func (m *Model) toggleBookmark() (tea.Model, tea.Cmd) {
	if m.currentEntity == nil || m.currentEntity.Key == nil {
		return m, nil
	}
	key := m.currentEntity.Key
	if idx := bookmarkIndex(m.bookmarks, key); idx >= 0 {
		m.bookmarks = append(m.bookmarks[:idx], m.bookmarks[idx+1:]...)
		m.status = "bookmark removed"
	} else {
		label := entityLabel(m.currentEntity)
		if m.namespace != "" && m.namespace != query.DefaultNamespaceLabel {
			label = m.namespace + "/" + label
		}
		m.bookmarks = append(m.bookmarks, bookmark{Label: label, Key: key})
		m.status = "bookmarked"
	}
	if err := saveBookmarks(m.bookmarks); err != nil {
		m.err = err
	}
	return m, nil
}

// startBookmarkList implements "ctrl+l": opens a picker over the saved
// bookmarks, from either browse or detail mode. All bookmarked entities are
// fetched in one batched Lookup so the picker can preview each one live as
// the user moves the selection, rather than only showing its label.
func (m *Model) startBookmarkList() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 {
		m.status = "no bookmarks"
		return m, nil
	}
	m.bookmarkCursor = 0
	m.bookmarkEntities = nil
	m.prevScreen = m.screen
	m.screen = screenBookmarks
	return m, lookupBookmarksCmd(m.client, m.bookmarks)
}

// updateBookmarkList drives the bookmark picker: j/k move the highlighted
// bookmark (whose preview panel updates as a side effect of viewBookmarks
// reading m.bookmarkCursor), enter opens it, esc/q cancels back out.
func (m *Model) updateBookmarkList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.bookmarkCursor < len(m.bookmarks)-1 {
			m.bookmarkCursor++
		}
		return m, nil
	case "k", "up":
		if m.bookmarkCursor > 0 {
			m.bookmarkCursor--
		}
		return m, nil
	case "enter", "l", "right":
		return m.openBookmark(m.bookmarkCursor)
	case "esc", "q":
		m.screen = m.prevScreen
		return m, nil
	}
	return m, nil
}

// openBookmark opens the idx'th bookmark. If its entity was already fetched
// for the picker's preview, it's opened directly (no extra round trip);
// otherwise (a lookup still in flight, or the bookmarked entity no longer
// exists) it falls back to a fresh Lookup via goToKey.
func (m *Model) openBookmark(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.bookmarks) {
		return m, nil
	}
	key := m.bookmarks[idx].Key
	e, ok := m.bookmarkEntities[key.String()]
	if !ok || e == nil {
		return m.goToKey(key)
	}
	if m.dirty.Dirty() {
		m.prevScreen = m.screen
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			return mm.openEntity(e)
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	return m.openEntity(e)
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
		if fe, ok := edit.NewFieldEditor(zero, m.width); ok {
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
