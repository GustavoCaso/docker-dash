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
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type containerSectionModel struct {
	section *Section
}

func newContainerSectionModel() containerSectionModel {
	client := client.NewMockClient()
	containers, _ := client.Containers().List(context.Background())
	section := New(context.Background(), containers, client.Containers())
	section.SetSize(120, 40)
	return containerSectionModel{section: section}
}

func (m containerSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m containerSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m containerSectionModel) View() string {
	return m.section.View()
}

func (m containerSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
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

func TestContainerListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListDetailsVisible(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	// Select a container - details panel is always shown (it's the default panel)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "Container:")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListLogsPanel(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Navigate to logs panel using shift+right
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})

	waitFor(t, tm, "Starting application")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerReset(t *testing.T) {
	model := newContainerSectionModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Navigate to logs panel
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})

	waitFor(t, tm, "Starting application")

	cmd := model.Reset()

	if cmd == nil {
		t.Error("Reset() should return non-nil cmd when activePanel was set")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListPanelNavigation(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Default is details panel
	waitFor(t, tm, "Container:")
	// Navigate to logs panel using shift+right
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})
	waitFor(t, tm, "Starting application")
	// Navigate back to details panel using shift+left
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftLeft})
	waitFor(t, tm, "Container:")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListStartStop(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Toggle start/stop
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
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
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx-proxy")
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Navigate to stats panel (index 2: details=0, logs=1, stats=2)
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftRight})
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
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
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

func TestContainerListExecMouseScroll(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	section := New(context.Background(), containers, dockerClient.Containers())
	section.SetSize(120, 40)

	// Navigate to stats panel (details=0, logs=1, stats=2, files=3 exec=4)
	// Instead if moving four time to the right, we move one to the left
	section.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	ep := section.panels[section.activePanelIdx].(*execPanel)

	// Give the viewport content tall enough to scroll
	lines := strings.Repeat("output line\n", 50)
	ep.viewport.SetContent(lines)
	ep.viewport.GotoBottom()
	beforeOffset := ep.viewport.YOffset

	// Send a scroll-up mouse event directly to the section
	section.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})

	if ep.viewport.YOffset >= beforeOffset {
		t.Errorf("scroll up should decrease YOffset: before=%d after=%d", beforeOffset, ep.viewport.YOffset)
	}
}

func TestActivePanelClosedOnLogsSessionClose(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	cl := New(context.Background(), containers, dockerClient.Containers())
	cl.SetSize(120, 40)

	// Navigate to logs panel (index 1)
	cl.activePanelIdx = 1
	lp := cl.panels[1].(*logsPanel)

	pr, pw := io.Pipe()
	lp.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	lp.logsOutput = "some logs"

	// Simulate exec close which calls activePanel().Close()
	cl.activePanel().Close()

	if lp.logsOutput != "" {
		t.Errorf("Close() should clear logsOutput, got %q", lp.logsOutput)
	}
}

func TestContainerListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "old-container") // stopped container present initially
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitForNot(t, tm, "old-container") // stopped container pruned
	waitFor(t, tm, "nginx-proxy")      // running containers remain
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsFilterFlag(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	cl := New(context.Background(), containers, dockerClient.Containers())
	cl.SetSize(120, 40)

	cl.isFilter = true

	cl.Reset()

	if cl.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
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

func TestContainerDeleteUpdatesSelection(t *testing.T) {
	c := client.NewMockClient()
	containers, _ := c.Containers().List(context.Background())
	section := New(context.Background(), containers, c.Containers())
	section.SetSize(120, 40)

	initialCount := len(section.list.Items())
	if initialCount == 0 {
		t.Fatal("expected at least one container in mock data")
	}

	section.list.Select(0)
	cmd := section.removeItem(0)

	if len(section.list.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.list.Items()))
	}
	if section.list.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.list.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(detailsMsg); !ok {
		t.Errorf("removeItem() cmd should produce detailsMsg, got %T", cmd())
	}
}

func TestContainerDeleteLastItemClampsSelection(t *testing.T) {
	c := client.NewMockClient()
	containers, _ := c.Containers().List(context.Background())
	section := New(context.Background(), containers, c.Containers())
	section.SetSize(120, 40)

	if len(section.list.Items()) == 0 {
		t.Fatal("expected at least one container in mock data")
	}

	// Delete all but the last item without checking cmd type (items still remain)
	for len(section.list.Items()) > 1 {
		section.removeItem(0)
	}

	// Delete the final item — no selected item remains, so cmd must be nil
	cmd := section.removeItem(0)
	if cmd != nil {
		t.Error("removeItem() should return nil cmd when list is empty")
	}
}

func TestContainerDeleteMiddleItemClampsToLastWhenAtEnd(t *testing.T) {
	c := client.NewMockClient()
	containers, _ := c.Containers().List(context.Background())
	section := New(context.Background(), containers, c.Containers())
	section.SetSize(120, 40)

	count := len(section.list.Items())
	if count < 2 {
		t.Fatal("expected at least two containers in mock data")
	}

	// Select and delete the last item — selection should clamp to new last
	last := count - 1
	section.list.Select(last)
	cmd := section.removeItem(last)

	if section.list.Index() != last-1 {
		t.Errorf("expected selection at %d after deleting last item, got %d", last-1, section.list.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(detailsMsg); !ok {
		t.Errorf("removeItem() cmd should produce detailsMsg, got %T", cmd())
	}
}

func TestPanelClosedOnUpDownNavigation(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	section := New(context.Background(), containers, dockerClient.Containers())
	section.SetSize(120, 40)

	// Navigate to stats panel (index 2)
	section.activePanelIdx = 2
	statsPanel := section.panels[2].(*statsPanel)

	// Set some state on the stats panel to verify it gets cleared
	pr, pw := io.Pipe()
	statsPanel.session = client.NewStatsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	statsPanel.lastView = "previous container stats"

	// Navigate down to next container - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify the stats panel was closed (session nil'd, lastView cleared)
	if statsPanel.session != nil {
		t.Error("Navigation should close stats panel, session should be nil")
	}
	if statsPanel.lastView != "" {
		t.Errorf("Navigation should clear stats panel lastView, got %q", statsPanel.lastView)
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	dockerClient := client.NewMockClient()
	containers, _ := dockerClient.Containers().List(context.Background())
	section := New(context.Background(), containers, dockerClient.Containers())
	section.SetSize(120, 40)

	// Navigate to logs panel (index 1) on second container
	section.list.Select(1) // Select second container
	section.activePanelIdx = 1
	logsPanel := section.panels[1].(*logsPanel)

	// Set some state on the logs panel
	pr, pw := io.Pipe()
	logsPanel.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	logsPanel.logsOutput = "previous container logs"

	// Navigate up to previous container - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify the logs panel was closed
	if logsPanel.logsOutput != "" {
		t.Errorf("Navigation should clear logs panel output, got %q", logsPanel.logsOutput)
	}
}
