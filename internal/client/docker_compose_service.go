package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	composegocli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	composeapi "github.com/docker/compose/v2/pkg/api"
	composepkg "github.com/docker/compose/v2/pkg/compose"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

var ErrComposeProjectNameAmbiguous = errors.New("compose project name is ambiguous")

type composeSDK interface {
	Up(ctx context.Context, project *composetypes.Project, options composeapi.UpOptions) error
	Down(ctx context.Context, projectName string, options composeapi.DownOptions) error
	Start(ctx context.Context, projectName string, options composeapi.StartOptions) error
	Stop(ctx context.Context, projectName string, options composeapi.StopOptions) error
	Restart(ctx context.Context, projectName string, options composeapi.RestartOptions) error
}

type composeProjectLoader func(context.Context, ComposeProject) (*composetypes.Project, error)

// composeProjectService manages Compose projects using label discovery plus the
// Docker Compose SDK for project actions.
type composeProjectService struct {
	engineClient *dockerclient.Client
	composeSvc   composeSDK
	dockerCLI    *command.DockerCli
	loadProject  composeProjectLoader
	// sshTarget is "user@host" or "host" when the Docker host is ssh://.
	// Empty string means local Docker socket — no SSH file fetching needed.
	sshTarget string
}

type composeProjectKey struct {
	name             string
	workingDir       string
	configFiles      string
	environmentFiles string
}

func newComposeProjectService(
	cfg config.DockerConfig,
	engineClient *dockerclient.Client,
) (*composeProjectService, error) {
	dockerCLI, err := command.NewDockerCli(
		command.WithOutputStream(io.Discard),
		command.WithErrorStream(io.Discard),
	)
	if err != nil {
		return nil, err
	}

	opts := flags.NewClientOptions()
	if cfg.Host != "" {
		opts.Hosts = []string{cfg.Host}
	}

	if cliErr := dockerCLI.Initialize(opts); cliErr != nil {
		return nil, cliErr
	}

	svc := &composeProjectService{
		engineClient: engineClient,
		composeSvc: composepkg.NewComposeService(
			dockerCLI,
			composepkg.WithPrompt(func(string, bool) (bool, error) { return true, nil }),
		),
		dockerCLI: dockerCLI,
		sshTarget: parseSSHTarget(cfg.Host),
	}

	return svc, nil
}

func (s *composeProjectService) Close() error {
	if s.dockerCLI == nil {
		return nil
	}

	return s.dockerCLI.Client().Close()
}

// List loads only Compose-managed containers and aggregates them into
// ComposeProject entries.
func (s *composeProjectService) List(ctx context.Context) ([]ComposeProject, error) {
	log.Printf("[compose] List")
	containers, err := s.listComposeContainers(ctx, "")
	if err != nil {
		return nil, err
	}

	projectMap := make(map[composeProjectKey]*ComposeProject)
	projectOrder := []composeProjectKey{}

	for _, c := range containers {
		key := composeProjectKey{
			name:             c.Labels[composeapi.ProjectLabel],
			workingDir:       c.Labels[composeapi.WorkingDirLabel],
			configFiles:      c.Labels[composeapi.ConfigFilesLabel],
			environmentFiles: c.Labels[composeapi.EnvironmentFileLabel],
		}

		proj, exists := projectMap[key]
		if !exists {
			proj = &ComposeProject{
				Name:             key.name,
				WorkingDir:       key.workingDir,
				ConfigFiles:      key.configFiles,
				EnvironmentFiles: key.environmentFiles,
			}
			projectMap[key] = proj
			projectOrder = append(projectOrder, key)
		}

		serviceName := c.Labels[composeapi.ServiceLabel]
		if serviceName == "" {
			serviceName = firstContainerName(c.Names)
		}

		proj.Services = append(proj.Services, ComposeServiceInfo{
			Name:  serviceName,
			State: c.State,
			Image: c.Image,
		})
	}

	projects := make([]ComposeProject, 0, len(projectOrder))
	for _, key := range projectOrder {
		projects = append(projects, *projectMap[key])
	}

	log.Printf("[compose] List: found %d projects", len(projects))
	return projects, nil
}

