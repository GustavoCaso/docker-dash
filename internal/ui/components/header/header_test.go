package header

import (
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

func TestHeaderNewDefaults(t *testing.T) {
	h := New()
	if h.ActiveView() != ViewImages {
		t.Errorf("expected initial ActiveView to be ViewImages, got %v", h.ActiveView())
	}
}

func TestHeaderMoveRight(t *testing.T) {
	h := New()

	if h.ActiveView() != ViewImages {
		t.Fatalf("expected ViewImages at start, got %v", h.ActiveView())
	}

	h.MoveRight()
	if h.ActiveView() != ViewContainers {
		t.Errorf("expected ViewContainers after MoveRight, got %v", h.ActiveView())
	}

	h.MoveRight()
	if h.ActiveView() != ViewVolumes {
		t.Errorf("expected ViewVolumes after second MoveRight, got %v", h.ActiveView())
	}

	h.MoveRight()
	if h.ActiveView() != ViewNetworks {
		t.Errorf("expected ViewNetworks after third MoveRight, got %v", h.ActiveView())
	}

	h.MoveRight()
	if h.ActiveView() != ViewImages {
		t.Errorf("expected ViewImages after wrapping MoveRight, got %v", h.ActiveView())
	}
}

func TestHeaderMoveLeft(t *testing.T) {
	h := New()

	h.MoveLeft()
	if h.ActiveView() != ViewNetworks {
		t.Errorf("expected ViewNetworks after MoveLeft from start, got %v", h.ActiveView())
	}

	h.MoveLeft()
	if h.ActiveView() != ViewVolumes {
		t.Errorf("expected ViewVolumes after second MoveLeft, got %v", h.ActiveView())
	}

	h.MoveLeft()
	if h.ActiveView() != ViewContainers {
		t.Errorf("expected ViewContainers after third MoveLeft, got %v", h.ActiveView())
	}

	h.MoveLeft()
	if h.ActiveView() != ViewImages {
		t.Errorf("expected ViewImages after fourth MoveLeft, got %v", h.ActiveView())
	}
}

func TestHeaderViewContainsSectionNames(t *testing.T) {
	h := New()
	h.SetWidth(120)

	output := h.View()

	for _, expected := range []string{"Images", "Containers", "Volumes", "Networks"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected View() output to contain %q", expected)
		}
	}
}

func TestHeaderViewContainsDockerIcon(t *testing.T) {
	h := New()
	h.SetWidth(120)

	output := h.View()

	if !strings.Contains(output, theme.IconDocker) {
		t.Error("expected View() output to contain Docker icon")
	}
}
