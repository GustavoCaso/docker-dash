package service_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"os"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/service"
	gossh "golang.org/x/crypto/ssh"
)

// generateTestKey writes a PEM-encoded RSA private key to a temp file and
// returns its path.
func generateTestKey(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privPEM, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "id_rsa_*")
	if err != nil {
		t.Fatal(err)
	}
	if err := pem.Encode(f, privPEM); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestNewSSHDialer_MissingKeyFile(t *testing.T) {
	_, err := service.NewSSHDialer("pi", "raspberrypi.local:22", "/nonexistent/id_rsa")
	if err == nil {
		t.Error("NewSSHDialer() with missing key file should return error")
	}
}

func TestNewSSHDialer_InvalidKeyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bad_key_*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("this is not a valid key"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	_, err = service.NewSSHDialer("pi", "raspberrypi.local:22", f.Name())
	if err == nil {
		t.Error("NewSSHDialer() with invalid key file should return error")
	}
}

func TestNewSSHDialer_ValidKey(t *testing.T) {
	keyPath := generateTestKey(t)
	dialer, err := service.NewSSHDialer("pi", "raspberrypi.local:22", keyPath)
	if err != nil {
		t.Fatalf("NewSSHDialer() with valid key should not error, got: %v", err)
	}
	if dialer == nil {
		t.Error("NewSSHDialer() returned nil dialer")
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		input string
		want  string
	}{
		{"~/.ssh/id_rsa", home + "/.ssh/id_rsa"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", home},
	}
	for _, tc := range tests {
		got := service.ExpandTilde(tc.input)
		if got != tc.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