func (s *composeProjectService) Up(ctx context.Context, project ComposeProject, opts ComposeUpOptions) error {
	if err := s.ensureProjectNameUnique(ctx, project); err != nil {
		return err
	}

	loadedProject, err := s.loadComposeProject(ctx, project)
	if err != nil {
		return err
	}

	upOptions := composeapi.UpOptions{
		Create: composeapi.CreateOptions{
			Services:      nil,
			RemoveOrphans: opts.RemoveOrphans,
		},
		Start: composeapi.StartOptions{
			Project: loadedProject,
			Wait:    opts.Wait,
		},
	}
	if opts.Build {
		upOptions.Create.Build = &composeapi.BuildOptions{}
	}

	return s.composeSvc.Up(ctx, loadedProject, upOptions)
}

func (s *composeProjectService) Down(ctx context.Context, project ComposeProject, opts ComposeDownOptions) error {
	if err := s.ensureProjectNameUnique(ctx, project); err != nil {
		return err
	}

	return s.composeSvc.Down(ctx, project.Name, composeapi.DownOptions{
		RemoveOrphans: opts.RemoveOrphans,
		Volumes:       opts.Volumes,
		Images:        opts.Images,
	})
}

func (s *composeProjectService) Start(ctx context.Context, project ComposeProject, _ ComposeStartOptions) error {
	if err := s.ensureProjectNameUnique(ctx, project); err != nil {
		return err
	}

	return s.composeSvc.Start(ctx, project.Name, composeapi.StartOptions{})
}

func (s *composeProjectService) Stop(ctx context.Context, project ComposeProject, opts ComposeStopOptions) error {
	if err := s.ensureProjectNameUnique(ctx, project); err != nil {
		return err
	}

	return s.composeSvc.Stop(ctx, project.Name, composeapi.StopOptions{Timeout: opts.Timeout})
}

func (s *composeProjectService) Restart(
	ctx context.Context,
	project ComposeProject,
	opts ComposeRestartOptions,
) error {
	if err := s.ensureProjectNameUnique(ctx, project); err != nil {
		return err
	}

	return s.composeSvc.Restart(ctx, project.Name, composeapi.RestartOptions{
		Timeout: opts.Timeout,
		NoDeps:  opts.NoDeps,
	})
}

func (s *composeProjectService) loadComposeProject(
	ctx context.Context,
	project ComposeProject,
) (*composetypes.Project, error) {
	configPaths := splitLabelValues(project.ConfigFiles)
	if len(configPaths) == 0 {
		return nil, fmt.Errorf("compose project %q has no config files label", project.Name)
	}

	projectOpts := []composegocli.ProjectOptionsFn{
		composegocli.WithWorkingDirectory(project.WorkingDir),
		composegocli.WithOsEnv,
		composegocli.WithEnvFiles(splitLabelValues(project.EnvironmentFiles)...),
		composegocli.WithDotEnv,
		composegocli.WithName(project.Name),
		composegocli.WithResolvedPaths(true),
	}

	if s.sshTarget != "" {
		sshOpts, cleanup, err := s.buildSSHProjectOpts(ctx, project)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		projectOpts = sshOpts
	}

	projectOptions, err := composegocli.NewProjectOptions(configPaths, projectOpts...)
	if err != nil {
		return nil, err
	}

	loadedProject, err := projectOptions.LoadProject(ctx)
	if err != nil {
		return nil, err
	}

	// For SSH hosts the SDK resolves paths against the local temp dir. Remap
	// everything back to the remote working dir so the remote Docker daemon
	// resolves bind-mount sources correctly on its end.
	if s.sshTarget != "" {
		remapProjectPaths(loadedProject, project.WorkingDir)
	}

	applyComposeLabels(
		loadedProject,
		project.WorkingDir,
		project.ConfigFiles,
		splitLabelValues(project.EnvironmentFiles),
	)

	return loadedProject, nil
}

