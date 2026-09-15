package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/datastore/query"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
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
		m.chordY.Reset()
		if m.detailSelected > 0 {
			m.detailSelected--
		}
		return m, nil

	case key.Matches(msg, km.Down):
		m.chordD.Reset()
		m.chordY.Reset()
		_, rows, err := m.currentScope()
		if err == nil && m.detailSelected < len(rows)-1 {
			m.detailSelected++
		}
		return m, nil

	case key.Matches(msg, km.HalfPageDown):
		m.chordD.Reset()
		m.chordY.Reset()
		_, rows, err := m.currentScope()
		if err == nil {
			m.detailSelected = min(m.detailSelected+m.halfPage(), max(0, len(rows)-1))
		}
		return m, nil

	case key.Matches(msg, km.HalfPageUp):
		m.chordD.Reset()
		m.chordY.Reset()
		m.detailSelected = max(0, m.detailSelected-m.halfPage())
		return m, nil

	case key.Matches(msg, km.Expand):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.detailExpand()

	case key.Matches(msg, km.Retype):
		m.chordD.Reset()
		m.chordY.Reset()
		if m.blockReadOnly() {
			return m, nil
		}
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
		if m.blockReadOnly() {
			return m, nil
		}
		m.status = "saving..."
		return m, saveEntityCmd(m.client, m.id, m.currentEntity)

	case key.Matches(msg, km.AddItem):
		if m.blockReadOnly() {
			return m, nil
		}
		return m.startAddItem()

	case key.Matches(msg, km.Refresh):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.startRefreshEntity()

	case key.Matches(msg, km.GoToKey):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.followKeyProperty()

	case key.Matches(msg, km.ToggleBookmark):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.toggleBookmark()

	case key.Matches(msg, km.ListBookmarks):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.startBookmarkList()

	case key.Matches(msg, km.FindReferences):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.startRefQueryFromDetail(false)

	case key.Matches(msg, km.FindReferencesAdd):
		m.chordD.Reset()
		m.chordY.Reset()
		return m.startRefQueryFromDetail(true)

	case key.Matches(msg, km.DeleteItem):
		if m.chordD.Complete('d') {
			if m.blockReadOnly() {
				return m, nil
			}
			return m.confirmDeleteItem()
		}
		m.chordD.Arm('d')
		return m, nil

	case key.Matches(msg, km.Yank):
		if m.chordY.Complete('y') {
			return m.yankSelectedValue()
		}
		m.chordY.Arm('y')
		return m, nil

	default:
		m.chordD.Reset()
		m.chordY.Reset()
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
	return m.goToKey(leaf.KeyValue, true)
}

// entityFrame is one entry of entityStack: the detail-view state to restore
// when backing out of an entity reached via followKeyProperty.
type entityFrame struct {
	entity     *model.Entity
	namespace  string
	detailPath nav.DetailPath
	selected   int
	filter     string
}

// pushEntityFrame saves the currently open entity's detail-view state onto
// entityStack, so detailBack can return to it later.
func (m *Model) pushEntityFrame() {
	m.entityStack = append(m.entityStack, entityFrame{
		entity:     m.currentEntity,
		namespace:  m.namespace,
		detailPath: m.detailPath,
		selected:   m.detailSelected,
		filter:     m.detailFilter,
	})
}

// popEntityFrame restores the most recently pushed frame, reporting whether
// there was one.
func (m *Model) popEntityFrame() bool {
	if len(m.entityStack) == 0 {
		return false
	}
	last := len(m.entityStack) - 1
	frame := m.entityStack[last]
	m.entityStack = m.entityStack[:last]

	m.currentEntity = frame.entity
	m.namespace = frame.namespace
	m.detailPath = frame.detailPath
	m.detailSelected = frame.selected
	m.detailFilter = frame.filter
	m.dirty.Reset()
	return true
}

