package client

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

// tarBuilder helps create in-memory tar archives for tests.
type tarBuilder struct {
	buf bytes.Buffer
	tw  *tar.Writer
}

func newTarBuilder() *tarBuilder {
	b := &tarBuilder{}
	b.tw = tar.NewWriter(&b.buf)
	return b
}

func (b *tarBuilder) addDir(name string) {
	_ = b.tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeDir,
	})
}

func (b *tarBuilder) addFile(name string, size int64) {
	_ = b.tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Size:     size,
	})
	if size > 0 {
		_, _ = b.tw.Write(make([]byte, size))
	}
}

func (b *tarBuilder) addSymlink(name, target string) {
	_ = b.tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeSymlink,
		Linkname: target,
	})
}

func (b *tarBuilder) reader() io.ReadCloser {
	b.tw.Close()
	return io.NopCloser(bytes.NewReader(b.buf.Bytes()))
}

// errReader returns a non-EOF error after yielding the provided bytes.
type errReader struct {
	r   io.Reader
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, e.err
	}
	return n, err
}

func (e *errReader) Close() error { return nil }

// findNode walks the tree breadth-first and returns the first node with the given name.
func findNode(root *FileNode, name string) *FileNode {
	if root == nil {
		return nil
	}
	if root.Name == name {
		return root
	}
	for _, c := range root.Children {
		if n := findNode(c, name); n != nil {
			return n
		}
	}
	return nil
}

func TestBuildContainerFileTree_ExplicitDirs(t *testing.T) {
	b := newTarBuilder()
	b.addDir("etc/")
	b.addFile("etc/hosts", 100)

	root := buildContainerFileTree(b.reader())

	if root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(root.Children) != 1 {
		t.Errorf("expected 1 child of root, got %d", len(root.Children))
	}
	etc := root.Children[0]
	if etc.Name != "etc" {
		t.Errorf("expected child name 'etc', got %q", etc.Name)
	}
	if !etc.IsDir {
		t.Error("expected 'etc' to be a directory")
	}
	if len(etc.Children) != 1 {
		t.Errorf("expected 1 child of etc, got %d", len(etc.Children))
	}
	hosts := etc.Children[0]
	if hosts.Name != "hosts" {
		t.Errorf("expected child name 'hosts', got %q", hosts.Name)
	}
	if hosts.IsDir {
		t.Error("expected 'hosts' to be a file")
	}
}

func TestBuildContainerFileTree_ImplicitDir_DepthAndPath(t *testing.T) {
	// Only a file entry — directory nodes must be created implicitly.
	b := newTarBuilder()
	b.addFile("usr/bin/env", 50)

	root := buildContainerFileTree(b.reader())
	usr := findNode(root, "usr")
	if usr == nil {
		t.Error("usr node not found")
	}
	if usr.Name != "usr" {
		t.Errorf("expected 'usr', got %q", usr.Name)
	}
	if usr.Depth != 1 {
		t.Errorf("implicit dir 'usr' depth: got %d, want 1", usr.Depth)
	}

	if usr.Path != "usr" {
		t.Errorf("implicit dir 'usr' path: got %q, want %q", usr.Path, "usr")
	}

	if len(usr.Children) != 1 {
		t.Errorf("expected 1 child of usr, got %d", len(usr.Children))
	}
	bin := findNode(usr, "bin")
	if bin == nil {
		t.Error("bin node not found")
	}

	if bin.Name != "bin" {
		t.Errorf("expected 'bin', got %q", bin.Name)
	}
	if bin.Depth != 2 {
		t.Errorf("implicit dir 'bin' depth: got %d, want 2", bin.Depth)
	}
	if bin.Path != "usr/bin" {
		t.Errorf("implicit dir 'bin' path: got %q, want %q", bin.Path, "usr/bin")
	}
}

func TestBuildContainerFileTree_NonEOFError(t *testing.T) {
	b := newTarBuilder()
	b.addFile("a.txt", 0)
	data, _ := io.ReadAll(b.reader())

	sentinelErr := errors.New("disk read error")
	badReader := &errReader{r: bytes.NewReader(data), err: sentinelErr}

	root := buildContainerFileTree(badReader)
	if root == nil {
		t.Fatal("expected non-nil root even on read error")
	}
	// "a.txt" was read before the error, so it should appear.
	if len(root.Children) != 1 || root.Children[0].Name != "a.txt" {
		t.Errorf("expected root to contain 'a.txt', got %v",
			func() []string {
				names := make([]string, len(root.Children))
				for i, c := range root.Children {
					names[i] = c.Name
				}
				return names
			}(),
		)
	}
}

