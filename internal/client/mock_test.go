package client

import (
	"archive/tar"
	"context"
	"io"
	"testing"
)

func TestMockClient_Ping(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestMockClient_ContainerList(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	containers, err := client.Containers().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(containers) == 0 {
		t.Error("List() returned empty, want sample containers")
	}
}

func TestMockClient_ImageList(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	images, err := client.Images().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(images) == 0 {
		t.Error("List() returned empty, want sample images")
	}
}

func TestMockClient_VolumeList(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	volumes, err := client.Volumes().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(volumes) == 0 {
		t.Error("List() returned empty, want sample volumes")
	}
}

func TestMockClient_ContainerExec(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	containers, err := client.Containers().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Use the first running container
	session, err := client.Containers().Exec(context.Background(), containers[0].ID)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	defer session.Close()

	// Send a command in a goroutine (io.Pipe is synchronous, so write blocks until read)
	errCh := make(chan error, 1)
	go func() {
		_, err := session.Writer.Write([]byte("echo hello\n"))
		errCh <- err
	}()

	// Read output
	buf := make([]byte, 1024)
	n, err := session.Reader.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// Check write succeeded
	if writeErr := <-errCh; writeErr != nil {
		t.Fatalf("Write() error = %v", writeErr)
	}

	output := string(buf[:n])
	if output == "" {
		t.Error("Read() returned empty output, want mock shell output")
	}
}

func TestMockClient_CopyFromContainer(t *testing.T) {
	client := NewMockClient()

	tests := map[string]struct {
		srcPath string
		entries []mockTarEntry
		wantErr bool
	}{
		"valid source path, copying etc directory": {
			srcPath: "/etc",
			entries: []mockTarEntry{
				{name: "etc", isDir: true},
				{name: "etc/nginx.conf", content: "worker_processes 2;"},
			},
		},
		"valid source path, copying usr directory": {
			srcPath: "/usr",
			entries: []mockTarEntry{
				{name: "usr", isDir: true},
				{name: "usr/bin", isDir: true},
				{name: "usr/bin/cat", content: "cat binary"},
			},
		},
		"valid source path, copying nginx file": {
			srcPath: "/etc/nginx.conf",
			entries: []mockTarEntry{
				{name: "nginx.conf", content: "worker_processes 2;"},
			},
		},
		"valid source path, copying bin directory": {
			srcPath: "/usr/bin",
			entries: []mockTarEntry{
				{name: "bin", isDir: true},
				{name: "bin/cat", content: "cat binary"},
			},
		},
		"invalid source path": {
			srcPath: "/home/docker/.bashrc",
			wantErr: true,
		},
	}

	containers, err := client.Containers().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(containers) == 0 {
		t.Fatal("expected at least one container")
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rc, err := client.Containers().CopyFromContainer(
				context.Background(),
				containers[0].ID,
				test.srcPath,
			)

			if test.wantErr {
				if err == nil {
					if rc != nil {
						rc.Close()
					}
					t.Fatal("expected error copying from container")
				}

				if rc != nil {
					t.Fatal("expected nil reader when error is returned")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error copying from container: %v", err)
			}
			defer rc.Close()

			tr := tar.NewReader(rc)

			for _, wantEntry := range test.entries {
				hdr, err := tr.Next()
				if err != nil {
					t.Fatalf("error reading tar entry: %v", err)
				}

				if hdr.Name != wantEntry.name {
					t.Errorf("expected header name %q, got %q", wantEntry.name, hdr.Name)
				}

				if wantEntry.isDir {
					if hdr.Typeflag != tar.TypeDir {
						t.Errorf("expected %q to be a directory", wantEntry.name)
					}

					if hdr.Size != 0 {
						t.Errorf("expected directory %q size to be 0, got %d", wantEntry.name, hdr.Size)
					}
					continue
				}

				if hdr.Typeflag != tar.TypeReg {
					t.Errorf("expected %q to be a file", wantEntry.name)
				}

				data, err := io.ReadAll(tr)
				if err != nil {
					t.Fatalf("error reading tar file %q: %v", hdr.Name, err)
				}

				if string(data) != wantEntry.content {
					t.Errorf("expected content %q, got %q", wantEntry.content, string(data))
				}
			}
		})
	}
}
