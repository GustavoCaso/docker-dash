package keys

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Esc        key.Binding
	Enter      key.Binding
	ScrollDown key.Binding
	ScrollUp   key.Binding
	Refresh    key.Binding
	RefreshAll key.Binding
	SwitchTab  key.Binding
	Delete     key.Binding
	FileTree   key.Binding

	ImageLayers           key.Binding
	CreateAndRunContainer key.Binding

	ContainerInfo      key.Binding
	ContainerDelete    key.Binding
	ContainerLogs      key.Binding
	ContainerStartStop key.Binding
	ContainerRestart   key.Binding
	ContainerExec      key.Binding

	Help key.Binding
	Quit key.Binding
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
	SwitchTab: key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "change focus"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	FileTree: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "show files"),
	),

	ImageLayers: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "show layer"),
	),
	CreateAndRunContainer: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "run container"),
	),

	ContainerInfo: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "info"),
	),
	ContainerDelete: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete container"),
	),
	ContainerLogs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	ContainerStartStop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/stop container"),
	),
	ContainerRestart: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "restart container"),
	),
	ContainerExec: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "exec into container"),
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

func (k KeyMap) SidebadrBindings() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Refresh,
		k.RefreshAll,
		k.SwitchTab,
	}
}

func (k KeyMap) ImageBindings() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.ScrollUp,
		k.ScrollDown,
		k.Delete,
		k.ImageLayers,
		k.CreateAndRunContainer,
	}
}

func (k KeyMap) ContainerBindings() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.ScrollUp,
		k.ScrollDown,
		k.ContainerDelete,
		k.ContainerInfo,
		k.ContainerLogs,
		k.ContainerStartStop,
		k.ContainerRestart,
		k.ContainerExec,
		k.FileTree,
	}
}

func (k KeyMap) VolumeBindings() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.ScrollUp,
		k.ScrollDown,
		k.Delete,
		k.FileTree,
	}
}
