package service

import (
	"testing"
)

func TestParseSSHTarget(t *testing.T) {
	tests := []struct {
		name           string
		rawURL         string
		wantUser       string
		wantSshAddr    string
		wantSocketPath string
		wantErr        bool
	}{
		{
			name:           "standard ssh url",
			rawURL:         "ssh://pi@raspberrypi.local",
			wantUser:       "pi",
			wantSshAddr:    "raspberrypi.local:22",
			wantSocketPath: "/var/run/docker.sock",
		},
		{
			name:           "custom port",
			rawURL:         "ssh://pi@raspberrypi.local:2222",
			wantUser:       "pi",
			wantSshAddr:    "raspberrypi.local:2222",
			wantSocketPath: "/var/run/docker.sock",
		},
		{
			name:           "custom socket path",
			rawURL:         "ssh://pi@raspberrypi.local/run/user/1000/docker.sock",
			wantUser:       "pi",
			wantSshAddr:    "raspberrypi.local:22",
			wantSocketPath: "/run/user/1000/docker.sock",
		},
		{
			name:           "no user",
			rawURL:         "ssh://raspberrypi.local",
			wantUser:       "",
			wantSshAddr:    "raspberrypi.local:22",
			wantSocketPath: "/var/run/docker.sock",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user, sshAddr, socketPath, err := parseSSHTarget(tc.rawURL)
			if tc.wantErr {
				if err == nil {
					t.Error("parseSSHTarget() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSSHTarget() unexpected error: %v", err)
			}
			if user != tc.wantUser {
				t.Errorf("user = %q, want %q", user, tc.wantUser)
			}
			if sshAddr != tc.wantSshAddr {
				t.Errorf("sshAddr = %q, want %q", sshAddr, tc.wantSshAddr)
			}
			if socketPath != tc.wantSocketPath {
				t.Errorf("socketPath = %q, want %q", socketPath, tc.wantSocketPath)
			}
		})
	}
}

func TestIsSSHHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"ssh://pi@host", true},
		{"ssh://host", true},
		{"tcp://host:2375", false},
		{"unix:///var/run/docker.sock", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isSSHHost(tc.host)
		if got != tc.want {
			t.Errorf("isSSHHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}
