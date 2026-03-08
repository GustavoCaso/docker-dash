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

func (m volumeSectionModel) Init() tea.Cmd { return nil }

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
	// Open file tree on the selected volume (postgres_data)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	waitFor(t, tm, "pgdata")

	cmd := model.Reset()

	if cmd == nil {
		t.Error("Reset() should return non-nil cmd when activePanel was set")
	}

	view := model.View()

	if strings.Contains(view, "pgdata") {
		t.Errorf("Reset should close the file tree panel. Found: %s", view)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsActivePanelAndFlags(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	s := New(context.Background(), volumes, c.Volumes())
	s.SetSize(120, 40)

	s.activePanel = s.fileTreePanel
	s.isFilter = true

	cmd := s.Reset()

	if s.activePanel != nil {
		t.Error("Reset() should set activePanel to nil")
	}
	if s.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd == nil {
		t.Error("Reset() should return non-nil cmd when activePanel was set")
	}
}

func TestResetWithNoActivePanelReturnsNilCmd(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	s := New(context.Background(), volumes, c.Volumes())
	s.SetSize(120, 40)

	s.isFilter = true

	cmd := s.Reset()

	if s.activePanel != nil {
		t.Error("Reset() should leave activePanel as nil when it was already nil")
	}
	if s.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd != nil {
		t.Error("Reset() should return nil cmd when no activePanel was set")
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
