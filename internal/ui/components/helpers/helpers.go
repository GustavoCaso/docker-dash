package helpers

import (
	"fmt"
	"strings"
)

const shortIDLength = 12 // length of shortened container/image IDs

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
