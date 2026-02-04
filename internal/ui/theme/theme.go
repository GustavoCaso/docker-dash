// Package theme provides Docker Desktop inspired colors, Nerd Font icons,
// and Lip Gloss styles for the docker-dash TUI application.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Docker Desktop inspired color palette
var (
	// Primary Docker colors
	DockerBlue = lipgloss.Color("#1D63ED")
	DockerDark = lipgloss.Color("#0B1929")

	// Status colors
	StatusRunning = lipgloss.Color("#2ECC71")
	StatusStopped = lipgloss.Color("#6C7A89")
	StatusError   = lipgloss.Color("#E74C3C")
	StatusPaused  = lipgloss.Color("#F39C12")

	// UI colors
	TextPrimary   = lipgloss.Color("#FFFFFF")
	TextSecondary = lipgloss.Color("#A0AEC0")
	TextMuted     = lipgloss.Color("#6C7A89")
	Border        = lipgloss.Color("#2D3748")
	BorderActive  = lipgloss.Color("#1D63ED")
	Background    = lipgloss.Color("#0D1117")
	Highlight     = lipgloss.Color("#1A365D")
)

// Nerd Font icon constants
// These require a Nerd Font patched terminal font to display correctly
const (
	// Docker related icons
	IconDocker    = "\uf308" // Docker whale icon
	IconContainer = "\uf4b7" // Container/cube icon
	IconImage     = "\ue7ba" // Layers/image icon
	IconVolume    = "\uf7c2" // Database/volume icon
	IconNetwork   = "\uf6ff" // Network icon

	// Status icons
	IconRunning = "\uf04b" // Play icon (running)
	IconStopped = "\uf04d" // Stop icon (stopped)
	IconPaused  = "\uf04c" // Pause icon
	IconError   = "\uf00d" // X/error icon

	// Tree navigation icons
	IconExpanded  = "\uf0d7" // Chevron down
	IconCollapsed = "\uf0da" // Chevron right

	// File system icons
	IconFolder = "\uf07b" // Folder icon
	IconFile   = "\uf15b" // File icon

	// Alert icons
	IconWarning = "\uf071" // Warning triangle
	IconInfo    = "\uf05a" // Info circle
	IconSuccess = "\uf00c" // Checkmark
)

var LogoStyle = lipgloss.NewStyle().
	Foreground(DockerBlue).
	Bold(true).
	PaddingLeft(1).
	PaddingTop(1)

// Sidebar styles
var (
	SidebarStyle = lipgloss.NewStyle().
			Background(DockerDark).
			Border(lipgloss.HiddenBorder())

	SidebarItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				MarginBottom(1).
				Foreground(TextSecondary)

	SidebarActiveStyle = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(DockerBlue).
				Padding(0, 1).
				MarginBottom(1).
				Bold(true)
)

// Main panel styles
var (
	MainPanelStyle = lipgloss.NewStyle().
			Background(Background)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Border).
			MarginBottom(1).
			PaddingBottom(1)
)

// List item styles
var (
	ListItemStyle = lipgloss.NewStyle().
			Foreground(TextSecondary).
			Padding(0, 1)

	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(Highlight).
				Bold(true).
				Padding(0, 1)
)

// Detail panel styles
var (
	DetailStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	DetailLabelStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Width(16)

	DetailValueStyle = lipgloss.NewStyle().
				Foreground(TextPrimary)
)

// Status indicator styles
var (
	StatusRunningStyle = lipgloss.NewStyle().
				Foreground(StatusRunning).
				Bold(true)

	StatusStoppedStyle = lipgloss.NewStyle().
				Foreground(StatusStopped)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(StatusError).
				Bold(true)

	StatusPausedStyle = lipgloss.NewStyle().
				Foreground(StatusPaused)
)

// Action button styles
var (
	ActionButtonStyle = lipgloss.NewStyle().
				Foreground(TextSecondary).
				Background(DockerDark).
				Padding(0, 2).
				MarginRight(1).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(Border)

	ActionButtonActiveStyle = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(DockerBlue).
				Padding(0, 2).
				MarginRight(1).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(DockerBlue)
)

// Status bar styles
var (
	HelpStyle = lipgloss.NewStyle().Padding(0, 1)
)

// StatusStyle returns the appropriate style for a given container/resource state.
// Recognized states: "running", "stopped", "exited", "paused", "error", "dead", "created"
func StatusStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return StatusRunningStyle
	case "stopped", "exited":
		return StatusStoppedStyle
	case "paused":
		return StatusPausedStyle
	case "error", "dead":
		return StatusErrorStyle
	case "created":
		return StatusStoppedStyle
	default:
		return StatusStoppedStyle
	}
}

// StatusIcon returns the appropriate icon for a given container/resource state.
func StatusIcon(state string) string {
	switch state {
	case "running":
		return IconRunning
	case "stopped", "exited":
		return IconStopped
	case "paused":
		return IconPaused
	case "error", "dead":
		return IconError
	default:
		return IconStopped
	}
}
