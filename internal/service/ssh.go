package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

// SSHDialer dials a network address through an SSH connection.
// The returned function satisfies the signature expected by client.WithDialContext.
type SSHDialer func(ctx context.Context, network, addr string) (net.Conn, error)

// sshConn wraps a net.Conn forwarded through an SSH channel and ensures the
// parent SSH client is closed when the connection is closed.
type sshConn struct {
	net.Conn
	client *gossh.Client
}

func (c *sshConn) Close() error {
	err := c.Conn.Close()
	c.client.Close()
	return err
}

// NewSSHDialer creates an SSHDialer that authenticates to sshAddr (host:port)
// as user using the private key at keyPath. The returned dialer opens a
// channel to addr on the remote host for each call.
func NewSSHDialer(user, sshAddr, keyPath string) (SSHDialer, error) {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading identity file %s: %w", keyPath, err)
	}

	signer, err := gossh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing identity file %s: %w", keyPath, err)
	}

	cfg := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Use DialContext so context cancellation and deadlines are honoured
		// for the TCP leg of the SSH connection.
		nd := &net.Dialer{}
		tcpConn, err := nd.DialContext(ctx, "tcp", sshAddr)
		if err != nil {
			return nil, fmt.Errorf("ssh tcp dial %s: %w", sshAddr, err)
		}

		sshClientConn, chans, reqs, err := gossh.NewClientConn(tcpConn, sshAddr, cfg)
		if err != nil {
			tcpConn.Close()
			return nil, fmt.Errorf("ssh handshake %s: %w", sshAddr, err)
		}

		sshClient := gossh.NewClient(sshClientConn, chans, reqs)
		conn, err := sshClient.Dial(network, addr)
		if err != nil {
			sshClient.Close()
			return nil, fmt.Errorf("ssh forward to %s: %w", addr, err)
		}

		// Wrap conn so that closing it also closes the SSH client.
		return &sshConn{Conn: conn, client: sshClient}, nil
	}, nil
}

// ExpandTilde replaces a leading ~ with the user's home directory.
func ExpandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
