package service_test

import (
	"context"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/service"
)

func TestMockClient_Ping(t *testing.T) {
	client := service.NewMockClient()
	defer client.Close()

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestMockClient_ContainerList(t *testing.T) {
	client := service.NewMockClient()
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
	client := service.NewMockClient()
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
	client := service.NewMockClient()
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
	client := service.NewMockClient()
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
