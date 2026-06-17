package networks

import (
	"context"
	"slices"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type networkSectionModel struct {
	section *Section
}

func newModel() networkSectionModel {
	c := client.NewMockClient()
	section := New(context.Background(), c.Networks())
	section.SetSize(120, 40)
	return networkSectionModel{section: section}
}

func (m networkSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m networkSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m networkSectionModel) View() tea.View {
	return tea.NewView(m.section.View())
}

func (m networkSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestNetworkReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	cmd := model.Reset()

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'P', Text: "P"})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(networkSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()

	// unused network IDs
	// def456ghi789def4
	// ghi789jkl012ghi7
	// mno345pqr678mno3

	unusedNetworkIds := []string{"def456ghi789def4", "ghi789jkl012ghi7", "mno345pqr678mno3"}

	for _, item := range items {
		if vi, ok := item.(networkItem); ok {
			if slices.Contains(unusedNetworkIds, vi.network.ID) {
				t.Fatalf("expected to delete unused networks, found %s in list after prune", vi.network.ID)
			}
		}
	}
}

func TestNetworkDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	// Delete first network with ID abc123def456abc1
	tm.Send(tea.KeyPressMsg{Code: 'D', Text: "D"})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(networkSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()

	for _, item := range items {
		if vi, ok := item.(networkItem); ok {
			if vi.network.ID == "abc123def456abc1" {
				t.Fatal("expected to delete bridge, found in list after delete")
			}
		}
	}
}

func TestNetworksLoadedMsgCallsUpdateItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Networks())
	section.SetSize(120, 40)

	if len(section.List.Items()) != 0 {
		t.Fatal("expected empty list before loading")
	}

	loadedMsg := section.RefreshCmd()()
	cmd := section.Update(loadedMsg)

	if len(section.List.Items()) == 0 {
		t.Fatal("UpdateItems should populate the list after networksLoadedMsg")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd after networksLoadedMsg")
	}
}

func TestNetworksLoadedMsgEmptyCallsUpdateItemsReset(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Networks())
	section.SetSize(120, 40)

	section.Update(section.RefreshCmd()())
	// Manually toggle filter via Update to set isFilter via the base's toggleFilter path
	section.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	cmd := section.Update(networksLoadedMsg{items: []list.Item{}})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after empty networksLoadedMsg, got %d", len(section.List.Items()))
	}
	if section.IsFilter() {
		t.Error("Reset via UpdateItems should clear isFilter")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd (SetItems) after empty networksLoadedMsg")
	}
}
