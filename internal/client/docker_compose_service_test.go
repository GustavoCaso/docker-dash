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

func TestParseSSHTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "host only", input: "ssh://myhost", want: "myhost"},
		{name: "user and host", input: "ssh://alice@myhost", want: "alice@myhost"},
		{name: "user host port", input: "ssh://alice@myhost:2222", want: "alice@myhost"},
		{name: "non-ssh url", input: "tcp://myhost:2375", want: ""},
		{name: "unix socket", input: "unix:///var/run/docker.sock", want: ""},
		{name: "empty", input: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseSSHTarget(tc.input)
			if got != tc.want {
				t.Fatalf("parseSSHTarget(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSSHResourceLoaderAccept(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existing := filepath.Join(tmpDir, "existing.yml")
	if err := os.WriteFile(existing, []byte("x: 1"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loader := newSSHResourceLoader("host", "/remote", tmpDir)

	if loader.Accept(existing) {
		t.Fatal("Accept(existing file) = true, want false")
	}

	missing := filepath.Join(tmpDir, "missing.yml")
	if !loader.Accept(missing) {
		t.Fatal("Accept(missing file) = false, want true")
	}
}

func TestSSHResourceLoaderDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	l := newSSHResourceLoader("host", "/remote", tmpDir)

	if got := l.Dir("/anything"); got != tmpDir {
		t.Fatalf("Dir() = %q, want %q", got, tmpDir)
	}
}

// fakeSshBin writes a tiny shell script that acts as `ssh` for testing.
// It echoes predefined content based on the requested remote path.
// Returns the bin dir that should be prepended to PATH.
func fakeSshBin(t *testing.T, responses map[string]string) string {
	t.Helper()

	binDir := t.TempDir()

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n# args: <target> cat <remotePath>\npath=$3\n")
	for remotePath, content := range responses {
		sb.WriteString("if [ \"$path\" = \"")
		sb.WriteString(remotePath)
		sb.WriteString("\" ]; then printf '%s' '")
		sb.WriteString(content)
		sb.WriteString("'; exit 0; fi\n")
	}
	sb.WriteString("echo 'file not found' >&2; exit 1\n")

	sshBin := filepath.Join(binDir, "ssh")
	if err := os.WriteFile(sshBin, []byte(sb.String()), 0o755); err != nil {
		t.Fatalf("WriteFile ssh stub: %v", err)
	}

	return binDir
}

func TestSSHResourceLoaderLoad_FetchesRemoteFile(t *testing.T) {
	tmpDir := t.TempDir()
	remotePath := "/remote/project/compose.yaml"
	fileContent := "services:\n  web:\n    image: nginx\n"

	binDir := fakeSshBin(t, map[string]string{remotePath: fileContent})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	l := newSSHResourceLoader("testhost", "/remote/project", tmpDir)

	localPath, err := l.Load(context.Background(), remotePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", localPath, err)
	}

	if string(got) != fileContent {
		t.Fatalf("Load() content = %q, want %q", string(got), fileContent)
	}
}

func TestSSHResourceLoaderLoad_ResolvesRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	remoteWorkingDir := "/remote/project"
	remotePath := "/remote/project/compose.yaml"
	fileContent := "services:\n  db:\n    image: postgres\n"

	binDir := fakeSshBin(t, map[string]string{remotePath: fileContent})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	l := newSSHResourceLoader("testhost", remoteWorkingDir, tmpDir)

	// Pass a relative path — loader should join it with remoteWorkingDir.
	localPath, err := l.Load(context.Background(), "compose.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", localPath, err)
	}

	if string(got) != fileContent {
		t.Fatalf("Load() content = %q, want %q", string(got), fileContent)
	}
}

func TestSSHResourceLoaderLoad_CachesResult(t *testing.T) {
	tmpDir := t.TempDir()
	remotePath := "/remote/project/compose.yaml"
	fileContent := "cached: true\n"

	// Pre-populate the cache file.
	localPath := filepath.Join(tmpDir, "compose.yaml")
	if err := os.WriteFile(localPath, []byte(fileContent), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Point PATH to an empty dir so any real ssh call would fail.
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	l := newSSHResourceLoader("testhost", "/remote/project", tmpDir)

	got, err := l.Load(context.Background(), remotePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got != localPath {
		t.Fatalf("Load() returned %q, want cached %q", got, localPath)
	}
}

func TestSSHResourceLoaderLoad_ErrorOnSSHFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Point PATH to an empty dir so ssh is not found / fails.
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	l := newSSHResourceLoader("testhost", "/remote/project", tmpDir)

	_, err := l.Load(context.Background(), "/remote/project/missing.yaml")
	if err == nil {
		t.Fatal("Load() expected error when ssh fails, got nil")
	}
}

func TestRemapProjectPaths(t *testing.T) {
	t.Parallel()

	localTmpDir := "/tmp/docker-dash-compose-abc"
	remoteWorkingDir := "/home/user/myapp"

	project := &composetypes.Project{
		WorkingDir:   localTmpDir,
		ComposeFiles: []string{localTmpDir + "/compose.yaml", localTmpDir + "/override.yaml"},
		Services: composetypes.Services{
			"web": {
				Volumes: []composetypes.ServiceVolumeConfig{
					{Type: "bind", Source: localTmpDir + "/data"},
					{Type: "volume", Source: "myvolume"},
				},
			},
		},
	}

	remapProjectPaths(project, remoteWorkingDir)

	if project.WorkingDir != remoteWorkingDir {
		t.Errorf("WorkingDir = %q, want %q", project.WorkingDir, remoteWorkingDir)
	}

	wantFiles := []string{remoteWorkingDir + "/compose.yaml", remoteWorkingDir + "/override.yaml"}
	for i, f := range project.ComposeFiles {
		if f != wantFiles[i] {
			t.Errorf("ComposeFiles[%d] = %q, want %q", i, f, wantFiles[i])
		}
	}

	webSvc := project.Services["web"]
	if webSvc.Volumes[0].Source != remoteWorkingDir+"/data" {
		t.Errorf("bind volume source = %q, want %q", webSvc.Volumes[0].Source, remoteWorkingDir+"/data")
	}

	if webSvc.Volumes[1].Source != "myvolume" {
		t.Errorf("named volume source changed unexpectedly: %q", webSvc.Volumes[1].Source)
	}
}

func TestLoadComposeProject_ViaSSH(t *testing.T) {
	composeContent := "services:\n  web:\n    image: nginx:latest\n"
	remoteWorkingDir := "/home/user/myapp"
	remoteComposePath := remoteWorkingDir + "/compose.yaml"

	binDir := fakeSshBin(t, map[string]string{
		remoteComposePath:          composeContent,
		remoteWorkingDir + "/.env": "",
	})
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cli := newTestEngineClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]containerapi.Summary{})
	})

	service := &composeProjectService{
		engineClient: cli,
		composeSvc:   &fakeComposeSDK{},
		sshTarget:    "testhost",
	}
	service.loadProject = service.loadComposeProject

	project := ComposeProject{
		Name:        "myapp",
		WorkingDir:  remoteWorkingDir,
		ConfigFiles: remoteComposePath,
	}

	loaded, err := service.loadComposeProject(context.Background(), project)
	if err != nil {
		t.Fatalf("loadComposeProject() error = %v", err)
	}

	if loaded.Name != project.Name {
		t.Fatalf("loaded project name = %q, want %q", loaded.Name, project.Name)
	}

	// After remapProjectPaths the project WorkingDir must point back to the remote dir.
	if loaded.WorkingDir != remoteWorkingDir {
		t.Fatalf("loaded WorkingDir = %q, want %q", loaded.WorkingDir, remoteWorkingDir)
	}
}
