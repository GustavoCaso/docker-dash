package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
)

func TestFullOutput(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestExecPanel(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Wait for initial render
	waitForString(t, tm, "Images")

	// Navigate header right to switch to Containers view (default is Images)
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	// Wait for Containers view to render
	waitForString(t, tm, "nginx-proxy")

	// Navigate to first container (should be running)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Press 'e' to open exec panel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	// Wait for the exec input prompt to appear
	waitForString(t, tm, "$")

	// Close exec panel and quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerStatsOnStoppedContainer(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch to Containers view
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForString(t, tm, "nginx-proxy")
	// Navigate to old-container (stopped, last in list)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Try to open stats on stopped container
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	waitForString(t, tm, "Container is not running")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestContainerLogsOnStoppedContainer(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch to Containers view
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForString(t, tm, "nginx-proxy")
	// Navigate to old-container (stopped, last in list)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Try to open stats on stopped container
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	waitForString(t, tm, "Container is not running")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSwitchingSectionResetActiveView(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch to Containers view
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	waitForString(t, tm, "Layers")
	// Swicth section
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	// Swicth back
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Layers")
	})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSwitchingSectionResetContainersActiveView(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Switch to Containers view
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForString(t, tm, "nginx-proxy")
	// Select a container and open logs panel
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	waitForString(t, tm, "Starting application")
	// Switch away and back
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Starting application")
	})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSwitchingSectionResetVolumesActiveView(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate to Volumes view (Images -> Containers -> Volumes)
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForString(t, tm, "postgres_data")
	// Open file tree panel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	waitForString(t, tm, "pgdata")
	// Switch away and back
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "pgdata")
	})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSwitchingSectionResetNetworksActiveView(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	// Navigate to Networks view (Images -> Containers -> Volumes -> Networks)
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForString(t, tm, "bridge")
	// Open details panel
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitForString(t, tm, "Network:")
	// Switch away and back
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitFor(t, tm, func(b []byte) bool {
		return !strings.Contains(string(b), "Network:")
	})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestVolumesView(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Wait for initial render
	waitForString(t, tm, "Images")

	// Navigate header right twice to switch to Volumes view
	// (Images -> Containers -> Volumes)
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	// Wait for Volumes view to render
	waitForString(t, tm, "postgres_data")

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestAutoRefreshInvalidInterval(t *testing.T) {
	cfg := &config.Config{
		Refresh: config.RefreshConfig{Interval: "not-a-duration"},
	}
	m := InitialModel(context.Background(), "test", cfg, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Invalid interval should surface as an error banner
	waitForString(t, tm, "Invalid refresh interval")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestAutoRefreshValidInterval(t *testing.T) {
	cfg := &config.Config{
		Refresh: config.RefreshConfig{Interval: "500ms"},
	}
	m := InitialModel(context.Background(), "test", cfg, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// UI should still render normally with a valid interval configured
	waitForString(t, tm, "Images")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalAppearsOnDelete(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// Default view is Images — wait for it
	waitForString(t, tm, "nginx")

	// Press 'd' to trigger delete — modal should appear
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	// Modal title and hint both appear in the same render; check for both in one pass
	// to avoid consuming the output reader between two separate waitForString calls.
	waitFor(t, tm, func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "Delete Image") && strings.Contains(s, "[y] confirm")
	})

	// Dismiss the modal before quitting — 'q' is swallowed by the modal.
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalDismissedOnN(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Trigger delete modal
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitForString(t, tm, "Delete Image")

	// Press 'n' to cancel — modal should disappear, images list should be visible again
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	waitForString(t, tm, "nginx")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalDismissedOnEsc(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Trigger delete modal
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitForString(t, tm, "Delete Image")

	// Press 'esc' to cancel
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForString(t, tm, "nginx")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestConfirmationModalConfirmDeletesImage(t *testing.T) {
	m := InitialModel(context.Background(), "test", &config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	waitForString(t, tm, "nginx")

	// Press 'd' to trigger delete on the first image (nginx:latest)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitForString(t, tm, "Delete Image")

	// Confirm with 'y' — banner should show success
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	waitForString(t, tm, "deleted")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
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
