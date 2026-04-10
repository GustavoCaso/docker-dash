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

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsFlags(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	s := New(context.Background(), volumes, c.Volumes())
	s.SetSize(120, 40)

	s.IsFilter = true

	cmd := s.Reset()

	if s.IsFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd != nil {
		t.Error("Reset() should return nil cmd")
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
func TestVolumeDeleteUpdatesSelection(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	section := New(context.Background(), volumes, c.Volumes())
	section.SetSize(120, 40)

	initialCount := len(section.List.Items())
	if initialCount == 0 {
		t.Fatal("expected at least one volume in mock data")
	}

	// Select the first item and delete it
	section.List.Select(0)
	section.removeItem(0)

	if len(section.List.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.List.Items()))
	}
	if section.List.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.List.Index())
	}
}

func TestVolumeDeleteLastItemClampsSelection(t *testing.T) {
	c := client.NewMockClient()
	volumes, _ := c.Volumes().List(context.Background())
	section := New(context.Background(), volumes, c.Volumes())
	section.SetSize(120, 40)

	count := len(section.List.Items())
	if count == 0 {
		t.Fatal("expected at least one volume in mock data")
	}

	// Select and delete items until one remains
	for len(section.List.Items()) > 1 {
		section.removeItem(len(section.List.Items()) - 1)
	}

	// Delete the last item
	section.removeItem(0)

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items, got %d", len(section.List.Items()))
	}
}