func TestBuildContainerFileTree_ComplexTree(t *testing.T) {
	// Layout:
	//   usr/
	//     bin/
	//       env        (file, 1234 bytes)
	//       sh -> bash (symlink)
	//     lib/
	//       libc.so.6  (file, 5678 bytes)
	//   etc/
	//     conf.d/
	//       app.conf   (file, 42 bytes)
	b := newTarBuilder()
	b.addDir("usr/")
	b.addDir("usr/bin/")
	b.addFile("usr/bin/env", 1234)
	b.addSymlink("usr/bin/sh", "bash")
	b.addDir("usr/lib/")
	b.addFile("usr/lib/libc.so.6", 5678)
	b.addDir("etc/")
	b.addDir("etc/conf.d/")
	b.addFile("etc/conf.d/app.conf", 42)

	root := buildContainerFileTree(b.reader())

	// Root should have two children: usr and etc.
	if len(root.Children) != 2 {
		t.Errorf("root children: got %d, want 2", len(root.Children))
	}

	// --- usr ---
	usr := findNode(root, "usr")
	if usr == nil {
		t.Fatal("node 'usr' not found")
	}
	if !usr.IsDir {
		t.Error("'usr' should be a directory")
	}
	if usr.Depth != 1 {
		t.Errorf("'usr' depth: got %d, want 1", usr.Depth)
	}

	bin := findNode(usr, "bin")
	if bin == nil {
		t.Fatal("node 'bin' not found")
	}
	if !bin.IsDir {
		t.Error("'bin' should be a directory")
	}
	if bin.Depth != 2 {
		t.Errorf("'bin' depth: got %d, want 2", bin.Depth)
	}

	env := findNode(bin, "env")
	if env == nil {
		t.Fatal("node 'env' not found")
	}
	if env.IsDir {
		t.Error("'env' should be a file")
	}
	if env.Size != 1234 {
		t.Errorf("'env' size: got %d, want 1234", env.Size)
	}
	if env.Depth != 3 {
		t.Errorf("'env' depth: got %d, want 3", env.Depth)
	}

	// --- symlink ---
	sh := findNode(bin, "sh")
	if sh == nil {
		t.Fatal("node 'sh' not found")
	}
	if sh.IsDir {
		t.Error("'sh' symlink should not be a directory")
	}
	if sh.Linkname != "bash" {
		t.Errorf("'sh' linkname: got %q, want %q", sh.Linkname, "bash")
	}
	if sh.Depth != 3 {
		t.Errorf("'sh' depth: got %d, want 3", sh.Depth)
	}

	// --- usr/lib ---
	lib := findNode(usr, "lib")
	if lib == nil {
		t.Fatal("node 'lib' not found")
	}
	if !lib.IsDir {
		t.Error("'lib' should be a directory")
	}
	libc := findNode(lib, "libc.so.6")
	if libc == nil {
		t.Fatal("node 'libc.so.6' not found")
	}
	if libc.Size != 5678 {
		t.Errorf("'libc.so.6' size: got %d, want 5678", libc.Size)
	}

	// --- etc ---
	etc := findNode(root, "etc")
	if etc == nil {
		t.Fatal("node 'etc' not found")
	}
	if etc.Depth != 1 {
		t.Errorf("'etc' depth: got %d, want 1", etc.Depth)
	}

	confd := findNode(etc, "conf.d")
	if confd == nil {
		t.Fatal("node 'conf.d' not found")
	}
	if confd.Depth != 2 {
		t.Errorf("'conf.d' depth: got %d, want 2", confd.Depth)
	}

	appConf := findNode(confd, "app.conf")
	if appConf == nil {
		t.Fatal("node 'app.conf' not found")
	}
	if appConf.Size != 42 {
		t.Errorf("'app.conf' size: got %d, want 42", appConf.Size)
	}
	if appConf.Depth != 3 {
		t.Errorf("'app.conf' depth: got %d, want 3", appConf.Depth)
	}
}

