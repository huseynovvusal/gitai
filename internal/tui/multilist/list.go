package multilist

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

// -- Key Bindings for Help Menu --

type additionalKeyMap struct {
	Toggle    key.Binding
	SelectAll key.Binding
	Invert    key.Binding
}

func newAdditionalKeyMap() additionalKeyMap {
	return additionalKeyMap{
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		Invert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "invert selection"),
		),
	}
}

// -- Model --

type Item struct {
	Value    string
	Selected bool
}

func (i Item) FilterValue() string { return i.Value }
func (i Item) Title() string       { return i.Value }
func (i Item) Description() string { return "" }

type Model struct {
	List list.Model
	keys additionalKeyMap
}

type Option func(*list.Model)

func WithHeight(h int) Option {
	return func(l *list.Model) {
		l.SetHeight(h)
	}
}

func WithWidth(w int) Option {
	return func(l *list.Model) {
		l.SetWidth(w)
	}
}

func WithPaginatorType(p paginator.Type) Option {
	return func(l *list.Model) {
		l.Paginator.Type = p
	}
}

func WithStatusBar(show bool) Option {
	return func(l *list.Model) {
		l.SetShowStatusBar(show)
	}
}

func WithTitleStyle(style lipgloss.Style) Option {
	return func(l *list.Model) {
		l.Styles.Title = style
	}
}

func WithSelected(selected []string) Option {
	return func(l *list.Model) {
		selectedMap := make(map[string]bool)
		for _, s := range selected {
			selectedMap[s] = true
		}

		items := l.Items()
		for i, itm := range items {
			item := itm.(Item)
			if selectedMap[item.Value] {
				item.Selected = true
				items[i] = item
			}
		}
		l.SetItems(items)
	}
}

// New creates a list. The default title is empty, the default height is 20.
// Pass options to override defaults.
func New(data []string, title string, opts ...Option) Model {
	items := make([]list.Item, len(data))
	for i, v := range data {
		items[i] = Item{Value: v, Selected: false}
	}

	l := list.New(items, delegate{}, 40, 20)
	l.Title = title

	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Paginator.Type = paginator.Arabic

	l.Styles.Title = shared.HeaderStyle
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	l.Styles.NoItems = lipgloss.NewStyle().Margin(1, 2)

	for _, opt := range opts {
		opt(&l)
	}

	keys := newAdditionalKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Toggle, keys.SelectAll, keys.Invert}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Toggle, keys.SelectAll, keys.Invert}
	}

	return Model{List: l, keys: keys}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.List.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, m.keys.Toggle) {
			return m.toggleCurrent()
		}

		if key.Matches(msg, m.keys.SelectAll) {
			return m.toggleAll()
		}

		if key.Matches(msg, m.keys.Invert) {
			return m.toggleInvert()
		}
	}

	var cmd tea.Cmd

	m.List, cmd = m.List.Update(msg)

	return m, cmd
}

func (m Model) View() string {
	return m.List.View()
}

func (m Model) SetSize(width, height int) {
	m.List.SetSize(width, height)
}

func (m Model) GetSelected() []string {
	var selected []string

	for _, i := range m.List.Items() {
		item := i.(Item)
		if item.Selected {
			selected = append(selected, item.Value)
		}
	}

	return selected
}

func (m Model) toggleCurrent() (Model, tea.Cmd) {
	// 1. Get the item currently selected in the view
	selectedItem := m.List.SelectedItem()
	if selectedItem == nil {
		return m, nil
	}

	item := selectedItem.(Item)
	item.Selected = !item.Selected

	globalIdx := -1

	allWaitItems := m.List.Items()
	for i, itm := range allWaitItems {
		if itm.(Item).Value == item.Value {
			globalIdx = i

			break
		}
	}

	if globalIdx != -1 {
		return m, m.List.SetItem(globalIdx, item)
	}

	return m, nil
}

func (m Model) toggleAll() (Model, tea.Cmd) {
	visibleItems := m.List.VisibleItems()
	if len(visibleItems) == 0 {
		return m, nil
	}

	targetState := true
	allVisibleSelected := true

	for _, i := range visibleItems {
		if !i.(Item).Selected {
			allVisibleSelected = false

			break
		}
	}

	if allVisibleSelected {
		targetState = false
	}

	visibleMap := make(map[string]bool)
	for _, i := range visibleItems {
		visibleMap[i.(Item).Value] = true
	}

	fullList := m.List.Items()

	var cmds []tea.Cmd

	for idx, i := range fullList {
		item := i.(Item)

		// Only toggle if this item is currently visible
		if visibleMap[item.Value] {
			if item.Selected != targetState {
				item.Selected = targetState
				cmds = append(cmds, m.List.SetItem(idx, item))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) toggleInvert() (Model, tea.Cmd) {
	visibleItems := m.List.VisibleItems()
	if len(visibleItems) == 0 {
		return m, nil
	}

	visibleMap := make(map[string]bool)
	for _, i := range visibleItems {
		visibleMap[i.(Item).Value] = true
	}

	fullList := m.List.Items()
	var cmds []tea.Cmd

	for idx, i := range fullList {
		item := i.(Item)
		if visibleMap[item.Value] {
			item.Selected = !item.Selected
			cmds = append(cmds, m.List.SetItem(idx, item))
		}
	}

	return m, tea.Batch(cmds...)
}

type delegate struct{}

func (d delegate) Height() int                             { return 1 }
func (d delegate) Spacing() int                            { return 0 }
func (d delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d delegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	checked := "[ ]"
	if i.Selected {
		checked = "[x]"
	}

	str := fmt.Sprintf("%s %s", checked, i.Value)

	if index == m.Index() {
		fmt.Fprint(w, shared.SelectedStyle.Render("> "+str))
	} else {
		fmt.Fprint(w, "  "+shared.FileStyle.Render(str))
	}
}
