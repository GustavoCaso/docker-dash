package message

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/ui/components/form"
)

// ShowBannerMsg is sent by components to display a banner notification.
type ShowBannerMsg struct {
	Message      string
	IsError      bool
	ClearTimeout time.Duration
}

func NewShowBannerMsg(message string, isError bool, clearTimeOut time.Duration) ShowBannerMsg {
	return ShowBannerMsg{
		Message:      message,
		IsError:      isError,
		ClearTimeout: clearTimeOut,
	}
}

// ShowSpinnerMsg asks the app to start showing the global spinner for the
// given section or operation ID.
type ShowSpinnerMsg struct {
	ID   string
	Text string
}

// CancelSpinnerMsg asks the app to stop showing the global spinner for the
// given section or operation ID.
type CancelSpinnerMsg struct {
	ID string
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

// ShowConfirmationMsg triggers the confirmation modal overlay.
// OnConfirm is the tea.Cmd to run when the user presses y.
type ShowConfirmationMsg struct {
	Title     string
	Body      string
	OnConfirm tea.Cmd
}

type ShowFormMsg struct {
	Form *form.Model
}
