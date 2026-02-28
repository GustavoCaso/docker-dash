package client

import (
	"context"
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

func TestMockClient_VolumeFileTree(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	volumes, err := client.Volumes().List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Every volume should be browsable, regardless of UsedCount
	for _, vol := range volumes {
		t.Run(vol.Name, func(t *testing.T) {
			ft, err := client.Volumes().FileTree(context.Background(), vol.Name)
			if err != nil {
				t.Fatalf("FileTree(%s) error = %v", vol.Name, err)
			}

			if ft.Tree == nil {
				t.Error("FileTree() returned nil tree")
			}

			if len(ft.Files) == 0 {
				t.Error("FileTree() returned empty files")
			}

			// Tree should render without error
			treeStr := ft.Tree.String()
			if treeStr == "" {
				t.Error("Tree.String() returned empty")
			}
		})
	}
}

func TestMockClient_VolumeFileTree_PostgresData(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	ft, err := client.Volumes().FileTree(context.Background(), "postgres_data")
	if err != nil {
		t.Fatalf("FileTree() error = %v", err)
	}

	fileSet := make(map[string]bool)
	for _, f := range ft.Files {
		fileSet[f] = true
	}

	expected := []string{"pgdata/PG_VERSION", "pgdata/postgresql.conf", "pgdata/base/1"}
	for _, f := range expected {
		if !fileSet[f] {
			t.Errorf("expected file %q in tree, got files: %v", f, ft.Files)
		}
	}
}

func TestMockClient_VolumeFileTree_NginxConfig(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	ft, err := client.Volumes().FileTree(context.Background(), "nginx_config")
	if err != nil {
		t.Fatalf("FileTree() error = %v", err)
	}

	fileSet := make(map[string]bool)
	for _, f := range ft.Files {
		fileSet[f] = true
	}

	expected := []string{"nginx.conf", "conf.d/default.conf"}
	for _, f := range expected {
		if !fileSet[f] {
			t.Errorf("expected file %q in tree, got files: %v", f, ft.Files)
		}
	}
}

func TestMockClient_VolumeFileTree_UnusedVolume(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	// app_data has UsedCount=0 — should still be browsable
	ft, err := client.Volumes().FileTree(context.Background(), "app_data")
	if err != nil {
		t.Fatalf("FileTree() unexpected error for unused volume: %v", err)
	}

	if ft.Tree == nil {
		t.Error("FileTree() returned nil tree")
	}

	if len(ft.Files) == 0 {
		t.Error("FileTree() returned empty files for unused volume")
	}
}

func TestMockClient_VolumeFileTree_NotFound(t *testing.T) {
	client := NewMockClient()
	defer client.Close()

	_, err := client.Volumes().FileTree(context.Background(), "nonexistent_volume")
	if err == nil {
		t.Error("FileTree() expected error for nonexistent volume, got nil")
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
