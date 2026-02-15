package service

import (
	"archive/tar"
	"context"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/tree"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// LocalDockerClient connects to the local Docker daemon
type LocalDockerClient struct {
	cli        *client.Client
	containers *localContainerService
	images     *localImageService
	volumes    *localVolumeService
}

// NewLocalDockerClient creates a client connected to the local Docker socket
func NewLocalDockerClient() (*LocalDockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	c := &LocalDockerClient{cli: cli}
	c.containers = &localContainerService{cli: cli}
	c.images = &localImageService{cli: cli}
	c.volumes = &localVolumeService{cli: cli}

	return c, nil
}

func (c *LocalDockerClient) Containers() ContainerService { return c.containers }
func (c *LocalDockerClient) Images() ImageService         { return c.images }
func (c *LocalDockerClient) Volumes() VolumeService       { return c.volumes }

func (c *LocalDockerClient) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *LocalDockerClient) Close() error {
	return c.cli.Close()
}

// Local Container Service
type localContainerService struct {
	cli *client.Client
}

func (s *localContainerService) List(ctx context.Context) ([]Container, error) {
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Container, len(containers))
	for i, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		state := StateStopped
		switch c.State {
		case "running":
			state = StateRunning
		case "paused":
			state = StatePaused
		case "restarting":
			state = StateRestarting
		}

		ports := make([]PortMapping, 0)
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, PortMapping{
					HostPort:      p.PublicPort,
					ContainerPort: p.PrivatePort,
					Protocol:      p.Type,
				})
			}
		}

		mounts := make([]Mount, len(c.Mounts))
		for j, m := range c.Mounts {
			mounts[j] = Mount{
				Type:        string(m.Type),
				Source:      m.Source,
				Destination: m.Destination,
			}
		}

		result[i] = Container{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   state,
			Created: timeFromUnix(c.Created),
			Ports:   ports,
			Mounts:  mounts,
		}
	}

	return result, nil
}

func (s *localContainerService) Get(ctx context.Context, id string) (*Container, error) {
	c, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	state := StateStopped
	if c.State.Running {
		state = StateRunning
	} else if c.State.Paused {
		state = StatePaused
	} else if c.State.Restarting {
		state = StateRestarting
	}

	return &Container{
		ID:     c.ID,
		Name:   strings.TrimPrefix(c.Name, "/"),
		Image:  c.Config.Image,
		Status: c.State.Status,
		State:  state,
	}, nil
}

func (s *localContainerService) Start(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *localContainerService) Stop(ctx context.Context, id string) error {
	return s.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (s *localContainerService) Restart(ctx context.Context, id string) error {
	return s.cli.ContainerRestart(ctx, id, container.StopOptions{})
}

func (s *localContainerService) Remove(ctx context.Context, id string, force bool) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

func (s *localContainerService) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	return s.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Timestamps: opts.Timestamps,
	})
}

func (s *localContainerService) Exec(ctx context.Context, id string) (*ExecSession, error) {
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          []string{"/bin/sh"},
	}

	execResp, err := s.cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return nil, err
	}

	attachResp, err := s.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, err
	}

	return NewExecSession(
		io.NopCloser(attachResp.Reader),
		attachResp.Conn,
		func() { attachResp.Close() },
	), nil
}

func (s *localContainerService) FileTree(ctx context.Context, id string) (ContainerFileTree, error) {
	reader, err := s.cli.ContainerExport(ctx, "51bc63fdf4eb47ec2699a7affd382b487d065ea6fbf5ccfc3106e9b4f2ee64f4")
	if err != nil {
		return ContainerFileTree{}, err
	}

	defer reader.Close()

	return buildContainerFileTree(reader), nil
}

func buildContainerFileTree(reader io.ReadCloser) ContainerFileTree {
	tr := tar.NewReader(reader)
	files := []string{}
	t := tree.Root(".")
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		file := hdr.Name
		files = append(files, file)
		subTree := t
		isDir := hdr.Typeflag == tar.TypeDir
		clean := strings.TrimSuffix(file, "/")
		parts := strings.Split(clean, "/")

		for i, part := range parts {
			if part == "." || part == "" {
				continue
			}

			isLast := i == len(parts)-1

			if isLast && !isDir {
				// file entry — add as leaf
				if hdr.Linkname != "" {
					subTree.Child(part + " -> " + hdr.Linkname)
				} else {
					subTree.Child(part)
				}
			} else {
				// directory entry — find existing subtree or create one
				found := false
				children := subTree.Children()
				for j := range children.Length() {
					child := children.At(j)
					if sub, ok := child.(*tree.Tree); ok && child.Value() == part {
						subTree = sub
						found = true
						break
					}
				}
				if !found {
					c := tree.Root(part)
					subTree.Child(c)
					subTree = c
				}
			}
		}
	}

	return ContainerFileTree{Files: files, Tree: t}
}

