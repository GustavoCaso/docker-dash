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
