package helper

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayBottomRight places text in the bottom right corner of content.
//
// The challenge: terminal text contains ANSI escape codes for styling (colors,
// bold, etc.) which take bytes but have zero visual width. For example:
//
//	"\x1b[32mHello\x1b[0m" displays as "Hello" (5 chars wide) but is 14 bytes
//
// Using len() would give wrong results. We use lipgloss.Width() for visual
// width and ansi.Truncate() to cut at visual width while preserving ANSI codes.
//
// Example with width=50 and overlay width=20:
//
//	Original: "│ image1:latest    │ 500MB    │ unused │"  (visual width 50)
//	Truncate to 30 chars, append 20-char overlay:
//	Result:   "│ image1:latest    │ 500MB [Success: Deleted]"
//
// The truncation preserves any ANSI styling in the original line.
func OverlayBottomRight(lastLineIdx int, content, overlay string, width int) string {
	lines := strings.Split(content, "\n")
	if len(lines) < lastLineIdx {
		return content
	}

	// Target second-to-last line (above the bottom border)
	targetIdx := len(lines) - lastLineIdx
	targetLine := lines[targetIdx]
	overlayWidth := lipgloss.Width(overlay)
	targetLineWidth := lipgloss.Width(targetLine)
	padding := width - targetLineWidth - overlayWidth

	if padding > 0 {
		// There's room - add padding between content and overlay
		lines[targetIdx] = targetLine + strings.Repeat(" ", padding) + overlay
	} else {
		// No room for padding - truncate the line to fit the overlay
		truncatedWidth := width - overlayWidth
		if truncatedWidth > 0 {
			lines[targetIdx] = ansi.Truncate(targetLine, truncatedWidth, "") + overlay
		} else {
			// Not enough room - just show the overlay (truncated to fit if needed)
			lines[targetIdx] = ansi.Truncate(overlay, width, "")
		}
	}

	return strings.Join(lines, "\n")
}
