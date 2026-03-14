package client

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/docker/cli/cli/connhelper"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

// dockerClient connects to a local or remote Docker daemon.
type dockerClient struct {
	cli        *client.Client
	containers *containerService
	images     *imageService
	volumes    *volumeService
	networks   *networkService
}

// NewDockerClientFromConfig creates a dockerClient using settings from cfg.
//
// Connection logic:
//   - cfg.Host empty → client.FromEnv (reads DOCKER_HOST, etc. from environment)
//   - cfg.Host is ssh:// AND identity_file set → custom SSH dialer with key file auth
//   - cfg.Host is ssh:// AND no identity_file → custom SSH dialer with SSH agent auth
//   - cfg.Host is anything else (tcp://, unix://) → client.WithHost directly
func NewDockerClientFromConfig(cfg config.DockerConfig) (Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	switch {
	case cfg.Host == "":
		opts = append(opts, client.FromEnv)
	case isSSHHost(cfg.Host):
		helper, err := connhelper.GetConnectionHelper(cfg.Host)
		if err != nil {
			return nil, err
		}

		httpClient := &http.Client{
			Transport: &http.Transport{
				DialContext: helper.Dialer,
			},
		}

		opts = append(opts,
			client.WithHTTPClient(httpClient),
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)

	default:
		opts = append(opts,
			client.WithHost(cfg.Host),
		)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	c := &dockerClient{cli: cli}
	c.containers = &containerService{cli: cli}
	c.images = &imageService{cli: cli}
	c.volumes = &volumeService{cli: cli}
	c.networks = &networkService{cli: cli}
	return c, nil
}

// isSSHHost reports whether host is an ssh:// URL.
func isSSHHost(host string) bool {
	return strings.HasPrefix(host, "ssh://")
}

func (c *dockerClient) Containers() ContainerService { return c.containers }
func (c *dockerClient) Images() ImageService         { return c.images }
func (c *dockerClient) Volumes() VolumeService       { return c.volumes }
func (c *dockerClient) Networks() NetworkService     { return c.networks }

func (c *dockerClient) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *dockerClient) Close() error {
	return c.cli.Close()
}

// Local Container Service.
type containerService struct {
	cli *client.Client
}

func (s *containerService) List(ctx context.Context) ([]Container, error) {
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

func (s *containerService) Run(ctx context.Context, image Image) (string, error) {
	ports := nat.PortSet{}

	for port := range image.Config.ExposedPorts {
		natPort, err := nat.NewPort(nat.SplitProtoPort(port))
		if err != nil {
			return "", err
		}
		ports[natPort] = struct{}{}
	}

	config := &container.Config{
		User:         image.Config.User,
		WorkingDir:   image.Config.WorkingDir,
		Labels:       image.Config.Labels,
		Env:          image.Config.Env,
		Cmd:          image.Config.Cmd,
		Entrypoint:   image.Config.Entrypoint,
		Image:        image.Name(),
		Shell:        image.Config.Shell,
		OnBuild:      image.Config.OnBuild,
		Volumes:      image.Config.Volumes,
		ExposedPorts: ports,
		Healthcheck:  image.Config.Healthcheck,
	}

	containerResponse, err := s.cli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return "", err
	}
	err = s.Start(ctx, containerResponse.ID)
	if err != nil {
		return "", err
	}
	return containerResponse.ID, nil
}

func (s *containerService) Get(ctx context.Context, id string) (*Container, error) {
	c, err := s.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	state := StateStopped
	switch {
	case c.State.Running:
		state = StateRunning
	case c.State.Paused:
		state = StatePaused
	case c.State.Restarting:
		state = StateRestarting
	}

	var created time.Time
	created, err = time.Parse(time.RFC3339Nano, c.Created)
	if err != nil {
		return nil, fmt.Errorf("parsing container created time: %w", err)
	}

	ports := make([]PortMapping, 0)
	for port, bindings := range c.NetworkSettings.Ports {
		containerPort, containerPortErr := strconv.ParseUint(port.Port(), 10, 16)
		if containerPortErr != nil {
			continue
		}
		for _, b := range bindings {
			hostPort, hostPortErr := strconv.ParseUint(b.HostPort, 10, 16)
			if hostPortErr != nil {
				continue
			}
			ports = append(ports, PortMapping{
				HostPort:      uint16(hostPort),
				ContainerPort: uint16(containerPort),
				Protocol:      port.Proto(),
			})
		}
	}

	mounts := make([]Mount, len(c.Mounts))
	for i, m := range c.Mounts {
		mounts[i] = Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
		}
	}

	networks := make([]NetworkInfo, 0, len(c.NetworkSettings.Networks))
	for name, n := range c.NetworkSettings.Networks {
		networks = append(networks, NetworkInfo{
			Name:      name,
			IPAddress: n.IPAddress,
			Gateway:   n.Gateway,
			Aliases:   n.Aliases,
		})
	}

	restartPolicy := string(c.HostConfig.RestartPolicy.Name)
	if c.HostConfig.RestartPolicy.MaximumRetryCount > 0 {
		restartPolicy = fmt.Sprintf("%s:%d", restartPolicy, c.HostConfig.RestartPolicy.MaximumRetryCount)
	}

	return &Container{
		ID:      c.ID,
		Name:    strings.TrimPrefix(c.Name, "/"),
		Image:   c.Config.Image,
		Status:  c.State.Status,
		State:   state,
		Created: created,
		Ports:   ports,
		Mounts:  mounts,
		// Networking
		Hostname:    c.Config.Hostname,
		NetworkMode: string(c.HostConfig.NetworkMode),
		Networks:    networks,
		// Runtime config
		Cmd:        c.Config.Cmd,
		Entrypoint: c.Config.Entrypoint,
		WorkingDir: c.Config.WorkingDir,
		Env:        c.Config.Env,
		Labels:     c.Config.Labels,
		// Resource limits
		MemoryLimit:   c.HostConfig.Memory,
		CPUShares:     c.HostConfig.CPUShares,
		RestartPolicy: restartPolicy,
		Privileged:    c.HostConfig.Privileged,
	}, nil
}

