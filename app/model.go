// Package app wires the datastore client, navigation state, panes, and edit
// forms into a single bubbletea Model.
package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
	"github.com/krishnan/datastore-tui/ui/edit"
	"github.com/krishnan/datastore-tui/ui/keymap"
	"github.com/krishnan/datastore-tui/ui/nav"
	"github.com/krishnan/datastore-tui/ui/panes"
)

// screen selects which full-screen view is active.
type screen int

const (
	screenBrowse screen = iota
	screenDetail
	screenFilterInput
	screenDetailFilterInput
	screenEditLeaf
	screenNewItemType
	screenNewItemValue
	screenNewEntityKey
	screenConfirmDeleteEntity
	screenConfirmDeleteItem
	screenConfirmQuit
	screenHelp
)

// newItemTarget selects what the type-select + value-edit flow (triggered
// either by "o" on an array, or by opening a null leaf for retyping) does
// once the user finishes picking a type and a value.
type newItemTarget int

const (
	targetArrayAppend newItemTarget = iota
	targetLeafRetype
)

// Model is the top-level bubbletea model.
type Model struct {
	client    *client.Client
	namespace string // resolved (non-label) namespace backing the current kind/entity lists

	nav        *nav.State
	detailPath nav.DetailPath

	currentEntity *model.Entity // non-nil while a detail view is open
	dirty         edit.Tracker

	screen         screen
	prevScreen     screen // screen to return to after a confirm/help overlay
	detailSelected int
	detailFilter   string // substring filter over the current scope's rows

	filterInput textinput.Model

	fieldEditor    *edit.FieldEditor
	editingSegment nav.PropSegment // which row's leaf value fieldEditor is editing

	newItemKind      model.ValueKind
	newItemTarget    newItemTarget
	retypeOriginal   model.Value // pre-retype value, reused if the same type is re-picked
	newItemTypeForm  *huh.Form
	newEntityKeyForm *huh.Form
	newEntityKeyKind string
	newEntityKeyID   string
	newEntityKeyName string

	chordG keymap.Chord
	chordD keymap.Chord

	confirmYes func(*Model) (tea.Model, tea.Cmd)

	width, height int
	status        string
	err           error
}

// New builds a fresh Model against c.
func New(c *client.Client) *Model {
	fi := textinput.New()
	fi.Prompt = "/"
	return &Model{
		client:      c,
		nav:         nav.NewState(),
		filterInput: fi,
	}
}

func (m *Model) Init() tea.Cmd {
	return loadNamespacesCmd(m.client)
}

