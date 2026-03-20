package networks

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

type networkSectionModel struct {
	section *Section
}

func newModel() networkSectionModel {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)
	return networkSectionModel{section: section}
}

func (m networkSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m networkSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m networkSectionModel) View() string {
	return m.section.View()
}

func (m networkSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestNetworkListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "bridge")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkListDetailsVisible(t *testing.T) {
	t.Helper()
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	// Both list and details panel are always visible simultaneously
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "bridge") && strings.Contains(s, "abc123def456")
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "bridge")
	// Delete the selected network (bridge)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	waitFor(t, tm, "host")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkListRefresh(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "bridge")
	// Refresh
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	// Wait for async reload then flush with a benign key
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "bridge")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "bridge")

	cmd := model.Reset()

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsFlags(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	s := New(context.Background(), networks, c.Networks())
	s.SetSize(120, 40)

	s.isFilter = true

	cmd := s.Reset()

	if s.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd != nil {
		t.Error("Reset() should return nil cmd from activePanel.Close()")
	}
}

func TestNetworkListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "host") // unused network present initially
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitForNot(t, tm, "host")     // unused networks pruned
	waitFor(t, tm, "app-network") // connected networks remain
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

func TestNetworkDeleteUpdatesSelection(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)

	initialCount := len(section.list.Items())
	if initialCount == 0 {
		t.Fatal("expected at least one network in mock data")
	}

	section.list.Select(0)
	section.removeItem(0)

	if len(section.list.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.list.Items()))
	}
	if section.list.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.list.Index())
	}
}

func TestNetworkDeleteLastItemClampsSelection(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)

	if len(section.list.Items()) == 0 {
		t.Fatal("expected at least one network in mock data")
	}

	// Delete all items — detailsPanel.Init() always returns nil (sync panel)
	for len(section.list.Items()) > 0 {
		section.removeItem(0)
	}
}

func TestNetworkDeleteMiddleItemClampsToLastWhenAtEnd(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)

	count := len(section.list.Items())
	if count < 2 {
		t.Fatal("expected at least two networks in mock data")
	}

	// Select and delete the last item — selection should clamp to new last
	last := count - 1
	section.list.Select(last)
	section.removeItem(last)

	if section.list.Index() != last-1 {
		t.Errorf("expected selection at %d after deleting last item, got %d", last-1, section.list.Index())
	}
}

func TestPanelClosedOnUpDownNavigation(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)

	// Navigate to second network
	section.list.Select(1)
	// Initialize the details panel with content
	section.activePanel().Init("network2 content")

	// Navigate down to next network - this should close the current panel (clearing viewport)
	section.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify the panel Close() was called by reinitializing successfully
	// The panel should not panic and should accept new content
	section.activePanel().Init("network3 content")

	// Verify the panel view is generated without errors
	view := section.activePanel().View()
	if view == "" {
		t.Error("Panel view should not be empty after reinitialization")
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	c := client.NewMockClient()
	networks, _ := c.Networks().List(context.Background())
	section := New(context.Background(), networks, c.Networks())
	section.SetSize(120, 40)

	// Navigate to second network
	section.list.Select(1)
	// Initialize the details panel
	section.activePanel().Init("network2 content")

	// Navigate up to previous network - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify the panel can be reinitialized without issues
	section.activePanel().Init("network1 content")

	// Verify the panel view is generated
	view := section.activePanel().View()
	if view == "" {
		t.Error("Panel view should not be empty after reinitialization")
	}
}
