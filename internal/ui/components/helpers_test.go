package components

import (
	"testing"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"strips sha256 prefix", "sha256:abc123def456789", "abc123def456"},
		{"truncates long ID without prefix", "abc123def456789", "abc123def456"},
		{"short ID unchanged", "abc123", "abc123"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortID(tt.id)
			if got != tt.want {
				t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 142 * 1024 * 1024, "142.0 MB"},
		{"gigabytes", 2 * 1024 * 1024 * 1024, "2.0 GB"},
		{"zero", 0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestTruncateCommand(t *testing.T) {
	tests := []struct {
		name   string
		cmd    string
		maxLen int
		want   string
	}{
		{"strips /bin/sh -c prefix", "/bin/sh -c echo hello", 50, "echo hello"},
		{"strips #(nop) prefix", "#(nop) CMD [\"nginx\"]", 50, "CMD [\"nginx\"]"},
		{"truncates long string", "this is a very long command that exceeds the limit", 20, "this is a very lo..."},
		{"short command unchanged", "ls", 50, "ls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateCommand(tt.cmd, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateCommand(%q, %d) = %q, want %q", tt.cmd, tt.maxLen, got, tt.want)
			}
		})
	}
}
