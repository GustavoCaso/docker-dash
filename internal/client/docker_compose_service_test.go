package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	composetypes "github.com/compose-spec/compose-go/v2/types"
	composeapi "github.com/docker/compose/v2/pkg/api"
	containerapi "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type fakeComposeSDK struct {
	upProject    *composetypes.Project
	startProject string
	stopProject  string
	restartName  string
	downName     string
}

func (f *fakeComposeSDK) Up(_ context.Context, project *composetypes.Project, _ composeapi.UpOptions) error {
	f.upProject = project
	return nil
}

func (f *fakeComposeSDK) Down(_ context.Context, projectName string, _ composeapi.DownOptions) error {
	f.downName = projectName
	return nil
}

func (f *fakeComposeSDK) Start(_ context.Context, projectName string, _ composeapi.StartOptions) error {
	f.startProject = projectName
	return nil
}

func (f *fakeComposeSDK) Stop(_ context.Context, projectName string, _ composeapi.StopOptions) error {
	f.stopProject = projectName
	return nil
}

func (f *fakeComposeSDK) Restart(_ context.Context, projectName string, _ composeapi.RestartOptions) error {
	f.restartName = projectName
	return nil
}

func newTestEngineClient(t *testing.T, handler http.HandlerFunc) *dockerclient.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost(server.URL),
		dockerclient.WithHTTPClient(server.Client()),
		dockerclient.WithVersion("1.48"),
	)
	if err != nil {
		t.Fatalf("NewClientWithOpts() error = %v", err)
	}

	t.Cleanup(func() {
		_ = cli.Close()
	})

	return cli
}

func TestComposeProjectServiceList_FiltersComposeContainers(t *testing.T) {
	t.Parallel()

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		gotFilters, err := url.QueryUnescape(r.URL.Query().Get("filters"))
		if err != nil {
			t.Fatalf("failed to decode filters: %v", err)
		}

		if !strings.Contains(gotFilters, composeapi.ProjectLabel) {
			t.Fatalf("expected compose label filter, got %q", gotFilters)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]containerapi.Summary{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})

	service := &composeProjectService{engineClient: cli}

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

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		containers := []containerapi.Summary{
			{
				ID:    "ctr-1",
				Names: []string{"/api-1"},
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					composeapi.ProjectLabel:         "web",
					composeapi.WorkingDirLabel:      "/tmp/project-a",
					composeapi.ConfigFilesLabel:     "/tmp/project-a/compose.yml",
					composeapi.EnvironmentFileLabel: "/tmp/project-a/.env",
					composeapi.ServiceLabel:         "api",
				},
			},
			{
				ID:    "ctr-2",
				Names: []string{"/api-1"},
				Image: "nginx:stable",
				State: "exited",
				Labels: map[string]string{
					composeapi.ProjectLabel:         "web",
					composeapi.WorkingDirLabel:      "/tmp/project-b",
					composeapi.ConfigFilesLabel:     "/tmp/project-b/compose.yml",
					composeapi.EnvironmentFileLabel: "/tmp/project-b/.env",
					composeapi.ServiceLabel:         "api",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(containers); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})

	service := &composeProjectService{engineClient: cli}

	projects, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("List() returned %d projects, want 2", len(projects))
	}

	if projects[0].EnvironmentFiles == "" || projects[1].EnvironmentFiles == "" {
		t.Fatalf("expected environment files to be captured, got %+v", projects)
	}
}

func TestComposeProjectServiceUp_LoadsProjectFromConfigFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "compose.yaml")
	if err := os.WriteFile(configFile, []byte("services:\n  web:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]containerapi.Summary{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})
	fakeCompose := &fakeComposeSDK{}
	service := &composeProjectService{
		engineClient: cli,
		composeSvc:   fakeCompose,
		loadProject:  nil,
	}
	service.loadProject = service.loadComposeProject

	project := ComposeProject{
		Name:        "test-app",
		WorkingDir:  tmpDir,
		ConfigFiles: configFile,
	}

	if err := service.Up(context.Background(), project, ComposeUpOptions{}); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	if fakeCompose.upProject == nil {
		t.Fatal("expected compose up to receive a loaded project")
	}

	if fakeCompose.upProject.Name != project.Name {
		t.Fatalf("loaded project name = %q, want %q", fakeCompose.upProject.Name, project.Name)
	}
}

func TestComposeProjectServiceUp_ErrorsWhenProjectCannotLoad(t *testing.T) {
	t.Parallel()

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]containerapi.Summary{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})

	service := &composeProjectService{
		engineClient: cli,
		composeSvc:   &fakeComposeSDK{},
	}
	service.loadProject = service.loadComposeProject

	err := service.Up(context.Background(), ComposeProject{
		Name:        "missing",
		WorkingDir:  t.TempDir(),
		ConfigFiles: filepath.Join(t.TempDir(), "missing-compose.yaml"),
	}, ComposeUpOptions{})
	if err == nil {
		t.Fatal("expected Up() to fail when compose file is missing")
	}
}

func TestComposeProjectServiceStart_ErrorsOnAmbiguousProjectName(t *testing.T) {
	t.Parallel()

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		gotFilters, err := url.QueryUnescape(r.URL.Query().Get("filters"))
		if err != nil {
			t.Fatalf("failed to decode filters: %v", err)
		}
		if !strings.Contains(gotFilters, composeapi.ProjectLabel+"=web") {
			t.Fatalf("expected project-name label filter, got %q", gotFilters)
		}

		containers := []containerapi.Summary{
			{
				ID:    "ctr-1",
				Names: []string{"/api-1"},
				State: "running",
				Labels: map[string]string{
					composeapi.ProjectLabel:     "web",
					composeapi.WorkingDirLabel:  "/tmp/project-a",
					composeapi.ConfigFilesLabel: "/tmp/project-a/compose.yml",
				},
			},
			{
				ID:    "ctr-2",
				Names: []string{"/api-1"},
				State: "running",
				Labels: map[string]string{
					composeapi.ProjectLabel:     "web",
					composeapi.WorkingDirLabel:  "/tmp/project-b",
					composeapi.ConfigFilesLabel: "/tmp/project-b/compose.yml",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(containers); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})

	service := &composeProjectService{
		engineClient: cli,
		composeSvc:   &fakeComposeSDK{},
	}

	err := service.Start(context.Background(), ComposeProject{
		Name:        "web",
		WorkingDir:  "/tmp/project-a",
		ConfigFiles: "/tmp/project-a/compose.yml",
	}, ComposeStartOptions{})
	if !errors.Is(err, ErrComposeProjectNameAmbiguous) {
		t.Fatalf("expected ambiguous project error, got %v", err)
	}
}

func TestComposeProjectServiceRestart_UsesComposeAPI(t *testing.T) {
	t.Parallel()

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]containerapi.Summary{}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	})
	fakeCompose := &fakeComposeSDK{}
	service := &composeProjectService{
		engineClient: cli,
		composeSvc:   fakeCompose,
	}

	project := ComposeProject{Name: "web", WorkingDir: "/tmp/web", ConfigFiles: "/tmp/web/compose.yml"}
	if err := service.Restart(context.Background(), project, ComposeRestartOptions{}); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	if fakeCompose.restartName != project.Name {
		t.Fatalf("restart target = %q, want %q", fakeCompose.restartName, project.Name)
	}
}