func (m *Model) currentScope() (model.Value, []panes.DetailRow, error) {
	scope, err := panes.GetValueAtPath(m.currentEntity, m.detailPath.Segments())
	if err != nil {
		return model.Value{}, nil, err
	}
	rows, err := panes.BuildRows(scope)
	if err != nil {
		return scope, nil, err
	}
	return scope, filterDetailRows(rows, m.detailFilter), nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case namespacesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.nav.SetNamespaces(msg.namespaces)
		return m, nil

	case kindsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.nav.SetKinds(msg.kinds)
		return m, nil

	case entitiesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.nav.SetEntitiesPage(msg.page, msg.appendPage)
		return m, nil

	case entitySavedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.dirty.Reset()
		m.status = "saved"
		return m, nil

	case entityDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.status = "deleted"
		ns, _ := m.nav.SelectedNamespace()
		kind, _ := m.nav.SelectedKind()
		return m, loadEntitiesCmd(m.client, ns, kind, "", false)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// huh.Form's Next/Prev/Submit key handling queues an internal follow-up
	// tea.Cmd (e.g. to advance groups); the resulting message isn't a
	// tea.KeyMsg, so it must be forwarded here rather than in handleKey, or
	// the form would visibly "eat" the keystroke and never actually advance.
	switch m.screen {
	case screenEditLeaf, screenNewItemValue:
		return m.updateEditLeaf(msg)
	case screenNewItemType:
		return m.updateNewItemType(msg)
	case screenNewEntityKey:
		return m.updateNewEntityKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	switch m.screen {
	case screenBrowse:
		return m.updateBrowse(msg)
	case screenFilterInput:
		return m.updateFilterInput(msg)
	case screenDetailFilterInput:
		return m.updateDetailFilterInput(msg)
	case screenDetail:
		return m.updateDetail(msg)
	case screenEditLeaf:
		return m.updateEditLeaf(msg)
	case screenNewItemType:
		return m.updateNewItemType(msg)
	case screenNewItemValue:
		return m.updateEditLeaf(msg) // same form-driving logic, different completion handler below
	case screenNewEntityKey:
		return m.updateNewEntityKey(msg)
	case screenConfirmDeleteEntity, screenConfirmDeleteItem, screenConfirmQuit:
		return m.updateConfirm(msg)
	case screenHelp:
		m.screen = m.prevScreen
		return m, nil
	}
	return m, nil
}

func (m *Model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	body := m.viewBody()
	statusLine := m.viewStatus()
	return lipgloss.JoinVertical(lipgloss.Left, body, statusLine)
}

func (m *Model) viewBody() string {
	contentHeight := m.height - 1
	switch m.screen {
	case screenFilterInput:
		return panes.RenderBrowse(m.nav, m.width, contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetailFilterInput:
		return m.viewDetailScreen(contentHeight-1) + "\n" + m.filterInput.View()
	case screenDetail, screenEditLeaf, screenNewItemType, screenNewItemValue, screenConfirmDeleteItem:
		return m.viewDetailScreen(contentHeight)
	case screenNewEntityKey:
		return m.newEntityKeyForm.View()
	case screenConfirmDeleteEntity:
		return "Delete entity " + entityLabel(m.nav.SelectedEntity()) + "? Press y to confirm, any other key to cancel."
	case screenConfirmQuit:
		return "Unsaved changes will be lost. Press y to quit anyway, any other key to cancel."
	case screenHelp:
		return helpText()
	default:
		return panes.RenderBrowse(m.nav, m.width, contentHeight)
	}
}

func (m *Model) viewDetailScreen(height int) string {
	breadcrumb := panes.FormatBreadcrumb(m.namespace, m.currentEntity.Key, &m.detailPath)
	_, rows, err := m.currentScope()
	if err != nil {
		return breadcrumb + "\n\n" + err.Error()
	}
	base := panes.RenderDetail(breadcrumb, rows, m.detailSelected, m.width, height-4)

	switch m.screen {
	case screenEditLeaf, screenNewItemValue:
		if m.fieldEditor != nil {
			return base + "\n" + m.fieldEditor.Form().View()
		}
	case screenNewItemType:
		return base + "\n" + m.newItemTypeForm.View()
	case screenConfirmDeleteItem:
		return base + "\n\nDelete this item? Press y to confirm, any other key to cancel."
	}
	return base
}

func (m *Model) viewStatus() string {
	style := lipgloss.NewStyle().Padding(0, 1)
	if m.err != nil {
		return style.Foreground(lipgloss.Color("196")).Render("error: " + m.err.Error())
	}
	dirtyMark := ""
	if m.dirty.Dirty() {
		dirtyMark = " [modified]"
	}
	help := "j/k move  ctrl+u/d half page  l/enter open  h back  t retype  / filter  o new  dd delete  ctrl+s save  ? help  q quit"
	if m.status != "" {
		return style.Render(m.status + dirtyMark + "  " + help)
	}
	return style.Render(help + dirtyMark)
}

func entityLabel(e *model.Entity) string {
	if e == nil {
		return ""
	}
	return e.Key.Last().String()
}

func helpText() string {
	return `Datastore TUI — Help

Browse mode:
  j/k, up/down   move
  ctrl+u/ctrl+d  half page up/down
  h/l, left/right back / drill in
  gg / G         jump to top / bottom of column
  /              filter the focused column
  enter          open selected entity
  o              new entity (Entity column)
  dd             delete selected entity (Entity column)
  R              refresh focused column
  q, ctrl+c      quit
  (right pane previews the highlighted entity's properties)

Detail mode (viewing/editing an entity):
  j/k            move between properties
  ctrl+u/ctrl+d  half page up/down
  l/enter        expand array/embedded entity, or edit a scalar
                 (a null property has nothing to edit this way)
  t              change the selected property's data type
  h/esc          back out one level, or to browse
  /              filter the current scope's properties
  o              add array item (when viewing an array)
  dd             delete array item (when viewing an array)
  ctrl+s         save pending edits
  q/esc          back to browse (prompts if unsaved)

Press any key to close this help.`
}
