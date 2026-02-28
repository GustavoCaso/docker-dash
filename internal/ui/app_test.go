package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
)

func TestFullOutput(t *testing.T) {
	m := InitialModel(&config.Config{}, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Images")
	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestExecPanel(t *testing.T) {
	m := InitialModel(&config.Config{}, client.NewMockClient())
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
	m := InitialModel(&config.Config{}, client.NewMockClient())
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

func TestVolumesView(t *testing.T) {
	m := InitialModel(&config.Config{}, client.NewMockClient())
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
	m := InitialModel(cfg, client.NewMockClient())
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
	m := InitialModel(cfg, client.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	// UI should still render normally with a valid interval configured
	waitForString(t, tm, "Images")

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
