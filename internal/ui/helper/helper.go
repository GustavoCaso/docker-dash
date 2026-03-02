package helper

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const shortIDLength = 12 // length of shortened container/image IDs

// OverlayBottomRight places text in the bottom right corner of content.
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

// ShortID returns first 12 characters of an ID.
func ShortID(id string) string {
	// Remove sha256: prefix if present
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > shortIDLength {
		return id[:shortIDLength]
	}
	return id
}

func FormatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// TruncateCommand shortens a command string.
func TruncateCommand(cmd string, maxLen int) string {
	// Clean up common prefixes
	cmd = strings.TrimPrefix(cmd, "/bin/sh -c ")
	cmd = strings.TrimPrefix(cmd, "#(nop) ")
	cmd = strings.TrimSpace(cmd)

	if len(cmd) > maxLen {
		return cmd[:maxLen-3] + "..."
	}
	return cmd
}
