package client

import (
	"context"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	composeProjectLabel     = "com.docker.compose.project"
	composeWorkingDirLabel  = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel = "com.docker.compose.project.config_files"
	composeServiceLabel     = "com.docker.compose.service"
)

// composeProjectService discovers Compose projects from running container labels.
type composeProjectService struct {
	cli *client.Client
}

type composeProjectKey struct {
	name        string
	workingDir  string
	configFiles string
}

// List loads only Compose-managed containers and aggregates them into
// ComposeProject entries.
func (s *composeProjectService) List(ctx context.Context) ([]ComposeProject, error) {
	log.Printf("[compose] List")
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", composeProjectLabel),
		),
	})
	if err != nil {
		return nil, err
	}

	projectMap := make(map[composeProjectKey]*ComposeProject)
	projectOrder := []composeProjectKey{}

	for _, c := range containers {
		key := composeProjectKey{
			name:        c.Labels[composeProjectLabel],
			workingDir:  c.Labels[composeWorkingDirLabel],
			configFiles: c.Labels[composeConfigFilesLabel],
		}

		proj, exists := projectMap[key]
		if !exists {
			proj = &ComposeProject{
				Name:        key.name,
				WorkingDir:  key.workingDir,
				ConfigFiles: key.configFiles,
			}
			projectMap[key] = proj
			projectOrder = append(projectOrder, key)
		}

		serviceName := c.Labels[composeServiceLabel]
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

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}

	if len(names[0]) > 0 && names[0][0] == '/' {
		return names[0][1:]
	}

	return names[0]
}
