package volumes

import (
	"context"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type volumeSectionModel struct {
	section *Section
}

func newModel() volumeSectionModel {
	c := client.NewMockClient()
	section := New(context.Background(), c.Volumes())
	section.SetSize(120, 40)
	return volumeSectionModel{section: section}
}

func (m volumeSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m volumeSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m volumeSectionModel) View() tea.View {
	return tea.NewView(m.section.View())
}

func (m volumeSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestVolumeReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	cmd := model.Reset()

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestVolumeListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyPressMsg{Code: 'P', Text: "P"})

	// Wait for the post-prune reload to settle, then quit and inspect model state.
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(volumeSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()
	names := make([]string, 0, len(items))
	for _, item := range items {
		if vi, ok := item.(volumeItem); ok {
			names = append(names, vi.volume.Name)
		}
	}

	foundPostgres := false
	foundApp := false
	for _, name := range names {
		if name == "postgres_data" {
			foundPostgres = true
		}
		if name == "app_data" {
			foundApp = true
		}
	}
	if !foundPostgres {
		t.Errorf("postgres_data (used volume) should remain after prune, got: %v", names)
	}
	if foundApp {
		t.Errorf("app_data (unused volume) should be pruned, got: %v", names)
	}
}

func TestVolumesLoadedMsgCallsUpdateItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Volumes())
	section.SetSize(120, 40)

	if len(section.List.Items()) != 0 {
		t.Fatal("expected empty list before loading")
	}

	loadedMsg := section.RefreshCmd()()
	cmd := section.Update(loadedMsg)

	if len(section.List.Items()) == 0 {
		t.Fatal("UpdateItems should populate the list after volumesLoadedMsg")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd after volumesLoadedMsg")
	}
}

func TestVolumesLoadedMsgEmptyCallsUpdateItemsReset(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Volumes())
	section.SetSize(120, 40)

	// Pre-load items then send empty loaded msg to trigger the reset branch.
	section.Update(section.RefreshCmd()())
	section.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	cmd := section.Update(volumesLoadedMsg{items: []list.Item{}})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after empty volumesLoadedMsg, got %d", len(section.List.Items()))
	}
	if section.IsFilter() {
		t.Error("Reset via UpdateItems should clear isFilter")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd (SetItems) after empty volumesLoadedMsg")
	}
}