func TestBuildContainerHealth(t *testing.T) {
	now := time.Now()
	t.Run("Status Healthy", func(t *testing.T) {
		payload := &container.ContainerJSONBase{
			State: &container.State{
				Health: &container.Health{
					Status:        container.Healthy,
					FailingStreak: 2,
					Log: []*container.HealthcheckResult{
						{
							End:    now,
							Output: "HTTP/1.1 200 OK",
						},
					},
				},
			},
		}

		pHealth := payload.State.Health
		h := buildContainerHealth(payload)

		if h.FailingStreak != pHealth.FailingStreak {
			t.Errorf(
				"unexpected number of failing streak got= %d, expected= %d",
				h.FailingStreak,
				pHealth.FailingStreak,
			)
		}
		if h.Status != HealthStatus(pHealth.Status) {
			t.Errorf("unexpected health status got= %q, expected= %q", h.Status, pHealth.Status)
		}
		if h.Output != pHealth.Log[len(pHealth.Log)-1].Output {
			t.Errorf("unexpected output got= %q, expected= %q", h.Output, pHealth.Log[len(pHealth.Log)-1].Output)
		}
		if h.LastCheck != pHealth.Log[len(pHealth.Log)-1].End {
			t.Errorf(
				"different last check time got= %q, expected= %q",
				h.LastCheck.String(),
				pHealth.Log[len(pHealth.Log)-1].End.String(),
			)
		}
	})

	t.Run("Status Unhealthy", func(t *testing.T) {
		payload := &container.ContainerJSONBase{
			State: &container.State{
				Health: &container.Health{
					Status:        container.Unhealthy,
					FailingStreak: 3,
					Log: []*container.HealthcheckResult{
						{
							End:    now.Add(-1 * time.Hour),
							Output: "connection refused",
						},
					},
				},
			},
		}

		pHealth := payload.State.Health
		h := buildContainerHealth(payload)

		if h.FailingStreak != pHealth.FailingStreak {
			t.Errorf(
				"unexpected number of failing streak got= %d, expected= %d",
				h.FailingStreak,
				pHealth.FailingStreak,
			)
		}
		if h.Status != HealthStatus(pHealth.Status) {
			t.Errorf("unexpected health status got= %q, expected= %q", h.Status, pHealth.Status)
		}
		if h.Output != pHealth.Log[len(pHealth.Log)-1].Output {
			t.Errorf("unexpected output got= %q, expected= %q", h.Output, pHealth.Log[len(pHealth.Log)-1].Output)
		}
		if h.LastCheck != pHealth.Log[len(pHealth.Log)-1].End {
			t.Errorf(
				"different last check time got= %q, expected= %q",
				h.LastCheck.String(),
				pHealth.Log[len(pHealth.Log)-1].End.String(),
			)
		}
	})

	t.Run("Status Starting", func(t *testing.T) {
		payload := &container.ContainerJSONBase{
			State: &container.State{
				Health: &container.Health{
					Status:        container.Starting,
					FailingStreak: 1,
					Log: []*container.HealthcheckResult{
						{
							End:    now.Add(-30 * time.Minute),
							Output: "HTTP/1.1 200 OK",
						},
					},
				},
			},
		}

		pHealth := payload.State.Health
		h := buildContainerHealth(payload)

		if h.FailingStreak != pHealth.FailingStreak {
			t.Errorf(
				"unexpected number of failing streak got= %d, expected= %d",
				h.FailingStreak,
				pHealth.FailingStreak,
			)
		}
		if h.Status != HealthStatus(pHealth.Status) {
			t.Errorf("unexpected health status got= %q, expected= %q", h.Status, pHealth.Status)
		}
		if h.Output != pHealth.Log[len(pHealth.Log)-1].Output {
			t.Errorf("unexpected output got= %q, expected= %q", h.Output, pHealth.Log[len(pHealth.Log)-1].Output)
		}
		if h.LastCheck != pHealth.Log[len(pHealth.Log)-1].End {
			t.Errorf(
				"different last check time got= %q, expected= %q",
				h.LastCheck.String(),
				pHealth.Log[len(pHealth.Log)-1].End.String(),
			)
		}
	})

	t.Run("Status None", func(t *testing.T) {
		payload := &container.ContainerJSONBase{
			State: &container.State{
				Health: &container.Health{
					Status: container.NoHealthcheck,
				},
			},
		}

		pHealth := payload.State.Health
		h := buildContainerHealth(payload)

		if h.FailingStreak != pHealth.FailingStreak {
			t.Errorf(
				"unexpected number of failing streak got= %d, expected= %d",
				h.FailingStreak,
				pHealth.FailingStreak,
			)
		}
		if h.Status != HealthStatus(pHealth.Status) {
			t.Errorf("unexpected health status got= %q, expected= %q", h.Status, pHealth.Status)
		}
		if h.Output != "" {
			t.Errorf("unexpected output got= %q, expected= %q", h.Output, "")
		}
		if !h.LastCheck.IsZero() {
			t.Errorf("unexpected last check time got= %q, expected= %q", h.LastCheck.String(), time.Time{})
		}
	})
}

