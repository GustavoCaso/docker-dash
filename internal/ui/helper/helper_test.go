package helper

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayBottomRight(t *testing.T) {
	t.Run("adds padding when line is shorter than width", func(t *testing.T) {
		// When the target line + overlay is shorter than width,
		// padding (spaces) is added to push the overlay to the right edge.
		//
		// Example:
		//   width = 20
		//   line  = "Hello"      (5 chars)
		//   overlay = "[OK]"     (4 chars)
		//   padding = 20 - 5 - 4 = 11 spaces
		//
		//   Before: "Hello"
		//   After:  "Hello           [OK]"
		//                 ^^^^^^^^^^^
		//                 11 spaces of padding

		content := "Line 1\nHello\nLine 3"
		overlay := "[OK]"
		width := 20

		result := OverlayBottomRight(2, content, overlay, width)
		lines := strings.Split(result, "\n")

		// Target line is second-to-last (index 1)
		targetLine := lines[1]

		// Should be: "Hello" + 11 spaces + "[OK]" = 20 chars total
		if lipgloss.Width(targetLine) != width {
			t.Errorf("expected width %d, got %d", width, lipgloss.Width(targetLine))
		}

		if !strings.HasPrefix(targetLine, "Hello") {
			t.Errorf("line should start with 'Hello', got: %q", targetLine)
		}
		if !strings.HasSuffix(targetLine, "[OK]") {
			t.Errorf("line should end with '[OK]', got: %q", targetLine)
		}
	})

	t.Run("truncates line when no room for padding", func(t *testing.T) {
		// When line + overlay exceeds width, the line is truncated
		// to make room for the overlay.
		//
		// Example:
		//   width = 20
		//   line  = "This is a long line!" (20 chars - fills entire width)
		//   overlay = "[OK]"               (4 chars)
		//   truncatedWidth = 20 - 4 = 16
		//
		//   Before: "This is a long line!"
		//   After:  "This is a long l[OK]"
		//           ^^^^^^^^^^^^^^^^
		//           truncated to 16 chars

		content := "Line 1\nThis is a long line!\nLine 3"
		overlay := "[OK]"
		width := 20

		result := OverlayBottomRight(2, content, overlay, width)
		lines := strings.Split(result, "\n")

		targetLine := lines[1]

		// Should be exactly width chars
		if lipgloss.Width(targetLine) != width {
			t.Errorf("expected width %d, got %d", width, lipgloss.Width(targetLine))
		}

		// Original content should be truncated
		if !strings.HasPrefix(targetLine, "This is a long l") {
			t.Errorf("line should start with truncated content, got: %q", targetLine)
		}
		if !strings.HasSuffix(targetLine, "[OK]") {
			t.Errorf("line should end with '[OK]', got: %q", targetLine)
		}
	})

	t.Run("shows only overlay when width is very small", func(t *testing.T) {
		// When width is smaller than the overlay itself, only the
		// overlay is shown (truncated if needed).
		//
		// Example:
		//   width = 3
		//   overlay = "[OK]" (4 chars)
		//   truncatedWidth = 3 - 4 = -1 (negative!)
		//
		//   Result: overlay truncated to 3 chars = "[OK"

		content := "Line 1\nHello\nLine 3"
		overlay := "[OK]"
		width := 3

		result := OverlayBottomRight(2, content, overlay, width)
		lines := strings.Split(result, "\n")

		targetLine := lines[1]

		// Should be truncated overlay only
		if lipgloss.Width(targetLine) != width {
			t.Errorf("expected width %d, got %d", width, lipgloss.Width(targetLine))
		}
	})

	t.Run("preserves ANSI escape codes when truncating", func(t *testing.T) {
		// ANSI escape codes (for colors, bold, etc.) take bytes but
		// have zero visual width. The function uses ansi.Truncate to
		// cut at visual width while preserving styling.
		//
		// Example:
		//   "\x1b[32mGreen\x1b[0m" displays as "Green" (5 chars)
		//   but is 14 bytes due to escape codes

		// Create a styled line using lipgloss
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("32"))
		styledText := style.Render("Hello World")

		content := "Line 1\n" + styledText + "\nLine 3"
		overlay := "[OK]"
		width := 15

		result := OverlayBottomRight(2, content, overlay, width)
		lines := strings.Split(result, "\n")

		targetLine := lines[1]

		// Visual width should be exactly 15
		if lipgloss.Width(targetLine) != width {
			t.Errorf("expected visual width %d, got %d", width, lipgloss.Width(targetLine))
		}

		// The line should end with the overlay
		if !strings.HasSuffix(targetLine, "[OK]") {
			t.Errorf("line should end with '[OK]', got: %q", targetLine)
		}
	})

	t.Run("targets correct line based on lastLineIdx", func(t *testing.T) {
		// lastLineIdx determines which line from the bottom to target:
		//   lastLineIdx=1 -> last line (index len-1)
		//   lastLineIdx=2 -> second-to-last line (index len-2)

		content := "Line 0\nLine 1\nLine 2\nLine 3"
		overlay := "[X]"
		width := 20

		// Target the last line (lastLineIdx=1)
		result1 := OverlayBottomRight(1, content, overlay, width)
		lines1 := strings.Split(result1, "\n")
		if !strings.Contains(lines1[3], "[X]") {
			t.Errorf("lastLineIdx=1 should modify last line, got: %v", lines1)
		}

		// Target second-to-last (lastLineIdx=2)
		result2 := OverlayBottomRight(2, content, overlay, width)
		lines2 := strings.Split(result2, "\n")
		if !strings.Contains(lines2[2], "[X]") {
			t.Errorf("lastLineIdx=2 should modify second-to-last line, got: %v", lines2)
		}
	})
}
