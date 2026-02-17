package components

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestStatusBarNewDefaults(t *testing.T) {
	sb := NewStatusBar()
	if sb == nil {
		t.Fatal("NewStatusBar() returned nil")
	}

	// View should not panic and can return empty string
	got := sb.View()
	_ = got
}

func TestStatusBarSetBindings(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(80, 1)
	sb.SetBindings([]key.Binding{
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	})

	got := sb.View()
	if got == "" {
		t.Error("View() returned empty string after setting bindings")
	}
}

func TestStatusBarSetSize(t *testing.T) {
	sb := NewStatusBar()
	sb.SetSize(40, 1)
	sb.SetBindings([]key.Binding{
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	})

	// Should not panic with a narrow width
	got := sb.View()
	_ = got
}
