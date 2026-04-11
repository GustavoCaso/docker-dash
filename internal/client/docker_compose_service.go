package client

import (
	"context"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// composeProjectService discovers Compose projects from running container labels.
type composeProjectService struct {
	cli *client.Client
}

// List scans all containers for Docker Compose labels and aggregates them
// into ComposeProject entries.
func (s *composeProjectService) List(ctx context.Context) ([]ComposeProject, error) {
	log.Printf("[compose] List")
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// Use a map to accumulate services per project.
	type projectKey struct{ name, workingDir, configFiles string }
	projectMap := make(map[string]*ComposeProject)
	projectOrder := []string{}

	for _, c := range containers {
		projectName := c.Labels["com.docker.compose.project"]
		if projectName == "" {
			continue
		}

		proj, exists := projectMap[projectName]
		if !exists {
			proj = &ComposeProject{
				Name:        projectName,
				WorkingDir:  c.Labels["com.docker.compose.project.working_dir"],
				ConfigFiles: c.Labels["com.docker.compose.project.config_files"],
			}
			projectMap[projectName] = proj
			projectOrder = append(projectOrder, projectName)
		}

		serviceName := c.Labels["com.docker.compose.service"]
		if serviceName == "" {
			serviceName = c.Names[0]
		}

		image := c.Image
		state := c.State

		proj.Services = append(proj.Services, ComposeServiceInfo{
			Name:  serviceName,
			State: state,
			Image: image,
		})
	}

	projects := make([]ComposeProject, 0, len(projectOrder))
	for _, name := range projectOrder {
		projects = append(projects, *projectMap[name])
	}

	log.Printf("[compose] List: found %d projects", len(projects))
	return projects, nil
}
