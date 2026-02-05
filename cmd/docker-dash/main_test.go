package main

import (
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/docker-dash/internal/service"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestFullOutput(t *testing.T) {
	m := initialModel(service.NewMockClient())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	waitForString(t, tm, "Docker")
	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})
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
