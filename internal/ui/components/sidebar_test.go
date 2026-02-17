package components

import (
	"strings"
	"testing"
)

func TestSidebarNewDefaults(t *testing.T) {
	s := NewSidebar()

	if s.ActiveView() != ViewImages {
		t.Errorf("expected initial ActiveView to be ViewImages, got %v", s.ActiveView())
	}

	if s.IsFocused() {
		t.Error("expected initial IsFocused to be false")
	}
}

func TestSidebarNavigation(t *testing.T) {
	s := NewSidebar()

	// Start at index 0 = ViewImages
	if s.ActiveView() != ViewImages {
		t.Fatalf("expected ViewImages at start, got %v", s.ActiveView())
	}

	// MoveDown -> ViewContainers
	s.MoveDown()
	if s.ActiveView() != ViewContainers {
		t.Errorf("expected ViewContainers after MoveDown, got %v", s.ActiveView())
	}

	// MoveDown again -> ViewVolumes
	s.MoveDown()
	if s.ActiveView() != ViewVolumes {
		t.Errorf("expected ViewVolumes after second MoveDown, got %v", s.ActiveView())
	}

	// MoveDown again -> wraps to ViewImages
	s.MoveDown()
	if s.ActiveView() != ViewImages {
		t.Errorf("expected ViewImages after wrapping MoveDown, got %v", s.ActiveView())
	}
}

func TestSidebarNavigationUp(t *testing.T) {
	s := NewSidebar()

	// MoveUp from index 0 -> wraps to ViewVolumes (last item)
	s.MoveUp()
	if s.ActiveView() != ViewVolumes {
		t.Errorf("expected ViewVolumes after MoveUp from start, got %v", s.ActiveView())
	}

	// MoveUp again -> ViewContainers
	s.MoveUp()
	if s.ActiveView() != ViewContainers {
		t.Errorf("expected ViewContainers after second MoveUp, got %v", s.ActiveView())
	}

	// MoveUp again -> ViewImages
	s.MoveUp()
	if s.ActiveView() != ViewImages {
		t.Errorf("expected ViewImages after third MoveUp, got %v", s.ActiveView())
	}
}

func TestSidebarFocus(t *testing.T) {
	s := NewSidebar()

	s.SetFocused(true)
	if !s.IsFocused() {
		t.Error("expected IsFocused to be true after SetFocused(true)")
	}

	s.SetFocused(false)
	if s.IsFocused() {
		t.Error("expected IsFocused to be false after SetFocused(false)")
	}
}

func TestSidebarViewRendering(t *testing.T) {
	s := NewSidebar()
	s.SetSize(24, 40)

	output := s.View()

	for _, expected := range []string{"Docker", "Images", "Containers", "Volumes"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected View() output to contain %q", expected)
		}
	}
}

func TestViewString(t *testing.T) {
	tests := []struct {
		view     View
		expected string
	}{
		{ViewContainers, "Containers"},
		{ViewImages, "Images"},
		{ViewVolumes, "Volumes"},
		{View(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.view.String(); got != tt.expected {
			t.Errorf("View(%d).String() = %q, want %q", int(tt.view), got, tt.expected)
		}
	}
}
