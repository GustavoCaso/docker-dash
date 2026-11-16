package containers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

type containerSectionModel struct {
	section *Section
}

func newContainerSectionModel() containerSectionModel {
	client := client.NewMockClient()
	section := New(context.Background(), client.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)
	return containerSectionModel{section: section}
}

func (m containerSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m containerSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	// Mirror the app's BubbleUpMsg handling: forward the key to the section.
	if bubbleMsg, ok := msg.(message.BubbleUpMsg); ok {
		return m, m.section.Update(bubbleMsg.KeyMsg)
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m containerSectionModel) View() tea.View {
	return tea.NewView(m.section.View())
}

func (m containerSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestContainerReset(t *testing.T) {
	model := newContainerSectionModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	// Set focus on panels
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to logs panel
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	time.Sleep(500 * time.Millisecond)

	cmd := model.Reset()

	if cmd == nil {
		t.Error("Reset() should return non-nil cmd when activePanel was set")
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListStartStop(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	// Toggle start/stop
	tm.Send(
		tea.KeyPressMsg{Code: 's', Text: "s"},
	) // We stop the container with id "abc123def456" or the first container
	time.Sleep(500 * time.Millisecond)
	tm.Quit()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(containerSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()
	for _, item := range items {
		if vi, ok := item.(containerItem); ok {
			if vi.container.ID == "abc123def456" {
				if vi.container.State != client.StateStopped {
					t.Fatalf(" expected nginx:latest container to be stopped, got: %s", vi.container.State)
				}
				return
			}
		}
	}

	t.Fatal("not found nginx:latest container")
}

func TestContainerListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	// Navigate to last container (old-container)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown}) // ID: jkl012mno345
	// Delete
	tm.Send(tea.KeyPressMsg{Code: 'D', Text: "D"})
	time.Sleep(500 * time.Millisecond)
	tm.Quit()

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(containerSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()

	for _, item := range items {
		if vi, ok := item.(containerItem); ok {
			if vi.container.ID == "jkl012mno345" {
				t.Fatal("expected to delete old-container, found in list after delete")
			}
		}
	}
}

func TestExecPanelCapturesInput(t *testing.T) {
	dockerClient := client.NewMockClient()
	section := New(
		context.Background(),
		dockerClient.Containers(),
		config.DefaultLogsConfig(),
		config.DefaultFilesConfig(),
	)
	section.SetSize(120, 40)

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to exec panel (details=0, logs=1, stats=2, files=3 exec=4)
	// Moving one to the left wraps to exec (index 4)
	section.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	ep, ok := section.ActivePanel().(*execPanel)
	if !ok {
		t.Fatal("active panel is not the exec panel")
	}

	if section.CapturesInput() {
		t.Fatal("section must not capture input before the exec session starts")
	}

	containers, err := dockerClient.Containers().List(t.Context())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	session, err := dockerClient.Containers().Exec(t.Context(), containers[0].ID, 80, 24)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	ep.Update(execSessionStartedMsg{session: session})
	t.Cleanup(func() { ep.Close() })

	if !section.CapturesInput() {
		t.Fatal("section should capture input while the exec session is live")
	}

	// 'd' is the container delete binding; it must reach the shell instead.
	if cmd := section.Update(tea.KeyPressMsg{Code: 'd', Text: "d"}); cmd != nil {
		if _, isConfirm := cmd().(message.ShowConfirmationMsg); isConfirm {
			t.Fatal("delete binding fired while exec panel captures input")
		}
	}

	section.Update(tea.KeyPressMsg{Code: ']', Mod: tea.ModCtrl})
	if section.CapturesInput() || section.IsPanelFocused() {
		t.Fatal("ctrl+] should release input and return focus to the list")
	}
	if name := section.ActivePanelName(); name != "Details" {
		t.Fatalf("ctrl+] should switch back to the details panel, got %q", name)
	}
	if ep.session != nil || ep.term != nil {
		t.Fatal("ctrl+] should close the exec session")
	}
}

func TestExecCloseMsgIgnoresStaleSession(t *testing.T) {
	dockerClient := client.NewMockClient()
	section := New(
		context.Background(),
		dockerClient.Containers(),
		config.DefaultLogsConfig(),
		config.DefaultFilesConfig(),
	)
	section.SetSize(120, 40)
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	section.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	ep, ok := section.ActivePanel().(*execPanel)
	if !ok {
		t.Fatal("active panel is not the exec panel")
	}

	live := client.NewExecSession("live", io.NopCloser(strings.NewReader("")), nopWriteCloser{}, nil)
	ep.session = live

	stale := client.NewExecSession("stale", io.NopCloser(strings.NewReader("")), nopWriteCloser{}, nil)
	if cmd := section.Update(execCloseMsg{session: stale}); cmd != nil {
		t.Fatal("stale execCloseMsg should be ignored")
	}
	if ep.session != live {
		t.Fatal("stale execCloseMsg closed the live session")
	}

	section.Update(execCloseMsg{session: live})
	if ep.session != nil {
		t.Fatal("execCloseMsg for the live session should close it")
	}
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

func TestActivePanelClosedOnLogsSessionClose(t *testing.T) {
	dockerClient := client.NewMockClient()
	section := New(
		context.Background(),
		dockerClient.Containers(),
		config.DefaultLogsConfig(),
		config.DefaultFilesConfig(),
	)
	section.SetSize(120, 40)

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to stats panel (details=0, logs=1, stats=2, files=3 exec=4)
	section.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	lp := section.ActivePanel().(*logsPanel)

	pr, pw := io.Pipe()
	lp.logsSession = client.NewLogsSession(io.NopCloser(pr), func() { pr.Close(); pw.Close() })
	lp.appendLines("some logs\n")

	// Simulate exec close which calls activePanel().Close()
	section.ActivePanel().Close()

	if len(lp.list.Items()) != 0 {
		t.Errorf("Close() should clear list items, got %d", len(lp.list.Items()))
	}
}

func TestContainerPauseUnpause(t *testing.T) {
	tm := teatest.NewTestModel(t, newContainerSectionModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	// Toggle pause/unpause
	tm.Send(
		tea.KeyPressMsg{Code: 'p', Text: "p"},
	) // We pause the container with id "abc123def456" or the first container
	time.Sleep(500 * time.Millisecond)
	tm.Quit()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(containerSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()
	for _, item := range items {
		if vi, ok := item.(containerItem); ok {
			if vi.container.ID == "abc123def456" {
				if vi.container.State != client.StatePaused {
					t.Fatalf(" expected nginx:latest container to be paused, got: %s", vi.container.State)
				}
				return
			}
		}
	}

	t.Fatal("not found nginx:latest container")
}

func TestContainerKill(t *testing.T) {
	model := newContainerSectionModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)

	tm.Send(
		tea.KeyPressMsg{Code: 'K', Text: "K"},
	) // We kill the container with id "abc123def456" or the first container
	time.Sleep(500 * time.Millisecond)
	tm.Quit()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	m, ok := fm.(containerSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()
	for _, item := range items {
		if vi, ok := item.(containerItem); ok {
			if vi.container.ID == "abc123def456" {
				if vi.container.State != client.StateStopped {
					t.Fatalf(" expected nginx:latest container to be stopped, got: %s", vi.container.State)
				}
				return
			}
		}
	}

	t.Fatal("not found nginx:latest container")
}

func TestContainersLoadedMsgCallsUpdateItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)

	if len(section.List.Items()) != 0 {
		t.Fatal("expected empty list before loading")
	}

	loadedMsg := section.RefreshCmd()()
	cmd := section.Update(loadedMsg)

	if len(section.List.Items()) == 0 {
		t.Fatal("UpdateItems should populate the list after containersLoadedMsg")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd after containersLoadedMsg")
	}
}

func TestContainersLoadedMsgEmptyCallsUpdateItemsReset(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)

	section.Update(section.RefreshCmd()())
	section.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	cmd := section.Update(containersLoadedMsg{items: []list.Item{}})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after empty containersLoadedMsg, got %d", len(section.List.Items()))
	}
	if section.IsFilter() {
		t.Error("Reset via UpdateItems should clear isFilter")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd (SetItems) after empty containersLoadedMsg")
	}
}

func TestContainerRestart(t *testing.T) {
	model := newContainerSectionModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)

	tm.Send(tea.KeyPressMsg{Code: 'R', Text: "R"})
	time.Sleep(500 * time.Millisecond)
	tm.Quit()
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(containerSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	items := m.section.List.Items()
	for _, item := range items {
		if vi, ok := item.(containerItem); ok {
			if vi.container.ID == "abc123def456" {
				// After restart the mock sets state to running
				if vi.container.State != client.StateRunning {
					t.Fatalf("expected nginx-proxy container to be running after restart, got: %s", vi.container.State)
				}
				return
			}
		}
	}
	t.Fatal("nginx-proxy container not found")
}

func TestContainerDeleteConfirmationMsg(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	cmd := section.confirmContainerDelete()
	if cmd == nil {
		t.Fatal("confirmContainerDelete should return non-nil cmd")
	}
	msg := cmd()
	confirm, ok := msg.(message.ShowConfirmationMsg)
	if !ok {
		t.Fatalf("expected ShowConfirmationMsg, got %T", msg)
	}
	if confirm.Title != "Delete Container" {
		t.Errorf("unexpected title: %q", confirm.Title)
	}
	if confirm.OnConfirm == nil {
		t.Error("OnConfirm should not be nil")
	}
}

func TestContainerRestartConfirmationMsg(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	cmd := section.confirmContainerRestart()
	if cmd == nil {
		t.Fatal("confirmContainerRestart should return non-nil cmd")
	}
	msg := cmd()
	confirm, ok := msg.(message.ShowConfirmationMsg)
	if !ok {
		t.Fatalf("expected ShowConfirmationMsg, got %T", msg)
	}
	if confirm.Title != "Restart Container" {
		t.Errorf("unexpected title: %q", confirm.Title)
	}
}

func TestContainerActionMsgError(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)

	result := section.handleMsg(containerActionMsg{
		ID:     "abc123def456",
		Action: "stopping",
		Idx:    0,
		Error:  errors.New("permission denied"),
	})

	if !result.Handled {
		t.Fatal("expected containerActionMsg error to be handled")
	}
	if !result.StopSpinner {
		t.Error("expected StopSpinner on error")
	}
	banner, ok := result.Cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result.Cmd())
	}
	if !banner.IsError {
		t.Error("expected IsError=true for action error")
	}
}

func TestContainersPrunedMsgError(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)

	result := section.handleMsg(containersPrunedMsg{err: errors.New("prune failed")})

	if !result.Handled {
		t.Fatal("expected containersPrunedMsg error to be handled")
	}
	if !result.StopSpinner {
		t.Error("expected StopSpinner on prune error")
	}
	banner, ok := result.Cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result.Cmd())
	}
	if !banner.IsError {
		t.Error("expected IsError=true for prune error")
	}
}

func TestContainersLoadedMsgError(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Containers(), config.DefaultLogsConfig(), config.DefaultFilesConfig())
	section.SetSize(120, 40)

	result := section.handleMsg(containersLoadedMsg{error: errors.New("connection refused")})

	if !result.Handled {
		t.Fatal("expected containersLoadedMsg error to be handled")
	}
	if !result.StopSpinner {
		t.Error("expected StopSpinner on load error")
	}
	banner, ok := result.Cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result.Cmd())
	}
	if !banner.IsError {
		t.Error("expected IsError=true for load error")
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
