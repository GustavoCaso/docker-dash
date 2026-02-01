package components_test

import (
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/ui/components"
)

func TestSidebar_View(t *testing.T) {
	sidebar := components.NewSidebar()
	sidebar.SetHeight(20)

	view := sidebar.View()

	// Verify view contains Containers, Images, Volumes items
	tests := []struct {
		name     string
		contains string
	}{
		{"Containers item", "Containers"},
		{"Images item", "Images"},
		{"Volumes item", "Volumes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(view, tt.contains) {
				t.Errorf("View() does not contain %q", tt.contains)
			}
		})
	}
}

func TestSidebar_Navigation(t *testing.T) {
	sidebar := components.NewSidebar()

	// Initial state: first item (Containers) is selected
	if sidebar.ActiveIndex() != 0 {
		t.Errorf("Initial ActiveIndex() = %d, want 0", sidebar.ActiveIndex())
	}

	// Test MoveDown
	sidebar.MoveDown()
	if sidebar.ActiveIndex() != 1 {
		t.Errorf("After MoveDown(), ActiveIndex() = %d, want 1", sidebar.ActiveIndex())
	}

	sidebar.MoveDown()
	if sidebar.ActiveIndex() != 2 {
		t.Errorf("After second MoveDown(), ActiveIndex() = %d, want 2", sidebar.ActiveIndex())
	}

	// Test wrapping at the bottom - should wrap to first item
	sidebar.MoveDown()
	if sidebar.ActiveIndex() != 0 {
		t.Errorf("After wrapping MoveDown(), ActiveIndex() = %d, want 0 (wrap)", sidebar.ActiveIndex())
	}

	// Test MoveUp wrapping from first item - should go to last item
	sidebar.MoveUp()
	if sidebar.ActiveIndex() != 2 {
		t.Errorf("After wrapping MoveUp(), ActiveIndex() = %d, want 2 (wrap)", sidebar.ActiveIndex())
	}

	// Test normal MoveUp
	sidebar.MoveUp()
	if sidebar.ActiveIndex() != 1 {
		t.Errorf("After MoveUp(), ActiveIndex() = %d, want 1", sidebar.ActiveIndex())
	}
}

func TestSidebar_ActiveView(t *testing.T) {
	sidebar := components.NewSidebar()

	tests := []struct {
		name     string
		index    int
		expected components.View
	}{
		{"Containers view at index 0", 0, components.ViewContainers},
		{"Images view at index 1", 1, components.ViewImages},
		{"Volumes view at index 2", 2, components.ViewVolumes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Navigate to the desired index
			sidebar = components.NewSidebar()
			for i := 0; i < tt.index; i++ {
				sidebar.MoveDown()
			}

			if sidebar.ActiveView() != tt.expected {
				t.Errorf("ActiveView() = %v, want %v", sidebar.ActiveView(), tt.expected)
			}
		})
	}
}

func TestSidebar_Focus(t *testing.T) {
	sidebar := components.NewSidebar()

	// Default should not be focused
	if sidebar.IsFocused() {
		t.Error("Initial IsFocused() = true, want false")
	}

	// Set focused
	sidebar.SetFocused(true)
	if !sidebar.IsFocused() {
		t.Error("After SetFocused(true), IsFocused() = false, want true")
	}

	// Unset focused
	sidebar.SetFocused(false)
	if sidebar.IsFocused() {
		t.Error("After SetFocused(false), IsFocused() = true, want false")
	}
}

func TestSidebar_SetHeight(t *testing.T) {
	sidebar := components.NewSidebar()

	// Set height and verify it affects rendering (no panic)
	sidebar.SetHeight(10)
	view := sidebar.View()
	if view == "" {
		t.Error("View() returned empty string after SetHeight")
	}

	sidebar.SetHeight(50)
	view = sidebar.View()
	if view == "" {
		t.Error("View() returned empty string after SetHeight(50)")
	}
}
