package message

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ShowBannerMsg is sent by components to display a banner notification.
type ShowBannerMsg struct {
	Message string
	IsError bool
}

// BubbleUpMsg is sent by components to communicate to other components
// An example is the image components create a new container and want sthe container component to
// refresh
// OnlyActive false propagates message to all components.
type BubbleUpMsg struct {
	KeyMsg     tea.KeyMsg
	OnlyActive bool
}

// AddContextualKeyBindingsMsg is sent by components extend the status component with extra help info.
type AddContextualKeyBindingsMsg struct {
	Bindings []key.Binding
}

// ClearContextualKeyBindingsMsg is sent by components to extra help info.
type ClearContextualKeyBindingsMsg struct {
}