// buildMuxFrame builds a Docker multiplexed-stream frame for the given streamType and payload.
// Frame format: [streamType, 0, 0, 0, size(4 bytes big-endian), payload...].
func buildMuxFrame(streamType byte, payload []byte) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = streamType
	binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

func TestCopyStreamRaw(t *testing.T) {
	// First byte > 0x02 → raw stream, io.Copy is used.
	data := []byte{0x03, 'h', 'e', 'l', 'l', 'o'}
	var buf bytes.Buffer
	err := copyStream(&buf, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("copyStream() raw error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("copyStream() raw output = %q, want %q", buf.Bytes(), data)
	}
}

func TestCopyStreamMuxStdout(t *testing.T) {
	// First byte == 0x01 (stdout) → Docker multiplexed format.
	payload := []byte("hello logs")
	frame := buildMuxFrame(0x01, payload)
	var buf bytes.Buffer
	err := copyStream(&buf, bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("copyStream() mux error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Errorf("copyStream() mux output = %q, want %q", buf.Bytes(), payload)
	}
}

func TestCopyStreamMuxStderr(t *testing.T) {
	// First byte == 0x02 (stderr) → Docker multiplexed format, written to same writer.
	payload := []byte("error output")
	frame := buildMuxFrame(0x02, payload)
	var buf bytes.Buffer
	err := copyStream(&buf, bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("copyStream() mux stderr error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Errorf("copyStream() mux stderr output = %q, want %q", buf.Bytes(), payload)
	}
}

func TestCopyStreamEmptyReader(t *testing.T) {
	// Empty reader → io.ReadFull returns io.ErrUnexpectedEOF (or io.EOF for 0 bytes).
	var buf bytes.Buffer
	err := copyStream(&buf, bytes.NewReader(nil))
	if err == nil {
		t.Fatal("copyStream() on empty reader should return an error")
	}
}

func TestCopyStreamMuxMultipleFrames(t *testing.T) {
	// Multiple frames: stdout then stderr, both written to the same writer.
	var input bytes.Buffer
	input.Write(buildMuxFrame(0x01, []byte("line1\n")))
	input.Write(buildMuxFrame(0x02, []byte("line2\n")))
	var buf bytes.Buffer
	err := copyStream(&buf, &input)
	if err != nil {
		t.Fatalf("copyStream() multiple frames error: %v", err)
	}
	got := buf.String()
	if got != "line1\nline2\n" {
		t.Errorf("copyStream() multiple frames = %q, want %q", got, "line1\nline2\n")
	}
}

func TestNewDockerClientFromConfig_EmptyHost(t *testing.T) {
	cfg := config.DockerConfig{}
	client, err := NewDockerClientFromConfig(cfg)
	if err == nil {
		if client == nil {
			t.Error("NewDockerClientFromConfig() returned nil client with nil error")
		} else {
			client.Close()
		}
	}
	// err != nil is acceptable — Docker daemon may not be available in CI.
}

func TestNewDockerClientFromConfig_TCPHost(t *testing.T) {
	cfg := config.DockerConfig{
		Host: "tcp://127.0.0.1:2375",
	}
	client, err := NewDockerClientFromConfig(cfg)
	// Client creation should succeed even if daemon is unreachable.
	if err != nil {
		t.Errorf("NewDockerClientFromConfig() TCP host creation error: %v", err)
	}
	defer client.Close()
}
