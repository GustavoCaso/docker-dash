package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

func TestComposeProjectServiceList_FiltersComposeContainers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		gotFilters, err := url.QueryUnescape(r.URL.Query().Get("filters"))
		if err != nil {
			t.Fatalf("failed to decode filters: %v", err)
		}

		if !strings.Contains(gotFilters, composeProjectLabel) {
			t.Fatalf("expected compose label filter, got %q", gotFilters)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]container.Summary{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(server.URL),
		dockerclient.WithHTTPClient(server.Client()),
		dockerclient.WithVersion("1.48"),
	)
	if err != nil {
		t.Fatalf("NewClientWithOpts() error = %v", err)
	}
	defer cli.Close()

	service := &composeProjectService{cli: cli}

	projects, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(projects) != 0 {
		t.Fatalf("List() returned %d projects, want 0", len(projects))
	}
}

func TestComposeProjectServiceList_GroupsByComposeIdentity(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		containers := []container.Summary{
			{
				ID:    "ctr-1",
				Names: []string{"/api-1"},
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					composeProjectLabel:     "web",
					composeWorkingDirLabel:  "/tmp/project-a",
					composeConfigFilesLabel: "/tmp/project-a/compose.yml",
					composeServiceLabel:     "api",
				},
			},
			{
				ID:    "ctr-2",
				Names: []string{"/api-1"},
				Image: "nginx:stable",
				State: "exited",
				Labels: map[string]string{
					composeProjectLabel:     "web",
					composeWorkingDirLabel:  "/tmp/project-b",
					composeConfigFilesLabel: "/tmp/project-b/compose.yml",
					composeServiceLabel:     "api",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(containers); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(server.URL),
		dockerclient.WithHTTPClient(server.Client()),
		dockerclient.WithVersion("1.48"),
	)
	if err != nil {
		t.Fatalf("NewClientWithOpts() error = %v", err)
	}
	defer cli.Close()

	service := &composeProjectService{cli: cli}

	projects, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("List() returned %d projects, want 2", len(projects))
	}

	if projects[0].WorkingDir == projects[1].WorkingDir {
		t.Fatalf("expected projects to remain distinct, got %+v", projects)
	}
}