// goToKey opens the entity identified by key (fetching it first), deferring
// to the unsaved-edit confirmation used elsewhere in detail mode if the
// current entity is dirty. When pushHistory is true (following a Key-typed
// property via "ctrl+]"), the current entity's detail-view state is saved
// first so detailBack can return to it once the user backs out of the
// followed entity's root — see entityFrame.
func (m *Model) goToKey(key *model.Key, pushHistory bool) (tea.Model, tea.Cmd) {
	if key == nil {
		return m, nil
	}
	if m.dirty.Dirty() {
		m.prevScreen = m.screen
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			if pushHistory {
				mm.pushEntityFrame()
			}
			mm.dirty.Reset()
			mm.status = "loading..."
			return mm, lookupKeyCmd(mm.client, mm.id, key)
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	if pushHistory {
		m.pushEntityFrame()
	}
	m.status = "loading..."
	return m, lookupKeyCmd(m.client, m.id, key)
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
			return mm, lookupKeyCmd(mm.client, mm.id, key)
		}
		m.screen = screenConfirmRefresh
		return m, nil
	}
	m.status = "refreshing..."
	return m, lookupKeyCmd(m.client, m.id, key)
}

// openEntity opens e in the detail view directly, without a Lookup round
// trip — used once an entity has already been fetched (a keyLookupMsg
// result, or a bookmark preview picked from the picker). t is the tab this
// applies to: the one that originated the lookup, which may not be the
// currently active tab by the time the result arrives.
func (m *Model) openEntity(t *tab, e *model.Entity) (tea.Model, tea.Cmd) {
	t.status = ""
	t.currentEntity = e
	t.namespace = namespaceLabel(e.Key.NamespaceID)
	t.detailPath.Reset()
	t.detailSelected = 0
	t.detailFilter = ""
	t.dirty.Reset()
	t.detailOrigin = screenBrowse
	t.screen = screenDetail
	return m, nil
}

// toggleBookmark implements "ctrl+b" in detail mode: bookmarking or
// unbookmarking the entity currently open, persisted to disk.
func (m *Model) toggleBookmark() (tea.Model, tea.Cmd) {
	return m.toggleBookmarkFor(m.currentEntity)
}

// toggleBookmarkFor bookmarks or unbookmarks e, persisted to disk. Shared by
// "ctrl+b" in detail mode (e is the open entity) and browse mode (e is
// whichever entity is highlighted in the Entity column), so a bookmark can
// be added without first opening the entity.
func (m *Model) toggleBookmarkFor(e *model.Entity) (tea.Model, tea.Cmd) {
	if e == nil || e.Key == nil {
		return m, nil
	}
	key := e.Key
	if idx := bookmarkIndex(m.bookmarks, key); idx >= 0 {
		m.bookmarks = append(m.bookmarks[:idx], m.bookmarks[idx+1:]...)
		m.status = "bookmark removed"
	} else {
		label := entityLabel(e)
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

// updateBookmarkList drives the bookmark picker: j/k (or ctrl+n/ctrl+p) move
// the highlighted bookmark (whose preview panel updates as a side effect of
// viewBookmarks reading m.bookmarkCursor), enter opens it, dd deletes it (no
// confirmation — bookmarks are just local pointers, not Datastore data),
// esc/q cancels back out.
func (m *Model) updateBookmarkList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "d" {
		m.chordD.Reset()
	}
	switch msg.String() {
	case "j", "down", "ctrl+n":
		if m.bookmarkCursor < len(m.bookmarks)-1 {
			m.bookmarkCursor++
		}
		return m, nil
	case "k", "up", "ctrl+p":
		if m.bookmarkCursor > 0 {
			m.bookmarkCursor--
		}
		return m, nil
	case "enter", "l", "right":
		return m.openBookmark(m.bookmarkCursor)
	case "d":
		if m.chordD.Complete('d') {
			return m.deleteSelectedBookmark()
		}
		m.chordD.Arm('d')
		return m, nil
	case "C":
		if len(m.bookmarks) == 0 {
			return m, nil
		}
		m.screen = screenConfirmClearBookmarks
		return m, nil
	case "esc", "q":
		m.screen = m.prevScreen
		return m, nil
	}
	return m, nil
}

// deleteSelectedBookmark implements "dd" in the bookmark picker: removes the
// highlighted bookmark and persists the change immediately, with no
// confirmation prompt (unlike entity/array-item deletion, this only affects
// a local pointer, not data in Datastore). Closes the picker if that was the
// last bookmark.
func (m *Model) deleteSelectedBookmark() (tea.Model, tea.Cmd) {
	if m.bookmarkCursor < 0 || m.bookmarkCursor >= len(m.bookmarks) {
		return m, nil
	}
	removed := m.bookmarks[m.bookmarkCursor]
	m.bookmarks = append(m.bookmarks[:m.bookmarkCursor:m.bookmarkCursor], m.bookmarks[m.bookmarkCursor+1:]...)
	if err := saveBookmarks(m.bookmarks); err != nil {
		m.err = err
	}
	if removed.Key != nil {
		delete(m.bookmarkEntities, removed.Key.String())
	}
	if m.bookmarkCursor >= len(m.bookmarks) {
		m.bookmarkCursor = len(m.bookmarks) - 1
	}
	m.status = "bookmark deleted"
	if len(m.bookmarks) == 0 {
		m.screen = m.prevScreen
	}
	return m, nil
}

// updateConfirmClearBookmarks drives the "C" confirmation prompt. It always
// returns to the bookmark picker (never m.prevScreen, which still holds the
// screen the picker itself was opened from and must survive this nested
// confirm untouched, so "esc"/"q" from the picker keeps working afterward).
func (m *Model) updateConfirmClearBookmarks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "y" {
		return m.clearAllBookmarks()
	}
	m.screen = screenBookmarks
	return m, nil
}