func (s *containerService) Start(ctx context.Context, id string) error {
	return s.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (s *containerService) Stop(ctx context.Context, id string) error {
	return s.cli.ContainerStop(ctx, id, container.StopOptions{})
}

func (s *containerService) Restart(ctx context.Context, id string) error {
	return s.cli.ContainerRestart(ctx, id, container.StopOptions{})
}

func (s *containerService) Remove(ctx context.Context, id string, force bool) error {
	return s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

func (s *containerService) Logs(ctx context.Context, id string, opts LogOptions) (*LogsSession, error) {
	now := time.Now()
	reader, err := s.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Timestamps: opts.Timestamps,
		Since:      strconv.FormatInt(now.Add(-time.Hour*logsSinceHours).Unix(), 10),
	})

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(pw, pw, reader)
		pw.CloseWithError(copyErr)
	}()

	return NewLogsSession(
		io.NopCloser(pr),
		func() { _ = reader.Close() },
	), nil
}

func (s *containerService) Exec(ctx context.Context, id string) (*ExecSession, error) {
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

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(pw, pw, attachResp.Reader)
		pw.CloseWithError(copyErr)
	}()

	return NewExecSession(
		io.NopCloser(pr),
		attachResp.Conn,
		func() { attachResp.Close() },
	), nil
}

func (s *containerService) Stats(ctx context.Context, id string) (*StatsSession, error) {
	reader, err := s.cli.ContainerStats(ctx, id, true)

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := io.Copy(pw, reader.Body)
		pw.CloseWithError(copyErr)
	}()

	return NewStatsSession(
		io.NopCloser(pr),
		func() { _ = reader.Body.Close() },
	), nil
}

func (s *containerService) Prune(ctx context.Context, _ PruneOptions) (PruneReport, error) {
	r, err := s.cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return PruneReport{}, err
	}
	return PruneReport{ItemsDeleted: len(r.ContainersDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

func (s *containerService) FileTree(ctx context.Context, id string) (*FileNode, error) {
	reader, err := s.cli.ContainerExport(ctx, id)
	if err != nil {
		return &FileNode{}, err
	}

	defer reader.Close()

	return buildContainerFileTree(reader), nil
}

func buildContainerFileTree(reader io.ReadCloser) *FileNode {
	tr := tar.NewReader(reader)
	t := &FileNode{Name: ".", Path: ".", IsDir: true}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		file := hdr.Name
		subTree := t
		isDir := hdr.Typeflag == tar.TypeDir
		clean := strings.TrimSuffix(file, "/")
		parts := strings.Split(clean, "/")

		length := len(parts)

		for i, part := range parts {
			if part == "." || part == "" {
				continue
			}

			isLast := i == length-1

			if isLast && !isDir {
				// file entry — add as leaf and move to next part
				child := &FileNode{
					Name:     part,
					Path:     file,
					IsDir:    false,
					Linkname: hdr.Linkname,
					Size:     hdr.Size,
					Mode:     hdr.FileInfo().Mode(),
					Parent:   subTree,
					Depth:    i + 1,
				}
				subTree.Children = append(subTree.Children, child)
				continue
			}

			// directory entry — find existing subtree or create one
			found := false
			for _, j := range subTree.Children {
				if j.Name == part {
					subTree = j
					found = true
					break
				}
			}
			if !found {
				c := &FileNode{
					Name:   part,
					Path:   strings.Join(parts[:i+1], "/"),
					IsDir:  true,
					Parent: subTree,
					Depth:  i + 1,
				}
				subTree.Children = append(subTree.Children, c)
				subTree = c
			}
		}
	}

	return t
}

// Local Image Service.
type imageService struct {
	cli *client.Client
}

func (s *imageService) List(ctx context.Context) ([]Image, error) {
	images, err := s.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Image, len(images))
	for i, img := range images {
		repo := none
		tag := none
		if len(img.RepoTags) > 0 {
			parts := strings.SplitN(img.RepoTags[0], ":", repoTagParts)
			repo = parts[0]
			if len(parts) > 1 {
				tag = parts[1]
			}
		}

		imageData := Image{
			ID:       img.ID,
			Repo:     repo,
			Tag:      tag,
			Size:     img.Size,
			Created:  time.Unix(img.Created, 0),
			Dangling: len(img.RepoTags) == 0 || repo == none && tag == none,
		}
		imageData.Containers = img.Containers
		result[i] = imageData
	}

	return result, nil
}

