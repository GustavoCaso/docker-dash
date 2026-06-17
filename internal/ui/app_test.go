package ui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/header"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

func TestFullOutput(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// Images.
func TestImageListRendersItems(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "nginx")
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListLayersVisible(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "No layer information available")
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// Containers.
func TestContainerListLogsPanel(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate header right to switch to Containers view (default is Images)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})

	// Wait for Containers view to render
	waitForString(t, tm, "nginx-proxy")
	// Set focus on panels
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to logs panel using shift+right
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})

	waitForString(t, tm, "Starting application")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestExecPanel(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Wait for initial render
	waitForString(t, tm, "Images")

	// Navigate header right to switch to Containers view (default is Images)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})

	// Wait for Containers view to render
	waitForString(t, tm, "nginx-proxy")

	// Navigate to first container (should be running)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	// Set focus on panels
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to exec panel using shift+right (panels: details=0, logs=1, stats=2, filetree=3, exec=4)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})

	// Wait for the exec input prompt to appear
	waitForString(t, tm, "$")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListDetailsVisible(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	// Wait for initial render
	waitForString(t, tm, "Images")
	// Navigate header right to switch to Containers view (default is Images)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	// Select a container - details panel is always shown (it's the default panel)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	waitForString(t, tm, "Container:")
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerSwitchSectionResetsPanel(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch to Containers view
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	waitForString(t, tm, "nginx-proxy")
	// Select a container and navigate to logs panel
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	// Set focus on panels
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	waitForString(t, tm, "Starting application")
	// Switch away and back - "Starting application" should disappear as panel is closed
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Starting application")
	})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSwitchingSectionResetActiveView(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch section and back
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Containers")
	})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListStatsShowsCPUAndMemLabels(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	waitForString(t, tm, "nginx-proxy")
	// Set focus on panels
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	// Navigate to stats panel (index 2: details=0, logs=1, stats=2)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	// Both labels appear in the same rendered frame; check them together so the
	// ANSI compressor (which only diffs changed lines) doesn't swallow one of them.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "NET") && strings.Contains(s, "I/O")
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*3))
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerListPrune(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})

	waitForString(t, tm, "old-container") // stopped container present initially
	tm.Send(tea.KeyPressMsg{Code: 'P', Text: "P"})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"}) // confirm prune command

	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return !strings.Contains(s, "old-container")
	})
	waitForString(t, tm, "nginx-proxy") // running containers remain
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// Volumes.
func TestSwitchingSectionResetVolumesActiveView(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate to Volumes view (Images -> Containers -> Volumes)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	waitForString(t, tm, "postgres_data")
	// Switch away and back
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Networks")
	})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestVolumesView(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Wait for initial render
	waitForString(t, tm, "Images")

	// Navigate header right twice to switch to Volumes view
	// (Images -> Containers -> Volumes)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})

	// Wait for Volumes view to render
	waitForString(t, tm, "postgres_data")

	// Quit
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// Networks.
func TestSwitchingSectionResetNetworksActiveView(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate to Networks view (Images -> Containers -> Volumes -> Networks)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "bridge") && strings.Contains(s, "abc123def456")
	})
	// Switch away and back
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Volumes")
	})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestNetworkListDelete(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate to Networks view (Images -> Containers -> Volumes -> Networks)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	// Delete the selected network (bridge)
	tm.Send(tea.KeyPressMsg{Code: 'D', Text: "D"})
	// Modal title and hint both appear in the same render; check for both in one pass
	// to avoid consuming the output reader between two separate waitForString calls.
	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "Delete Network") && strings.Contains(s, "[y] confirm")
	})
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"}) // confirm delete command
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestAutoRefreshInvalidInterval(t *testing.T) {
	cfg := &config.Config{
		Refresh: config.RefreshConfig{Interval: "not-a-duration"},
	}
	m := New(context.Background(), "test", cfg, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Invalid interval should surface as an error banner
	waitForString(t, tm, "Invalid refresh interval")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestAutoRefreshValidInterval(t *testing.T) {
	cfg := &config.Config{
		Refresh: config.RefreshConfig{Interval: "500ms"},
	}
	m := New(context.Background(), "test", cfg, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// UI should still render normally with a valid interval configured
	waitForString(t, tm, "Images")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalAppearsOnDelete(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Default view is Images — wait for it
	waitForString(t, tm, "nginx")

	// Press 'd' to trigger delete — modal should appear
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})

	// Modal title and hint both appear in the same render; check for both in one pass
	// to avoid consuming the output reader between two separate waitForString calls.
	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "Delete Image") && strings.Contains(s, "[y] confirm")
	})

	// Dismiss the modal before quitting — 'q' is swallowed by the modal.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalDismissedOnN(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Trigger delete modal
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})
	waitForString(t, tm, "Delete Image")

	// Press 'n' to cancel — modal should disappear, images list should be visible again
	tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})
	waitForString(t, tm, "nginx")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalDismissedOnEsc(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Trigger delete modal
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})
	waitForString(t, tm, "Delete Image")

	// Press 'esc' to cancel
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	waitForString(t, tm, "nginx")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalConfirmDeletesImage(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Press 'd' to trigger delete on the first image (nginx:latest)
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})
	waitForString(t, tm, "Delete Image")

	// Confirm with 'y' — banner should show success
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})
	waitForString(t, tm, "deleted")

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContextualKeyBindingsBeforeViewRender(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())

	// Send AddContextualKeyBindingsMsg before activeKeys is set — must not panic.
	m.Update(message.AddContextualKeyBindingsMsg{
		Bindings: []key.Binding{key.NewBinding(key.WithKeys("x"))},
	})

	// Send ClearContextualKeyBindingsMsg before activeKeys is set — must not panic.
	m.Update(message.ClearContextualKeyBindingsMsg{})
}

