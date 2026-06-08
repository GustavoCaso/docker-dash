package compose

import (
	"context"
	"errors"
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

func TestComposeLoadedMsgCallsUpdateItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)

	// Only call Init; do not pre-load so the list starts empty.
	if len(section.List.Items()) != 0 {
		t.Fatal("expected empty list before loading")
	}

	loadedMsg := section.RefreshCmd()()
	cmd := section.Update(loadedMsg)

	if len(section.List.Items()) == 0 {
		t.Fatal("UpdateItems should populate the list after composeLoadedMsg")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd after composeLoadedMsg")
	}
}

func TestComposeLoadedMsgEmptyCallsUpdateItemsReset(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)

	section.Update(section.RefreshCmd()())
	section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	cmd := section.Update(composeLoadedMsg{items: []list.Item{}})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after empty composeLoadedMsg, got %d", len(section.List.Items()))
	}
	if section.IsFilter() {
		t.Error("Reset via UpdateItems should clear isFilter")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd (SetItems) after empty composeLoadedMsg")
	}
}

func TestComposeKeyShowsForm(t *testing.T) {
	tests := []struct {
		name   string
		keyMsg tea.KeyMsg
	}{
		{"up", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")}},
		{"down", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}},
		{"restart", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := newModel()
			cmd := model.section.Update(tt.keyMsg)
			if cmd == nil {
				t.Fatal("expected form command, got nil")
			}
			msg := cmd()
			formMsg, ok := msg.(message.ShowFormMsg)
			if !ok {
				t.Fatalf("expected ShowFormMsg, got %T", msg)
			}
			if formMsg.Form == nil {
				t.Fatal("expected non-nil form")
			}
		})
	}
}

func TestComposeStartStopShowsConfirmation(t *testing.T) {
	model := newModel()

	cmd := model.section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if cmd == nil {
		t.Fatal("expected confirmation command, got nil")
	}
	msg := cmd()
	confirm, ok := msg.(message.ShowConfirmationMsg)
	if !ok {
		t.Fatalf("expected ShowConfirmationMsg, got %T", msg)
	}
	if confirm.OnConfirm == nil {
		t.Fatal("expected non-nil OnConfirm")
	}
}

func TestComposeActionMsgSuccess(t *testing.T) {
	model := newModel()
	project := client.ComposeProject{Name: "web-app"}

	result := model.section.handleMsg(composeActionMsg{project: project, action: "up", err: nil})

	if !result.Handled {
		t.Fatal("expected composeActionMsg to be handled")
	}
	if result.Cmd == nil {
		t.Fatal("expected non-nil cmd (refresh + banner)")
	}
}

func TestComposeActionMsgError(t *testing.T) {
	model := newModel()
	project := client.ComposeProject{Name: "web-app"}

	result := model.section.handleMsg(composeActionMsg{
		project: project,
		action:  "up",
		err:     errors.New("daemon unreachable"),
	})

	if !result.Handled {
		t.Fatal("expected composeActionMsg error to be handled")
	}
	if result.Cmd == nil {
		t.Fatal("expected error banner cmd")
	}
	bannerMsg, ok := result.Cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result.Cmd())
	}
	if !bannerMsg.IsError {
		t.Error("expected IsError=true")
	}
}

func TestComposeLoadedMsgError(t *testing.T) {
	model := newModel()

	result := model.section.handleMsg(composeLoadedMsg{error: errors.New("connection refused")})

	if !result.Handled {
		t.Fatal("expected handled")
	}
	if !result.StopSpinner {
		t.Error("expected StopSpinner on error")
	}
	bannerMsg, ok := result.Cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result.Cmd())
	}
	if !bannerMsg.IsError {
		t.Error("expected IsError=true")
	}
}