func (s *composeProjectService) buildSSHProjectOpts(
	ctx context.Context,
	project ComposeProject,
) ([]composegocli.ProjectOptionsFn, func(), error) {
	tmpDir, err := os.MkdirTemp("", "docker-dash-compose-*")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp dir for remote compose files: %w", err)
	}

	log.Printf("[compose] loadComposeProject: using SSH resource loader for target %q, tmpDir=%q", s.sshTarget, tmpDir)

	sshLoader := newSSHResourceLoader(s.sshTarget, project.WorkingDir, tmpDir)

	opts := []composegocli.ProjectOptionsFn{
		composegocli.WithWorkingDirectory(project.WorkingDir),
		composegocli.WithOsEnv,
		composegocli.WithName(project.Name),
		composegocli.WithResolvedPaths(true),
		composegocli.WithResourceLoader(sshLoader),
	}

	// Env files are not loaded via ResourceLoader — pre-fetch them via the SSH
	// loader and pass local paths to WithEnvFiles.
	for _, remotePath := range splitLabelValues(project.EnvironmentFiles) {
		localPath, fetchErr := sshLoader.Load(ctx, remotePath)
		if fetchErr != nil {
			log.Printf("[compose] loadComposeProject: skipping env file %s: %v", remotePath, fetchErr)
			continue
		}
		opts = append(opts, composegocli.WithEnvFiles(localPath))
	}
	// Fetch the default .env; ignore errors if it doesn't exist.
	if dotEnvPath, fetchErr := sshLoader.Load(ctx, filepath.Join(project.WorkingDir, ".env")); fetchErr == nil {
		opts = append(opts, composegocli.WithEnvFiles(dotEnvPath))
	}

	return opts, func() { _ = os.RemoveAll(tmpDir) }, nil
}

// remapProjectPaths replaces any local temp dir prefix in a loaded project's
// paths with the remote working dir, so the remote Docker daemon receives the
// correct absolute paths for bind mounts and compose file references.
func remapProjectPaths(project *composetypes.Project, remoteWorkingDir string) {
	localTmpDir := project.WorkingDir
	project.WorkingDir = remoteWorkingDir

	for i, f := range project.ComposeFiles {
		project.ComposeFiles[i] = remapPath(f, localTmpDir, remoteWorkingDir)
	}

	for name, service := range project.Services {
		for i, vol := range service.Volumes {
			if vol.Type == "bind" {
				vol.Source = remapPath(vol.Source, localTmpDir, remoteWorkingDir)
				service.Volumes[i] = vol
			}
		}
		project.Services[name] = service
	}
}

func remapPath(path, localTmpDir, remoteWorkingDir string) string {
	if strings.HasPrefix(path, localTmpDir) {
		return remoteWorkingDir + path[len(localTmpDir):]
	}
	return path
}

func (s *composeProjectService) ensureProjectNameUnique(ctx context.Context, project ComposeProject) error {
	containers, err := s.listComposeContainers(ctx, project.Name)
	if err != nil {
		return err
	}

	targetKey := composeProjectKey{
		name:             project.Name,
		workingDir:       project.WorkingDir,
		configFiles:      project.ConfigFiles,
		environmentFiles: project.EnvironmentFiles,
	}

	for _, existing := range containers {
		existingKey := composeProjectKey{
			name:             existing.Labels[composeapi.ProjectLabel],
			workingDir:       existing.Labels[composeapi.WorkingDirLabel],
			configFiles:      existing.Labels[composeapi.ConfigFilesLabel],
			environmentFiles: existing.Labels[composeapi.EnvironmentFileLabel],
		}
		if existingKey != targetKey {
			return fmt.Errorf("%w: %s", ErrComposeProjectNameAmbiguous, project.Name)
		}
	}

	return nil
}

