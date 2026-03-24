package form

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func newTestForm(t *testing.T) *Model {
	t.Helper()
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("name").Title("Name"),
		),
	)
	return New("Test Form", f, func(_ *huh.Form) tea.Cmd {
		return func() tea.Msg { return "callback-fired" }
	})
}

func TestNew(t *testing.T) {
	m := newTestForm(t)

	if m.title != "Test Form" {
		t.Errorf("expected title %q, got %q", "Test Form", m.title)
	}
	if m.callbackFired {
		t.Error("callbackFired should be false initially")
	}
	if m.form == nil {
		t.Error("form should not be nil")
	}
}

func TestState_InitiallyNormal(t *testing.T) {
	m := newTestForm(t)
	cmd := m.Init()
	// huh.Form.Init() returns a non-nil command to initialize field focus.
	if cmd == nil {
		t.Error("Init() should return a non-nil tea.Cmd")
	}

	if m.State() != huh.StateNormal {
		t.Errorf("expected StateNormal initially, got %v", m.State())
	}
}

func TestView_ShowsTitleWhenActive(t *testing.T) {
	m := newTestForm(t)
	_ = m.Init()

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty when form is active")
	}

	if len(view) < len("Test Form") {
		t.Errorf("View() too short to contain title: %q", view)
	}
}

func TestView_EmptyWhenCompleted(t *testing.T) {
	m := newTestForm(t)

	m.form.State = huh.StateCompleted

	view := m.View()
	if view != "" {
		t.Errorf("View() should return empty string when StateCompleted, got %q", view)
	}
}

func TestCallbackFiredOnlyOnce(t *testing.T) {
	callCount := 0
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("name").Title("Name"),
		),
	)
	m := New("Test Form", f, func(_ *huh.Form) tea.Cmd {
		callCount++
		return nil
	})
	_ = m.Init()

	// Simulate completion state and call Update multiple times.
	m.form.State = huh.StateCompleted

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if callCount != 1 {
		t.Errorf("callback should fire exactly once, fired %d times", callCount)
	}
}
