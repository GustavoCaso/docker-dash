package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	composegocli "github.com/compose-spec/compose-go/v2/cli"
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
	}
	svc.loadProject = svc.loadComposeProject

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

	loadedProject, err := s.loadProject(ctx, project)
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

	projectOptions, err := composegocli.NewProjectOptions(
		configPaths,
		composegocli.WithWorkingDirectory(project.WorkingDir),
		composegocli.WithOsEnv,
		composegocli.WithEnvFiles(splitLabelValues(project.EnvironmentFiles)...),
		composegocli.WithDotEnv,
		composegocli.WithName(project.Name),
		composegocli.WithResolvedPaths(true),
	)
	if err != nil {
		return nil, err
	}

	loadedProject, err := projectOptions.LoadProject(ctx)
	if err != nil {
		return nil, err
	}

	applyComposeLabels(loadedProject, splitLabelValues(project.EnvironmentFiles))

	return loadedProject, nil
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

func applyComposeLabels(project *composetypes.Project, envFiles []string) {
	for name, service := range project.Services {
		if service.CustomLabels == nil {
			service.CustomLabels = map[string]string{}
		}

		service.CustomLabels[composeapi.ProjectLabel] = project.Name
		service.CustomLabels[composeapi.ServiceLabel] = name
		service.CustomLabels[composeapi.VersionLabel] = composeapi.ComposeVersion
		service.CustomLabels[composeapi.WorkingDirLabel] = project.WorkingDir
		service.CustomLabels[composeapi.ConfigFilesLabel] = strings.Join(project.ComposeFiles, ",")
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