func (s *composeProjectService) listComposeContainers(
	ctx context.Context,
	projectName string,
) ([]containerapi.Summary, error) {
	filterArgs := filters.NewArgs(filters.Arg("label", composeapi.ProjectLabel))
	if projectName != "" {
		filterArgs.Add("label", fmt.Sprintf("%s=%s", composeapi.ProjectLabel, projectName))
	}

	return s.engineClient.ContainerList(ctx, containerapi.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
}

func applyComposeLabels(project *composetypes.Project, workingDir, configFiles string, envFiles []string) {
	for name, service := range project.Services {
		if service.CustomLabels == nil {
			service.CustomLabels = map[string]string{}
		}

		service.CustomLabels[composeapi.ProjectLabel] = project.Name
		service.CustomLabels[composeapi.ServiceLabel] = name
		service.CustomLabels[composeapi.VersionLabel] = composeapi.ComposeVersion
		// Use the original remote paths from container labels, not SDK-resolved
		// paths which may point to local temp files on SSH hosts.
		service.CustomLabels[composeapi.WorkingDirLabel] = workingDir
		service.CustomLabels[composeapi.ConfigFilesLabel] = configFiles
		service.CustomLabels[composeapi.OneoffLabel] = "False"
		if len(envFiles) > 0 {
			service.CustomLabels[composeapi.EnvironmentFileLabel] = strings.Join(envFiles, ",")
		}

		project.Services[name] = service
	}
}

func splitLabelValues(values string) []string {
	if values == "" {
		return nil
	}

	parts := strings.Split(values, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}

	if len(names[0]) > 0 && names[0][0] == '/' {
		return names[0][1:]
	}

	return names[0]
}

// parseSSHTarget extracts "user@host" or "host" from an ssh:// URL.
// Returns empty string for non-SSH hosts.
func parseSSHTarget(host string) string {
	if !strings.HasPrefix(host, "ssh://") {
		return ""
	}

	u, err := url.Parse(host)
	if err != nil {
		return ""
	}

	if u.User != nil && u.User.Username() != "" {
		return u.User.Username() + "@" + u.Hostname()
	}

	return u.Hostname()
}

// sshResourceLoader implements loader.ResourceLoader by fetching remote files
// over SSH on demand. All fetched files land in a shared tmpDir so that
// relative cross-references between compose files resolve correctly.
type sshResourceLoader struct {
	sshTarget        string
	remoteWorkingDir string
	tmpDir           string
}

func newSSHResourceLoader(sshTarget, remoteWorkingDir, tmpDir string) *sshResourceLoader {
	return &sshResourceLoader{
		sshTarget:        sshTarget,
		remoteWorkingDir: remoteWorkingDir,
		tmpDir:           tmpDir,
	}
}

// Accept returns true for any path that doesn't already exist locally.
// This covers both absolute remote paths and relative includes that the
// localResourceLoader would otherwise fail to find.
func (r *sshResourceLoader) Accept(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// Load fetches the remote file via SSH and writes it to the shared temp dir.
// Relative paths are resolved against the remote working directory.
// Returns the local path so the Compose SDK can read it normally.
func (r *sshResourceLoader) Load(ctx context.Context, path string) (string, error) {
	remotePath := path
	if !filepath.IsAbs(remotePath) {
		remotePath = filepath.Join(r.remoteWorkingDir, remotePath)
	}

	localPath := filepath.Join(r.tmpDir, filepath.Base(remotePath))

	// Return cached copy if already fetched.
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	log.Printf("[compose] sshResourceLoader: fetching %q from %q", remotePath, r.sshTarget)
	//nolint:gosec // remotePath is derived from Docker container labels on the remote host
	content, err := exec.CommandContext(ctx, "ssh", r.sshTarget, "cat", remotePath).Output()
	if err != nil {
		return "", fmt.Errorf("ssh cat %s:%s: %w", r.sshTarget, remotePath, err)
	}

	if writeErr := os.WriteFile(localPath, content, 0o600); writeErr != nil {
		return "", writeErr
	}

	return localPath, nil
}

// Dir returns the local temp dir so that when the SDK rebuilds ResourceLoaders
// for included files it sets localResourceLoader.WorkingDir to our temp dir,
// not the remote path.
func (r *sshResourceLoader) Dir(_ string) string {
	return r.tmpDir
}

var _ loader.ResourceLoader = (*sshResourceLoader)(nil)