// Local Image Service
type localImageService struct {
	cli *client.Client
}

func (s *localImageService) List(ctx context.Context) ([]Image, error) {
	images, err := s.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Image, len(images))
	for i, img := range images {
		repo := "<none>"
		tag := "<none>"
		if len(img.RepoTags) > 0 {
			parts := strings.SplitN(img.RepoTags[0], ":", 2)
			repo = parts[0]
			if len(parts) > 1 {
				tag = parts[1]
			}
		}

		// Fetch layer history for this image
		layers := s.fetchLayers(ctx, img.ID)

		result[i] = Image{
			ID:         img.ID,
			Repo:       repo,
			Tag:        tag,
			Size:       img.Size,
			Created:    timeFromUnix(img.Created),
			Dangling:   len(img.RepoTags) == 0 || img.RepoTags[0] == "<none>:<none>",
			Layers:     layers,
			Containers: img.Containers,
		}
	}

	return result, nil
}

// fetchLayers retrieves the layer history for an image
func (s *localImageService) fetchLayers(ctx context.Context, imageID string) []Layer {
	history, err := s.cli.ImageHistory(ctx, imageID)
	if err != nil {
		return nil
	}

	layers := make([]Layer, 0, len(history))
	for _, h := range history {
		layers = append(layers, Layer{
			ID:      h.ID,
			Command: h.CreatedBy,
			Size:    h.Size,
			Created: timeFromUnix(h.Created),
		})
	}

	return layers
}

func (s *localImageService) Get(ctx context.Context, id string) (*Image, error) {
	img, _, err := s.cli.ImageInspectWithRaw(ctx, id)
	if err != nil {
		return nil, err
	}

	repo := "<none>"
	tag := "<none>"
	if len(img.RepoTags) > 0 {
		parts := strings.SplitN(img.RepoTags[0], ":", 2)
		repo = parts[0]
		if len(parts) > 1 {
			tag = parts[1]
		}
	}

	return &Image{
		ID:   img.ID,
		Repo: repo,
		Tag:  tag,
		Size: img.Size,
	}, nil
}

func (s *localImageService) Remove(ctx context.Context, id string, force bool) error {
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	return err
}

// Local Volume Service
type localVolumeService struct {
	cli *client.Client
}

func (s *localVolumeService) List(ctx context.Context) ([]Volume, error) {
	resp, err := s.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]Volume, len(resp.Volumes))
	for i, v := range resp.Volumes {
		result[i] = Volume{
			Name:      v.Name,
			Driver:    v.Driver,
			MountPath: v.Mountpoint,
			// Note: Size requires disk usage API call, skipping for now
		}
	}

	return result, nil
}

func (s *localVolumeService) Get(ctx context.Context, name string) (*Volume, error) {
	v, err := s.cli.VolumeInspect(ctx, name)
	if err != nil {
		return nil, err
	}

	return &Volume{
		Name:      v.Name,
		Driver:    v.Driver,
		MountPath: v.Mountpoint,
	}, nil
}

func (s *localVolumeService) Remove(ctx context.Context, name string, force bool) error {
	return s.cli.VolumeRemove(ctx, name, force)
}

func (s *localVolumeService) Browse(ctx context.Context, name string, path string) ([]FileEntry, error) {
	// Volume browsing requires running a container to access the volume
	// For now, return empty - this is a complex feature
	return nil, nil
}

// Helper to get containers using a volume
func (s *localVolumeService) getVolumeUsage(ctx context.Context, volumeName string) ([]string, error) {
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("volume", volumeName)),
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	return ids, nil
}

// timeFromUnix converts Unix timestamp to time.Time
func timeFromUnix(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// Ensure LocalDockerClient implements DockerClient
var _ DockerClient = (*LocalDockerClient)(nil)
