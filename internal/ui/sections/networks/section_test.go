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

func (m networkSectionModel) Init() tea.Cmd { return nil }

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

func TestNetworkListDetailsToggle(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "bridge")
	// Show details panel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitFor(t, tm, "Network:")
	// Hide details panel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
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
	// Details
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitFor(t, tm, "Network:")

	model.Reset()

	view := model.View()

	if strings.Contains(view, "bridge:") {
		t.Errorf("Reset should reset viewport. Found: %s", view)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func waitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}
