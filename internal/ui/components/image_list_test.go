package components

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/service"
)

type imageListModel struct {
	list *ImageList
}

func newImageListModel() imageListModel {
	client := service.NewMockClient()
	images, _ := client.Images().List(context.Background())
	il := NewImageList(images, client)
	il.SetSize(120, 40)
	return imageListModel{list: il}
}

func (m imageListModel) Init() tea.Cmd { return nil }

func (m imageListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	cmd := m.list.Update(msg)
	return m, cmd
}

func (m imageListModel) View() string {
	return m.list.View()
}

func TestImageListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newImageListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListLayersToggle(t *testing.T) {
	tm := teatest.NewTestModel(t, newImageListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Select an image
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Show layers
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	waitFor(t, tm, "Layers for")
	// Hide layers
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newImageListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Select an image
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Delete (deletes the selected image which is node after KeyDown)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitFor(t, tm, "postgres")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListRefresh(t *testing.T) {
	tm := teatest.NewTestModel(t, newImageListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Refresh
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	// The refresh triggers spinner + async reload. After reload completes,
	// send a benign key to trigger a re-render so output is flushed.
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListRunContainer(t *testing.T) {
	tm := teatest.NewTestModel(t, newImageListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Select an image
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Run container
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	// Wait for async operation, then flush output
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
