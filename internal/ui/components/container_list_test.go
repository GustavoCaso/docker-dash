package components_test

import (
	"strings"
	"testing"
	"time"

	"github.com/GustavoCaso/docker-dash/internal/service"
	"github.com/GustavoCaso/docker-dash/internal/ui/components"
)

// Helper to create test containers
func createTestContainers() []service.Container {
	return []service.Container{
		{
			ID:     "abc123def456",
			Name:   "web-server",
			Image:  "nginx:latest",
			Status: "Up 2 hours",
			State:  service.StateRunning,
			Created: time.Now().Add(-2 * time.Hour),
			Ports: []service.PortMapping{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
			Mounts: []service.Mount{
				{Type: "bind", Source: "/app", Destination: "/var/www"},
			},
		},
		{
			ID:      "xyz789ghi012",
			Name:    "database",
			Image:   "postgres:15",
			Status:  "Exited (0) 5 minutes ago",
			State:   service.StateStopped,
			Created: time.Now().Add(-24 * time.Hour),
			Ports:   []service.PortMapping{},
			Mounts:  []service.Mount{},
		},
		{
			ID:     "mno345pqr678",
			Name:   "cache",
			Image:  "redis:7",
			Status: "Up 30 minutes",
			State:  service.StateRunning,
			Created: time.Now().Add(-30 * time.Minute),
			Ports: []service.PortMapping{
				{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"},
			},
			Mounts: []service.Mount{},
		},
	}
}

func TestContainerList_View(t *testing.T) {
	containers := createTestContainers()
	list := components.NewContainerList(containers)
	list.SetSize(80, 20)

	view := list.View()

	// Verify container names appear in view
	tests := []struct {
		name     string
		contains string
	}{
		{"web-server container", "web-server"},
		{"database container", "database"},
		{"cache container", "cache"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(view, tt.contains) {
				t.Errorf("View() does not contain %q", tt.contains)
			}
		})
	}
}

func TestContainerList_Navigation(t *testing.T) {
	containers := createTestContainers()
	list := components.NewContainerList(containers)

	// Initial state: first container is selected
	if list.SelectedIndex() != 0 {
		t.Errorf("Initial SelectedIndex() = %d, want 0", list.SelectedIndex())
	}

	// Test MoveDown
	list.MoveDown()
	if list.SelectedIndex() != 1 {
		t.Errorf("After MoveDown(), SelectedIndex() = %d, want 1", list.SelectedIndex())
	}

	list.MoveDown()
	if list.SelectedIndex() != 2 {
		t.Errorf("After second MoveDown(), SelectedIndex() = %d, want 2", list.SelectedIndex())
	}

	// Test wrapping at the bottom - should wrap to first item
	list.MoveDown()
	if list.SelectedIndex() != 0 {
		t.Errorf("After wrapping MoveDown(), SelectedIndex() = %d, want 0 (wrap)", list.SelectedIndex())
	}

	// Test MoveUp wrapping from first item - should go to last item
	list.MoveUp()
	if list.SelectedIndex() != 2 {
		t.Errorf("After wrapping MoveUp(), SelectedIndex() = %d, want 2 (wrap)", list.SelectedIndex())
	}

	// Test normal MoveUp
	list.MoveUp()
	if list.SelectedIndex() != 1 {
		t.Errorf("After MoveUp(), SelectedIndex() = %d, want 1", list.SelectedIndex())
	}
}

func TestContainerList_Expand(t *testing.T) {
	containers := createTestContainers()
	list := components.NewContainerList(containers)
	list.SetSize(80, 30)

	// Initially nothing should be expanded
	if list.IsExpanded(0) {
		t.Error("Initial IsExpanded(0) = true, want false")
	}

	// Toggle expand on first container
	list.ToggleExpand()
	if !list.IsExpanded(0) {
		t.Error("After ToggleExpand(), IsExpanded(0) = false, want true")
	}

	// Verify ID shows when expanded
	view := list.View()
	if !strings.Contains(view, "abc123def456") {
		t.Error("Expanded view does not contain container ID 'abc123def456'")
	}

	// Verify Image shows when expanded
	if !strings.Contains(view, "nginx:latest") {
		t.Error("Expanded view does not contain image 'nginx:latest'")
	}

	// Toggle expand again should collapse
	list.ToggleExpand()
	if list.IsExpanded(0) {
		t.Error("After second ToggleExpand(), IsExpanded(0) = true, want false")
	}

	// Navigate to second container and expand
	list.MoveDown()
	list.ToggleExpand()
	if !list.IsExpanded(1) {
		t.Error("After MoveDown and ToggleExpand, IsExpanded(1) = false, want true")
	}
	if list.IsExpanded(0) {
		t.Error("First container should not be expanded when second is expanded")
	}
}

func TestContainerList_SelectedContainer(t *testing.T) {
	containers := createTestContainers()
	list := components.NewContainerList(containers)

	// Test SelectedContainer at initial position
	selected := list.SelectedContainer()
	if selected == nil {
		t.Fatal("SelectedContainer() returned nil")
	}
	if selected.Name != "web-server" {
		t.Errorf("SelectedContainer().Name = %q, want %q", selected.Name, "web-server")
	}

	// Navigate and verify selection changes
	list.MoveDown()
	selected = list.SelectedContainer()
	if selected == nil {
		t.Fatal("SelectedContainer() returned nil after MoveDown")
	}
	if selected.Name != "database" {
		t.Errorf("SelectedContainer().Name = %q, want %q", selected.Name, "database")
	}

	// Test with empty list
	emptyList := components.NewContainerList([]service.Container{})
	if emptyList.SelectedContainer() != nil {
		t.Error("SelectedContainer() on empty list should return nil")
	}
}

func TestContainerList_SetContainers(t *testing.T) {
	list := components.NewContainerList([]service.Container{})

	if list.SelectedIndex() != 0 {
		t.Errorf("Empty list SelectedIndex() = %d, want 0", list.SelectedIndex())
	}

	// Set new containers
	containers := createTestContainers()
	list.SetContainers(containers)

	// Verify containers were set
	selected := list.SelectedContainer()
	if selected == nil {
		t.Fatal("SelectedContainer() returned nil after SetContainers")
	}
	if selected.Name != "web-server" {
		t.Errorf("SelectedContainer().Name = %q, want %q", selected.Name, "web-server")
	}
}

func TestContainerList_Focus(t *testing.T) {
	list := components.NewContainerList([]service.Container{})

	// Default should not be focused
	if list.IsFocused() {
		t.Error("Initial IsFocused() = true, want false")
	}

	// Set focused
	list.SetFocused(true)
	if !list.IsFocused() {
		t.Error("After SetFocused(true), IsFocused() = false, want true")
	}

	// Unset focused
	list.SetFocused(false)
	if list.IsFocused() {
		t.Error("After SetFocused(false), IsFocused() = true, want false")
	}
}

func TestContainerList_Actions(t *testing.T) {
	containers := createTestContainers()
	list := components.NewContainerList(containers)
	list.SetSize(80, 30)

	// Expand to show actions
	list.ToggleExpand()

	// Initially actions should not be focused
	if list.ActionsFocused() {
		t.Error("Initial ActionsFocused() = true, want false")
	}

	// Set actions focused
	list.SetActionsFocused(true)
	if !list.ActionsFocused() {
		t.Error("After SetActionsFocused(true), ActionsFocused() = false, want true")
	}

	// Initial action index should be 0
	if list.SelectedAction() != 0 {
		t.Errorf("Initial SelectedAction() = %d, want 0", list.SelectedAction())
	}

	// Move right
	list.MoveActionRight()
	if list.SelectedAction() != 1 {
		t.Errorf("After MoveActionRight(), SelectedAction() = %d, want 1", list.SelectedAction())
	}

	// Move left
	list.MoveActionLeft()
	if list.SelectedAction() != 0 {
		t.Errorf("After MoveActionLeft(), SelectedAction() = %d, want 0", list.SelectedAction())
	}

	// Move left should not go below 0
	list.MoveActionLeft()
	if list.SelectedAction() != 0 {
		t.Errorf("After MoveActionLeft() at 0, SelectedAction() = %d, want 0", list.SelectedAction())
	}
}

func TestContainerList_GetActionsForRunningContainer(t *testing.T) {
	// Running container should have: Logs, Shell, Stop, Restart, Remove
	containers := []service.Container{
		{
			ID:    "abc123",
			Name:  "running-container",
			State: service.StateRunning,
		},
	}
	list := components.NewContainerList(containers)
	list.ToggleExpand()
	list.SetActionsFocused(true)
	list.SetSize(80, 30)

	view := list.View()

	// Check running container actions
	expectedActions := []string{"Logs", "Shell", "Stop", "Restart", "Remove"}
	for _, action := range expectedActions {
		if !strings.Contains(view, action) {
			t.Errorf("Running container view should contain action %q", action)
		}
	}
}

func TestContainerList_GetActionsForStoppedContainer(t *testing.T) {
	// Stopped container should have: Start, Remove
	containers := []service.Container{
		{
			ID:    "xyz789",
			Name:  "stopped-container",
			State: service.StateStopped,
		},
	}
	list := components.NewContainerList(containers)
	list.ToggleExpand()
	list.SetActionsFocused(true)
	list.SetSize(80, 30)

	view := list.View()

	// Check stopped container actions
	if !strings.Contains(view, "Start") {
		t.Error("Stopped container view should contain action 'Start'")
	}
	if !strings.Contains(view, "Remove") {
		t.Error("Stopped container view should contain action 'Remove'")
	}
	// Should NOT have running container actions
	if strings.Contains(view, "Shell") {
		t.Error("Stopped container view should NOT contain action 'Shell'")
	}
	if strings.Contains(view, "Logs") {
		t.Error("Stopped container view should NOT contain action 'Logs'")
	}
}

func TestContainerList_RunningAndStoppedCount(t *testing.T) {
	containers := createTestContainers() // 2 running, 1 stopped
	list := components.NewContainerList(containers)

	runningCount := list.RunningCount()
	if runningCount != 2 {
		t.Errorf("RunningCount() = %d, want 2", runningCount)
	}

	stoppedCount := list.StoppedCount()
	if stoppedCount != 1 {
		t.Errorf("StoppedCount() = %d, want 1", stoppedCount)
	}
}

func TestContainerList_SetSize(t *testing.T) {
	list := components.NewContainerList([]service.Container{})

	// Set size and verify it affects rendering (no panic)
	list.SetSize(100, 50)
	view := list.View()
	if view == "" {
		t.Error("View() returned empty string after SetSize")
	}
}

func TestContainerList_ViewShowsStatusAndPorts(t *testing.T) {
	containers := []service.Container{
		{
			ID:     "test123",
			Name:   "test-container",
			Image:  "test:latest",
			Status: "Up 2 hours",
			State:  service.StateRunning,
			Ports: []service.PortMapping{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
		},
	}
	list := components.NewContainerList(containers)
	list.SetSize(80, 20)

	view := list.View()

	// Verify status is shown
	if !strings.Contains(view, "Up 2 hours") {
		t.Error("View() should contain status 'Up 2 hours'")
	}

	// Verify ports are shown
	if !strings.Contains(view, "8080") {
		t.Error("View() should contain port '8080'")
	}
}

func TestContainerList_ExpandedViewShowsDetails(t *testing.T) {
	containers := []service.Container{
		{
			ID:     "fullid12345",
			Name:   "detailed-container",
			Image:  "myimage:v1",
			Status: "Up 1 hour",
			State:  service.StateRunning,
			Ports: []service.PortMapping{
				{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
			},
			Mounts: []service.Mount{
				{Type: "bind", Source: "/host/path", Destination: "/container/path"},
			},
		},
	}
	list := components.NewContainerList(containers)
	list.SetSize(80, 30)
	list.ToggleExpand()

	view := list.View()

	// Check expanded details
	tests := []struct {
		name     string
		contains string
	}{
		{"Container ID", "fullid12345"},
		{"Image", "myimage:v1"},
		{"Status", "Up 1 hour"},
		{"Ports", "3000"},
		{"Mount source", "/host/path"},
		{"Mount destination", "/container/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(view, tt.contains) {
				t.Errorf("Expanded view does not contain %s: %q", tt.name, tt.contains)
			}
		})
	}
}
