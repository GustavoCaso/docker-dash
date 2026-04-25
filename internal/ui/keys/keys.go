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
	Space      key.Binding
	Refresh    key.Binding
	RefreshAll key.Binding
	Delete     key.Binding
	Filter     key.Binding

	CreateAndRunContainer key.Binding
	PullImage             key.Binding
	PullImageUpdate       key.Binding

	ContainerDelete       key.Binding
	ContainerStartStop    key.Binding
	ContainerRestart      key.Binding
	ContainerPauseUnpause key.Binding

	ComposeUp        key.Binding
	ComposeDown      key.Binding
	ComposeStartStop key.Binding
	ComposeRestart   key.Binding

	NetworkDelete key.Binding

	PanelNext key.Binding
	PanelPrev key.Binding

	Prune key.Binding

	SystemInfo key.Binding
	Help       key.Binding
	Quit       key.Binding
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
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
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
	PullImage: key.NewBinding(
		key.WithKeys("+"),
		key.WithHelp("+", "pull image"),
	),
	PullImageUpdate: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "pull update"),
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
	ContainerPauseUnpause: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "pause/unpause container"),
	),
	ComposeUp: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "compose up"),
	),
	ComposeDown: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "compose down"),
	),
	ComposeStartStop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/stop project"),
	),
	ComposeRestart: key.NewBinding(
		key.WithKeys("ctrl+R"),
		key.WithHelp("ctrl+R", "restart project"),
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
	SystemInfo: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "system info"),
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
			{k.PullImage, k.PullImageUpdate},
			{k.Help, k.Quit, k.SystemInfo},
		},
		contextualKeys: []key.Binding{},
	}
}

func (k KeyMap) ContainerKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.ContainerDelete, k.ContainerStartStop, k.ContainerRestart, k.Prune},
			{k.ContainerPauseUnpause, k.Filter},
			{k.Help, k.Quit, k.SystemInfo},
		},
		contextualKeys: []key.Binding{},
	}
}

func (k KeyMap) VolumeKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.Delete, k.Prune, k.Filter},
			{k.Help, k.Quit, k.SystemInfo},
		},
		contextualKeys: []key.Binding{},
	}
}

func (k KeyMap) NetworkKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.NetworkDelete, k.Prune, k.Filter},
			{k.Help, k.Quit, k.SystemInfo},
		},
		contextualKeys: []key.Binding{},
	}
}

func (k KeyMap) ComposeKeyMap() *ViewKeyMap {
	return &ViewKeyMap{
		short: k.navigationKeys(),
		full: [][]key.Binding{
			{k.Left, k.Right, k.PanelNext, k.PanelPrev},
			{k.Up, k.Down, k.ScrollUp, k.ScrollDown},
			{k.ComposeUp, k.ComposeDown, k.ComposeStartStop, k.ComposeRestart},
			{k.Refresh, k.Filter},
			{k.Help, k.Quit, k.SystemInfo},
		},
		contextualKeys: []key.Binding{},
	}
}
