package containers

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
)

type containerListModel struct {
	list *List
}

func newContainerListModel() containerListModel {
	client := client.NewMockClient()
	containers, _ := client.Containers().List(context.Background())
	cl := New(containers, client.Containers())
	cl.SetSize(120, 40)
	return containerListModel{list: cl}
}

func (m containerListModel) Init() tea.Cmd { return nil }

func (m containerListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	cmd := m.list.Update(msg)
	return m, cmd
}

func (m containerListModel) View() string {
	return m.list.View()
}

func waitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}

func TestContainerListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListDetailsToggle(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	// Select a container
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Show details
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitFor(t, tm, "Container:")
	// Hide details
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListLogsToggle(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Show logs
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListStartStop(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Toggle start/stop
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	// Navigate to last container (old-container)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Delete
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
func TestContainerListStatsShowsCPUAndMemLabels(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Open stats on running container
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	// Both labels appear in the same rendered frame; check them together so the
	// ANSI compressor (which only diffs changed lines) doesn't swallow one of them.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "CPU") && strings.Contains(s, "MEM")
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListRefresh(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerListModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	// Refresh - send key and give time for the async command to process
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	// The refresh triggers spinner + async reload. After reload completes,
	// send a benign key to trigger a re-render so output is flushed.
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListExecClearClearsOutput(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	cl := New(containers, dockerClient.Containers())
	cl.SetSize(120, 40)

	pr, pw := io.Pipe()
	cl.showExec = true
	cl.execSession = client.NewExecSession(io.NopCloser(pr), pw, func() {
		pr.Close()
		pw.Close()
	})
	cl.execOutput = "previous exec output"
	cl.viewport.SetContent("previous exec output")
	cl.execInput.Focus()

	runes := []rune{}
	for _, ch := range "clear" {
		runes = append(runes, ch)
	}
	cl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: runes})
	cl.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cl.execOutput != "" {
		t.Errorf("execOutput should be empty after clear, got %q", cl.execOutput)
	}
	if cl.execInput.Value() != "" {
		t.Errorf("execInput should be empty after clear, got %q", cl.execInput.Value())
	}

	cl.execSession.Close()
}

func TestContainerListExecClearWithWhitespace(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	cl := New(containers, dockerClient.Containers())
	cl.SetSize(120, 40)

	pr, pw := io.Pipe()
	cl.showExec = true
	cl.execSession = client.NewExecSession(io.NopCloser(pr), pw, func() {
		pr.Close()
		pw.Close()
	})
	cl.execOutput = "some output"
	cl.execInput.Focus()

	// Type "  clear  " (with surrounding spaces) — TrimSpace should still match.
	runes := []rune{}
	for _, ch := range "  clear  " {
		runes = append(runes, ch)
	}

	cl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: runes})
	cl.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cl.execOutput != "" {
		t.Errorf("execOutput should be empty after '  clear  ', got %q", cl.execOutput)
	}

	cl.execSession.Close()
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{512 * 1024 * 1024, "512.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
