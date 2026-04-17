package compose

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type composeSectionModel struct {
	section *Section
}

func newModel() composeSectionModel {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)
	// Load data synchronously so tests that call View/Update directly work without teatest.
	section.Update(section.RefreshCmd()())
	return composeSectionModel{section: section}
}

func (m composeSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m composeSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m composeSectionModel) View() string {
	return m.section.View()
}

// TestComposeViewRendersItems verifies that the section view directly contains
// the expected project names without going through the teatest async output
// pipeline (which can be flaky for sections with panels when no initial command
// is returned from Init).
func TestComposeViewRendersItems(t *testing.T) {
	m := newModel()
	view := m.section.View()
	for _, name := range []string{"web-app", "monitoring"} {
		if !strings.Contains(view, name) {
			t.Errorf("expected View() to contain %q", name)
		}
	}
}

// TestComposeViewDetailsPanel checks that the details panel renders project info.
func TestComposeViewDetailsPanel(t *testing.T) {
	m := newModel()
	view := m.section.View()
	if !strings.Contains(view, "Details") {
		t.Error("expected View() to contain 'Details' panel tab")
	}
}

func TestComposeReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	cmd := model.section.Reset()
	if cmd != nil {
		t.Error("Reset() should return nil cmd for compose section")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestComposeRefresh(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	// Trigger a refresh
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	// After async reload, check that the model still renders without errors
	time.Sleep(300 * time.Millisecond)

	view := model.section.View()
	if !strings.Contains(view, "web-app") {
		t.Error("expected View() to still contain 'web-app' after refresh")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestComposeKeyShowsConfirmation(t *testing.T) {
	model := newModel()

	cmd := model.section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("expected confirmation command")
	}

	msg := cmd()
	confirmation, ok := msg.(message.ShowConfirmationMsg)
	if !ok {
		t.Fatalf("expected ShowConfirmationMsg, got %T", msg)
	}

	if confirmation.Title != "Compose Up" {
		t.Fatalf("unexpected confirmation title: %s", confirmation.Title)
	}
}

func TestComposeLoadedMsgRefreshesActivePanel(t *testing.T) {
	model := newModel()

	updated := client.ComposeProject{
		Name:             "web-app",
		WorkingDir:       "/tmp/alternate",
		ConfigFiles:      "/tmp/alternate/compose.yml",
		EnvironmentFiles: "/tmp/alternate/.env",
		Services: []client.ComposeServiceInfo{
			{Name: "api", State: "running", Image: "node:18-alpine"},
		},
	}

	items := []list.Item{composeItem{project: updated}}
	result := model.section.handleMsg(composeLoadedMsg{items: items})
	if !result.Handled {
		t.Fatal("expected composeLoadedMsg to be handled")
	}
	_ = result.Cmd

	details := model.section.ActivePanel().View()
	if !strings.Contains(details, updated.WorkingDir) {
		t.Fatalf("expected details to show refreshed project, got %q", details)
	}
}
