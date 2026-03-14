package volumes

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type volumeSectionModel struct {
	section *Section
}

func newModel() volumeSectionModel {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	section := New(context.Background(), volumes, c.Volumes())
	section.SetSize(120, 40)
	return volumeSectionModel{section: section}
}

func (m volumeSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m volumeSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m volumeSectionModel) View() string {
	return m.section.View()
}

func (m volumeSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestVolumeReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "postgres_data")

	cmd := model.Reset()

	if cmd == nil {
		t.Error("Reset() should return cmd")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsFlags(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	s := New(context.Background(), volumes, c.Volumes())
	s.SetSize(120, 40)

	s.isFilter = true

	cmd := s.Reset()

	if s.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd == nil {
		t.Error("Reset() should return cmd from activePanel.Close()")
	}
}

func TestVolumeListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "postgres_data")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestVolumeListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "app_data") // unused volume present initially
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitForNot(t, tm, "app_data")   // unused volume pruned
	waitFor(t, tm, "postgres_data") // used volumes remain
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func waitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}

func waitForNot(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return !strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}

func TestPanelClosedOnUpDownNavigation(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	section := New(context.Background(), volumes, c.Volumes())
	section.SetSize(120, 40)

	// Navigate to second volume
	section.list.Select(1)
	// Initialize the filetree panel with content
	section.activePanel().Init("volume2")

	// Navigate down to next volume - this should close the current panel (clearing viewport)
	section.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify the panel Close() was called by reinitializing successfully
	section.activePanel().Init("volume3")

	// Verify the panel view is generated without errors
	view := section.activePanel().View()
	if view == "" {
		t.Error("Panel view should not be empty after reinitialization")
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	section := New(context.Background(), volumes, c.Volumes())
	section.SetSize(120, 40)

	// Navigate to second volume
	section.list.Select(1)
	// Initialize the filetree panel
	section.activePanel().Init("volume2")

	// Navigate up to previous volume - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify the panel can be reinitialized without issues
	section.activePanel().Init("volume1")

	// Verify the panel view is generated
	view := section.activePanel().View()
	if view == "" {
		t.Error("Panel view should not be empty after reinitialization")
	}
}