// fetchLayers retrieves the layer history for an image.
func (s *imageService) fetchLayers(ctx context.Context, imageID string) []Layer {
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
	slices.Reverse(layers)
	return layers
}

func (s *imageService) Get(ctx context.Context, id string) (Image, error) {
	img, err := s.cli.ImageInspect(ctx, id, client.ImageInspectWithManifests(true))
	if err != nil {
		return Image{}, err
	}

	repo := none
	tag := none
	if len(img.RepoTags) > 0 {
		parts := strings.SplitN(img.RepoTags[0], ":", repoTagParts)
		repo = parts[0]
		if len(parts) > 1 {
			tag = parts[1]
		}
	}

	created, err := time.Parse(time.RFC3339Nano, img.Created)
	if err != nil {
		return Image{}, err
	}

	// Fetch layer history for this image
	layers := s.fetchLayers(ctx, img.ID)

	return Image{
		ID:       img.ID,
		Repo:     repo,
		Tag:      tag,
		Size:     img.Size,
		Created:  created,
		Dangling: len(img.RepoTags) == 0 || repo == none && tag == none,
		Layers:   layers,
		Config:   img.Config,
	}, nil
}

func (s *imageService) Remove(ctx context.Context, id string, force bool) error {
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	return err
}

func (s *imageService) Prune(ctx context.Context, opts PruneOptions) (PruneReport, error) {
	f := filters.Args{}
	if opts.All {
		f = filters.NewArgs(filters.Arg("dangling", "false"))
	}
	r, err := s.cli.ImagesPrune(ctx, f)
	if err != nil {
		return PruneReport{}, err
	}
	return PruneReport{ItemsDeleted: len(r.ImagesDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

// Local Volume Service.
type volumeService struct {
	cli *client.Client
}

func (s *volumeService) List(ctx context.Context) ([]Volume, error) {
	du, err := s.cli.DiskUsage(ctx, dockertypes.DiskUsageOptions{
		Types: []dockertypes.DiskUsageObject{dockertypes.VolumeObject},
	})
	if err != nil {
		return nil, err
	}

	result := make([]Volume, len(du.Volumes))
	for i, v := range du.Volumes {
		size := int64(0)
		usedCount := 0

		if v.UsageData != nil {
			size = v.UsageData.Size
			usedCount = int(v.UsageData.RefCount)
		}

		result[i] = Volume{
			Name:      v.Name,
			Driver:    v.Driver,
			MountPath: v.Mountpoint,
			Size:      size,
			UsedCount: usedCount,
		}
	}

	return result, nil
}

func (s *volumeService) Remove(ctx context.Context, name string, force bool) error {
	return s.cli.VolumeRemove(ctx, name, force)
}

func (s *volumeService) Prune(ctx context.Context, opts PruneOptions) (PruneReport, error) {
	f := filters.Args{}
	if opts.All {
		f = filters.NewArgs(filters.Arg("all", "true"))
	}
	r, err := s.cli.VolumesPrune(ctx, f)
	if err != nil {
		return PruneReport{}, err
	}
	return PruneReport{ItemsDeleted: len(r.VolumesDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

func (s *volumeService) FileTree(ctx context.Context, name string) (*FileNode, error) {
	// Try to find a running container that mounts this volume
	containerID, mountPoint, err := s.findRunningContainerForVolume(ctx, name)
	if err != nil {
		return &FileNode{}, err
	}

	if containerID != "" {
		return s.copyFileTree(ctx, containerID, mountPoint)
	}

	// No running container — spin up a temporary one to browse the volume
	return s.fileTreeViaTempContainer(ctx, name)
}

// findRunningContainerForVolume returns the ID and mount point of a running
// container that uses the given volume, or empty strings if none found.
func (s *volumeService) findRunningContainerForVolume(ctx context.Context, name string) (string, string, error) {
	usedBy, err := s.getVolumeUsage(ctx, name)
	if err != nil {
		return "", "", err
	}

	for _, cID := range usedBy {
		inspect, inspectErr := s.cli.ContainerInspect(ctx, cID)
		if inspectErr != nil || !inspect.State.Running {
			continue
		}
		for _, m := range inspect.Mounts {
			if m.Name == name {
				return cID, m.Destination, nil
			}
		}
	}

	return "", "", nil
}

// copyFileTree reads a tar archive from a container path and builds a file tree.
func (s *volumeService) copyFileTree(ctx context.Context, containerID, path string) (*FileNode, error) {
	reader, _, err := s.cli.CopyFromContainer(ctx, containerID, path)
	if err != nil {
		return &FileNode{}, fmt.Errorf("copy from container failed: %w", err)
	}
	defer reader.Close()

	return buildContainerFileTree(reader), nil
}

const (
	volumeMountPath = "/mnt/volume"
	alpineImage     = "alpine:latest"
	logsSinceHours  = 2 // hours of log history to fetch
	repoTagParts    = 2 // parts when splitting repo:tag on ":"
)

// ensureImage pulls the image if it doesn't exist locally.
func (s *volumeService) ensureImage(ctx context.Context, ref string) error {
	_, err := s.cli.ImageInspect(ctx, ref)
	if err == nil {
		return nil // already exists
	}

	reader, err := s.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull %s: %w", ref, err)
	}
	defer reader.Close()
	// Drain the reader to complete the pull
	_, err = io.Copy(io.Discard, reader)
	return err
}

// fileTreeViaTempContainer creates a temporary alpine container to browse
// a volume that is not in use by any running container.
func (s *volumeService) fileTreeViaTempContainer(ctx context.Context, volumeName string) (*FileNode, error) {
	if err := s.ensureImage(ctx, alpineImage); err != nil {
		return &FileNode{}, err
	}

	resp, err := s.cli.ContainerCreate(ctx, &container.Config{
		Image: alpineImage,
		Cmd:   []string{"true"},
	}, &container.HostConfig{
		Binds: []string{volumeName + ":" + volumeMountPath},
	}, nil, nil, "")
	if err != nil {
		return &FileNode{}, fmt.Errorf("failed to create temp container: %w", err)
	}

	// Always clean up the temporary container
	defer func() { _ = s.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }()

	ft, err := s.copyFileTree(ctx, resp.ID, volumeMountPath)
	if err != nil {
		return &FileNode{}, fmt.Errorf("failed to read volume files: %w", err)
	}

	return ft, nil
}

// Helper to get containers using a volume.
func (s *volumeService) getVolumeUsage(ctx context.Context, volumeName string) ([]string, error) {
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

// Local Network Service.
type networkService struct {
	cli *client.Client
}

func (s *networkService) List(ctx context.Context) ([]Network, error) {
	networks, err := s.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]Network, len(networks))
	for i, n := range networks {
		inspectResponse, inspectErr := s.cli.NetworkInspect(ctx, n.ID, network.InspectOptions{})

		if inspectErr != nil {
			return nil, inspectErr
		}

		subnet := ""
		gateway := ""
		if len(n.IPAM.Config) > 0 {
			subnet = inspectResponse.IPAM.Config[0].Subnet
			gateway = inspectResponse.IPAM.Config[0].Gateway
		}
		connected := make([]NetworkContainer, 0, len(inspectResponse.Containers))
		for _, c := range inspectResponse.Containers {
			connected = append(connected, NetworkContainer{
				Name:        c.Name,
				IPv4Address: c.IPv4Address,
				IPv6Address: c.IPv6Address,
				MacAddress:  c.MacAddress,
			})
		}
		result[i] = Network{
			ID:                  n.ID,
			Name:                n.Name,
			Driver:              n.Driver,
			Scope:               n.Scope,
			Internal:            n.Internal,
			Created:             n.Created,
			ConnectedContainers: connected,
			IPAM:                NetworkIPAM{Subnet: subnet, Gateway: gateway},
		}
	}
	return result, nil
}

func (s *networkService) Remove(ctx context.Context, id string) error {
	return s.cli.NetworkRemove(ctx, id)
}

func (s *networkService) Prune(ctx context.Context, _ PruneOptions) (PruneReport, error) {
	r, err := s.cli.NetworksPrune(ctx, filters.Args{})
	if err != nil {
		return PruneReport{}, err
	}
	return PruneReport{ItemsDeleted: len(r.NetworksDeleted), SpaceReclaimed: 0}, nil
}

// timeFromUnix converts Unix timestamp to time.Time.
func timeFromUnix(unix int64) time.Time {
	return time.Unix(unix, 0)
}