func TestSpinnerOverlayFollowsShowAndCancelMessages(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.Update(message.ShowSpinnerMsg{
		ID:   "images",
		Text: "Refreshing...",
		Scope: message.SpinnerScope{
			Section: string(sections.ImagesSection),
		},
	})

	if !strings.Contains(appModel.View().Content, "Refreshing...") {
		t.Fatal("spinner overlay should render after ShowSpinnerMsg")
	}

	appModel.Update(message.CancelSpinnerMsg{ID: "images"})

	if strings.Contains(appModel.View().Content, "Refreshing...") {
		t.Fatal("spinner overlay should disappear after CancelSpinnerMsg")
	}
}

func TestSpinnerOverlayHidesWhenActiveSectionHasNoSpinner(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.Update(message.ShowSpinnerMsg{
		ID:   "images",
		Text: "Refreshing...",
		Scope: message.SpinnerScope{
			Section: string(sections.ImagesSection),
		},
	})
	appModel.Update(message.ShowSpinnerMsg{
		ID:   "containers",
		Text: "Loading containers...",
		Scope: message.SpinnerScope{
			Section: string(sections.ContainersSection),
		},
	})
	appModel.Update(message.CancelSpinnerMsg{ID: "images"})

	if text := appModel.activeSpinnerText(); text != "" {
		t.Fatalf("activeSpinnerText() = %q, want empty string", text)
	}
}

func TestSpinnerOverlayIgnoresNestedActiveSpinner(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	appModel.Update(message.ShowSpinnerMsg{
		ID:   "containers.files.1",
		Text: "Loading files...",
		Scope: message.SpinnerScope{
			Section: string(sections.ContainersSection),
			Panel:   "Files",
		},
	})

	if text := appModel.activeSpinnerText(); text != "" {
		t.Fatalf("activeSpinnerText() = %q, want empty string", text)
	}
}

func TestSpinnerOverlayShowsNestedSpinnerForActivePanel(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	// Set focus on panels
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	appModel.Update(message.ShowSpinnerMsg{
		ID:   "containers.files.1",
		Text: "Loading files...",
		Scope: message.SpinnerScope{
			Section: string(sections.ContainersSection),
			Panel:   "Files",
		},
	})

	if text := appModel.activeSpinnerText(); text != "Loading files..." {
		t.Fatalf("activeSpinnerText() = %q, want %q", text, "Loading files...")
	}
}

func TestFilterModeBlocksGlobalShortcuts(t *testing.T) {
	m := New(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Default view is Images — wait for it
	waitForString(t, tm, "nginx")

	// Activate filter mode with '/'
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})

	// 'd' would normally open the delete confirmation modal; in filter mode it
	// should be treated as filter input, so no confirmation modal must appear.
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})

	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return !strings.Contains(s, "Delete Image")
	})

	// Exit filter mode with Esc, then quit normally.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContextualKeyBindingsProcessedDuringFilterMode(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	// activeKeys is set lazily inside View(); trigger it.
	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.View()

	// Activate filter on the active section.
	appModel.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	if !appModel.isFilterActive() {
		t.Fatal("filter mode did not activate")
	}

	if appModel.activeKeys == nil {
		t.Fatal("activeKeys should not be nil after View()")
	}

	sentinel := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "sentinel"))

	// AddContextualKeyBindingsMsg must be processed even while filter is active.
	appModel.Update(message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{sentinel}})

	shortHelp := appModel.activeKeys.ShortHelp()
	found := false
	for _, b := range shortHelp {
		if slices.Contains(b.Keys(), "x") {
			found = true
			break
		}
	}
	if !found {
		t.Error("contextual binding should appear in ShortHelp after AddContextualKeyBindingsMsg during filter mode")
	}

	// ClearContextualKeyBindingsMsg must also be processed while filter is active.
	appModel.Update(message.ClearContextualKeyBindingsMsg{})

	shortHelp = appModel.activeKeys.ShortHelp()
	for _, b := range shortHelp {
		if slices.Contains(b.Keys(), "x") {
			t.Error(
				"contextual binding should be gone from ShortHelp after ClearContextualKeyBindingsMsg during filter mode",
			)
			break
		}
	}
}

func TestArrowKeysStayInContainersWhenLogsPanelFocused(t *testing.T) {
	appModel, ok := New(context.Background(), "test", &config.Config{}, client.NewMockClient()).(*model)
	if !ok {
		t.Fatal("New should return *model")
	}

	appModel.Update(tea.WindowSizeMsg{Width: 300, Height: 100})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // Images -> Containers
	// Set focus on panels
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}) // Details -> Logs panel

	section := appModel.activeSection()
	if section.ActivePanelName() != "Logs" {
		t.Fatalf("active panel = %q, want Logs", section.ActivePanelName())
	}
	if !section.IsPanelFocused() {
		t.Fatal("expected panel focus to be active")
	}

	appModel.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if appModel.header.ActiveView() != header.ViewContainers {
		t.Fatal("right arrow should not switch header when logs panel is focused")
	}

	appModel.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if appModel.header.ActiveView() != header.ViewContainers {
		t.Fatal("left arrow should not switch header when logs panel is focused")
	}
}

func waitForString(t *testing.T, tm *teatest.TestModel, s string) {
	teatest.WaitFor(
		t,
		tm.Output(),
		func(b []byte) bool {
			return strings.Contains(string(b), s)
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*10),
	)
}

func waitFor(t *testing.T, tm *teatest.TestModel, f func(b []byte) bool) {
	teatest.WaitFor(
		t,
		tm.Output(),
		f,
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*10),
	)
}