// clearAllBookmarks implements "C" (with confirmation) in the bookmark
// picker: removes every saved bookmark and returns to the (now empty)
// picker.
func (m *Model) clearAllBookmarks() (tea.Model, tea.Cmd) {
	m.bookmarks = nil
	m.bookmarkEntities = nil
	m.bookmarkCursor = 0
	if err := saveBookmarks(m.bookmarks); err != nil {
		m.err = err
	}
	m.status = "all bookmarks cleared"
	m.screen = screenBookmarks
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
	// Opening a bookmark always starts a fresh navigation root, unrelated to
	// wherever "ctrl+]" chasing had gotten to before ctrl+l was pressed.
	m.entityStack = nil
	key := m.bookmarks[idx].Key
	e, ok := m.bookmarkEntities[key.String()]
	if !ok || e == nil {
		return m.goToKey(key, false)
	}
	if m.dirty.Dirty() {
		m.prevScreen = m.screen
		m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
			return mm.openEntity(mm.tab, e)
		}
		m.screen = screenConfirmQuit
		return m, nil
	}
	return m.openEntity(m.tab, e)
}

// detailBack implements "h"/"esc" in detail mode: stepping out one nesting
// level; at an entity's root, popping back to whichever entity a "ctrl+]"
// follow was chased from (see entityFrame), or — with no such entity —
// leaving the detail view for browse mode.
func (m *Model) detailBack() (tea.Model, tea.Cmd) {
	if m.detailPath.Depth() > 0 {
		m.detailPath.Pop()
		m.detailSelected = 0
		m.detailFilter = ""
		return m, nil
	}
	if len(m.entityStack) > 0 {
		if m.dirty.Dirty() {
			m.prevScreen = screenDetail
			m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
				mm.popEntityFrame()
				return mm, nil
			}
			m.screen = screenConfirmQuit
			return m, nil
		}
		m.popEntityFrame()
		return m, nil
	}
	return m.exitDetailToBrowse()
}

// exitDetailToBrowse implements "q"/"esc" out of the detail view's root (and
// "h"/"esc" via detailBack, once entityStack is empty): returns to whichever
// screen opened this detail view — browse, or table view if it was opened
// via "enter" on a table-view row (see detailOrigin, openEntityDetailFromTable).
func (m *Model) exitDetailToBrowse() (tea.Model, tea.Cmd) {
	m.entityStack = nil
	if !m.dirty.Dirty() {
		m.currentEntity = nil
		m.screen = m.detailOrigin
		return m, nil
	}
	m.prevScreen = screenDetail
	origin := m.detailOrigin
	m.confirmYes = func(mm *Model) (tea.Model, tea.Cmd) {
		mm.currentEntity = nil
		mm.dirty.Reset()
		mm.screen = origin
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
	m.entityStack = nil
	m.detailPath.Reset()
	m.detailSelected = 0
	m.detailFilter = ""
	m.dirty.MarkDirty() // a brand new entity always needs an initial save
	m.detailOrigin = screenBrowse
	m.screen = screenDetail
	return m, nil
}
