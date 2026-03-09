package keys

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Esc        key.Binding
	Enter      key.Binding
	ScrollDown key.Binding
	ScrollUp   key.Binding
	Refresh    key.Binding
	RefreshAll key.Binding
	Delete     key.Binding
	Filter     key.Binding

	CreateAndRunContainer key.Binding

	ContainerDelete    key.Binding
	ContainerStartStop key.Binding
	ContainerRestart   key.Binding

	NetworkDelete key.Binding

	PanelNext key.Binding
	PanelPrev key.Binding

	Prune key.Binding

	Help key.Binding
	Quit key.Binding
}

var navigation = key.NewBinding(
	key.WithKeys("↑↓←→"),
	key.WithHelp("↑↓←→", "navigation"),
)

var scroll = key.NewBinding(
	key.WithKeys("j/k"),
	key.WithHelp("j/k", "scroll"),
)

var panelNavigation = key.NewBinding(
	key.WithKeys("shift+→/shift+←"),
	key.WithHelp("shift+→/shift+←", "panels"),
)

func (k *KeyMap) navigationKeys() []key.Binding {
	return []key.Binding{navigation, scroll, panelNavigation, k.Filter}
}

var Keys = &KeyMap{
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "exit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "enter"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev section"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next section"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "scroll down"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	RefreshAll: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh all"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	CreateAndRunContainer: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "create and run container"),
	),
	ContainerDelete: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete container"),
	),
	ContainerStartStop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/stop container"),
	),
	ContainerRestart: key.NewBinding(
		key.WithKeys("ctrl+R"),
		key.WithHelp("ctrl+R", "restart container"),
	),
	NetworkDelete: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete network"),
	),
	PanelNext: key.NewBinding(
		key.WithKeys("shift+right"),
		key.WithHelp("shift+→", "next panel"),
	),
	PanelPrev: key.NewBinding(
		key.WithKeys("shift+left"),
		key.WithHelp("shift+←", "prev panel"),
	),
	Prune: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "prune"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ViewKeyMap implements help.KeyMap for list views.
type ViewKeyMap struct {
	short             []key.Binding
	full              [][]key.Binding
	contextualKeys    []key.Binding
	contextualEnabled bool
}

func (v *ViewKeyMap) ShortHelp() []key.Binding {
	if v.contextualEnabled {
		return v.contextualKeys
	}
	return v.short
}
func (v *ViewKeyMap) FullHelp() [][]key.Binding { return v.full }
func (v *ViewKeyMap) ToggleContextual(bindings []key.Binding) {
	v.contextualKeys = bindings
	v.contextualEnabled = true
}
func (v *ViewKeyMap) DisableContextual() {
	v.contextualEnabled = false
}

func (k KeyMap) ImageKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.Delete, k.CreateAndRunContainer, k.Prune, k.Filter},
			{k.Help, k.Quit},
		},
	}
}

func (k KeyMap) ContainerKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.ContainerDelete, k.ContainerStartStop, k.ContainerRestart, k.Prune},
			{k.Filter, k.Help, k.Quit},
		},
	}
}

func (k KeyMap) VolumeKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.Delete, k.Prune, k.Filter},
			{k.Help, k.Quit},
		},
	}
}

func (k KeyMap) NetworkKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.NetworkDelete, k.Prune, k.Filter},
			{k.Help, k.Quit},
		},
	}
}