func TestBuildRestartOptions(t *testing.T) {
	tenSec := 10 * time.Second
	oneMin := time.Minute

	tests := []struct {
		name        string
		noDeps      bool
		timeoutStr  string
		wantNoDeps  bool
		wantTimeout *time.Duration
	}{
		{"defaults", false, "", false, nil},
		{"noDeps set", true, "", true, nil},
		{"timeout set", false, "10s", false, &tenSec},
		{"noDeps and timeout", true, "1m", true, &oneMin},
		{"whitespace timeout", false, "  ", false, nil},
		{"invalid timeout ignored", false, "garbage", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildRestartOptions(tt.noDeps, tt.timeoutStr)
			if opts.NoDeps != tt.wantNoDeps {
				t.Errorf("NoDeps: got %v, want %v", opts.NoDeps, tt.wantNoDeps)
			}
			if tt.wantTimeout == nil && opts.Timeout != nil {
				t.Errorf("Timeout: got %v, want nil", opts.Timeout)
			}
			if tt.wantTimeout != nil {
				if opts.Timeout == nil {
					t.Errorf("Timeout: got nil, want %v", *tt.wantTimeout)
				} else if *opts.Timeout != *tt.wantTimeout {
					t.Errorf("Timeout: got %v, want %v", *opts.Timeout, *tt.wantTimeout)
				}
			}
		})
	}
}

func TestValidateOptionalDuration(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},
		{"   ", false},
		{"10s", false},
		{"1m30s", false},
		{"500ms", false},
		{"garbage", true},
		{"10", true},
	}

	for _, tt := range tests {
		err := validateOptionalDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateOptionalDuration(%q): wantErr=%v, got %v", tt.input, tt.wantErr, err)
		}
	}
}

func TestComposeUpOptsReachClient(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	opts := client.ComposeUpOptions{Build: true, RemoveOrphans: true, Wait: false}
	cmd := section.projectUpCmd(opts)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	actionMsg, ok := msg.(composeActionMsg)
	if !ok {
		t.Fatalf("expected composeActionMsg, got %T", msg)
	}
	if actionMsg.err != nil {
		t.Fatalf("unexpected error: %v", actionMsg.err)
	}
	mock, ok := c.Compose().(*client.MockComposeProjectService)
	if !ok {
		t.Fatal("expected *client.MockComposeProjectService")
	}
	if mock.LastUpOpts.Build != opts.Build {
		t.Errorf("Build: got %v, want %v", mock.LastUpOpts.Build, opts.Build)
	}
	if mock.LastUpOpts.RemoveOrphans != opts.RemoveOrphans {
		t.Errorf("RemoveOrphans: got %v, want %v", mock.LastUpOpts.RemoveOrphans, opts.RemoveOrphans)
	}
}

func TestComposeDownOptsReachClient(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	opts := client.ComposeDownOptions{RemoveOrphans: true}
	cmd := section.projectDownCmd(opts)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	actionMsg, ok := msg.(composeActionMsg)
	if !ok {
		t.Fatalf("expected composeActionMsg, got %T", msg)
	}
	if actionMsg.err != nil {
		t.Fatalf("unexpected error: %v", actionMsg.err)
	}
	mock, ok := c.Compose().(*client.MockComposeProjectService)
	if !ok {
		t.Fatal("expected *client.MockComposeProjectService")
	}
	if mock.LastDownOpts.RemoveOrphans != opts.RemoveOrphans {
		t.Errorf("RemoveOrphans: got %v, want %v", mock.LastDownOpts.RemoveOrphans, opts.RemoveOrphans)
	}
}

func TestComposeRestartOptsReachClient(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	timeout := 30 * time.Second
	opts := client.ComposeRestartOptions{NoDeps: true, Timeout: &timeout}
	cmd := section.projectRestartCmd(opts)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	actionMsg, ok := msg.(composeActionMsg)
	if !ok {
		t.Fatalf("expected composeActionMsg, got %T", msg)
	}
	if actionMsg.err != nil {
		t.Fatalf("unexpected error: %v", actionMsg.err)
	}
	mock, ok := c.Compose().(*client.MockComposeProjectService)
	if !ok {
		t.Fatal("expected *client.MockComposeProjectService")
	}
	if !mock.LastRestartOpts.NoDeps {
		t.Error("NoDeps: got false, want true")
	}
	if mock.LastRestartOpts.Timeout == nil || *mock.LastRestartOpts.Timeout != timeout {
		t.Errorf("Timeout: got %v, want %v", mock.LastRestartOpts.Timeout, timeout)
	}
}

func TestComposeFormKeysReturnNilOnEmptyList(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c.Compose())
	section.SetSize(120, 40)
	// Do NOT load items — list is empty, selectedProject() returns false.

	for _, key := range []tea.Msg{
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")},
	} {
		cmd := section.Update(key)
		if cmd != nil {
			t.Errorf("expected nil cmd for key %v on empty list, got non-nil", key)
		}
	}
}
