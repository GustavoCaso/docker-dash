package client

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
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
	log.Printf("[docker] Ping")
	_, err := c.cli.Ping(ctx)
	log.Printf("[docker] Ping: done err=%v", err)
	return err
}

func (c *dockerClient) Close() error {
	log.Printf("[docker] Close")
	return c.cli.Close()
}

// Local Container Service.
type containerService struct {
	cli *client.Client
}

func (s *containerService) List(ctx context.Context) ([]Container, error) {
	log.Printf("[docker] ContainerList: all=true")
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

	log.Printf("[docker] ContainerList: returned count=%d err=%v", len(result), err)
	return result, nil
}

func (s *containerService) Run(ctx context.Context, img Image) (string, error) {
	log.Printf("[docker] ContainerCreate+Start: image=%q", img.Name())

	ports := nat.PortSet{}

	for port := range img.Config.ExposedPorts {
		natPort, err := nat.NewPort(nat.SplitProtoPort(port))
		if err != nil {
			return "", err
		}
		ports[natPort] = struct{}{}
	}

	config := &container.Config{
		User:         img.Config.User,
		WorkingDir:   img.Config.WorkingDir,
		Labels:       img.Config.Labels,
		Env:          img.Config.Env,
		Cmd:          img.Config.Cmd,
		Entrypoint:   img.Config.Entrypoint,
		Image:        img.Name(),
		Shell:        img.Config.Shell,
		OnBuild:      img.Config.OnBuild,
		Volumes:      img.Config.Volumes,
		ExposedPorts: ports,
		Healthcheck:  img.Config.Healthcheck,
	}

	containerResponse, err := s.cli.ContainerCreate(ctx, config, nil, nil, nil, "")
	if err != nil {
		return "", err
	}
	err = s.Start(ctx, containerResponse.ID)
	if err != nil {
		return "", err
	}
	log.Printf("[docker] ContainerCreate+Start: containerID=%q", containerResponse.ID)
	return containerResponse.ID, nil
}

func (s *containerService) Get(ctx context.Context, id string) (*Container, error) {
	log.Printf("[docker] ContainerInspect: id=%q", id)
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

	log.Printf("[docker] ContainerInspect: done")
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
	log.Printf("[docker] ContainerStart: id=%q", id)
	err := s.cli.ContainerStart(ctx, id, container.StartOptions{})
	log.Printf("[docker] ContainerStart: done err=%v", err)
	return err
}

func (s *containerService) Stop(ctx context.Context, id string) error {
	log.Printf("[docker] ContainerStop: id=%q", id)
	err := s.cli.ContainerStop(ctx, id, container.StopOptions{})
	log.Printf("[docker] ContainerStop: done err=%v", err)
	return err
}

func (s *containerService) Restart(ctx context.Context, id string) error {
	log.Printf("[docker] ContainerRestart: id=%q", id)
	err := s.cli.ContainerRestart(ctx, id, container.StopOptions{})
	log.Printf("[docker] ContainerRestart: done err=%v", err)
	return err
}

func (s *containerService) Remove(ctx context.Context, id string, force bool) error {
	log.Printf("[docker] ContainerRemove: id=%q force=%v", id, force)
	err := s.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
	log.Printf("[docker] ContainerRemove: done err=%v", err)
	return err
}

func (s *containerService) Logs(ctx context.Context, id string, opts LogOptions) (*LogsSession, error) {
	log.Printf("[docker] ContainerLogs: id=%q follow=%v tail=%q", id, opts.Follow, opts.Tail)
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
		pw.CloseWithError(copyStream(pw, reader))
	}()

	log.Printf("[docker] ContainerLogs: session created")
	return NewLogsSession(
		io.NopCloser(pr),
		func() { _ = reader.Close() },
	), nil
}

// maxMuxStreamType is the highest valid STREAM_TYPE value in Docker's multiplexed stream format.
// Per the Docker API docs: 0=stdin, 1=stdout, 2=stderr.
// Frames start with an 8-byte header: [STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4].
const maxMuxStreamType = 0x02

// copyStream detects whether r is a Docker multiplexed stream or a raw stream,
// then copies accordingly. This handles engines like OrbStack and Colima that
// may not use Docker's multiplexed format.
func copyStream(w io.Writer, r io.Reader) error {
	var streamType [1]byte
	_, err := io.ReadFull(r, streamType[:])
	if err != nil {
		return err
	}

	full := io.MultiReader(strings.NewReader(string(streamType[:])), r)

	if streamType[0] <= maxMuxStreamType {
		_, err = stdcopy.StdCopy(w, w, full)
	} else {
		log.Printf("[docker] ContainerLogs: raw stream detected (first byte=0x%02x), using io.Copy", streamType[0])
		_, err = io.Copy(w, full)
	}
	return err
}

func (s *containerService) Exec(ctx context.Context, id string) (*ExecSession, error) {
	log.Printf("[docker] ContainerExecCreate+Attach: id=%q", id)
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

	log.Printf("[docker] ContainerExecCreate+Attach: done")
	return NewExecSession(
		io.NopCloser(pr),
		attachResp.Conn,
		func() { attachResp.Close() },
	), nil
}

func (s *containerService) Stats(ctx context.Context, id string) (*StatsSession, error) {
	log.Printf("[docker] ContainerStats: id=%q stream=true", id)
	reader, err := s.cli.ContainerStats(ctx, id, true)

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := io.Copy(pw, reader.Body)
		pw.CloseWithError(copyErr)
	}()

	log.Printf("[docker] ContainerStats: session created")
	return NewStatsSession(
		io.NopCloser(pr),
		func() { _ = reader.Body.Close() },
	), nil
}

func (s *containerService) Prune(ctx context.Context, _ PruneOptions) (PruneReport, error) {
	log.Printf("[docker] ContainersPrune")
	r, err := s.cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return PruneReport{}, err
	}
	log.Printf("[docker] ContainersPrune: deleted=%d spaceReclaimed=%d", len(r.ContainersDeleted), r.SpaceReclaimed)
	return PruneReport{ItemsDeleted: len(r.ContainersDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

func (s *containerService) FileTree(ctx context.Context, id string) (*FileNode, error) {
	log.Printf("[docker] ContainerExport (file tree): id=%q", id)
	reader, err := s.cli.ContainerExport(ctx, id)
	if err != nil {
		return &FileNode{}, err
	}

	defer reader.Close()

	log.Printf("[docker] ContainerExport: done")
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
	log.Printf("[docker] ImageList: all=true")
	images, err := s.cli.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]Image, len(images))
	for i, img := range images {
		imageData, imageErr := s.get(ctx, img.ID)
		if imageErr != nil {
			return []Image{}, imageErr
		}

		imageData.Containers = img.Containers
		result[i] = imageData
	}

	log.Printf("[docker] ImageList: returned count=%d", len(result))
	return result, nil
}

// FetchLayers retrieves the layer history for an image.
func (s *imageService) FetchLayers(ctx context.Context, imageID string) []Layer {
	log.Printf("[docker] ImageHistory: id=%q", imageID)
	history, err := s.cli.ImageHistory(ctx, imageID)
	if err != nil {
		return []Layer{}
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
	log.Printf("[docker] ImageHistory: returned count=%d", len(layers))
	return layers
}

func (s *imageService) get(ctx context.Context, id string) (Image, error) {
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

	return Image{
		ID:       img.ID,
		Repo:     repo,
		Tag:      tag,
		Size:     img.Size,
		Created:  created,
		Dangling: len(img.RepoTags) == 0 || repo == none && tag == none,
		Config:   img.Config,
	}, nil
}

func (s *imageService) Remove(ctx context.Context, id string, force bool) error {
	log.Printf("[docker] ImageRemove: id=%q force=%v", id, force)
	_, err := s.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	log.Printf("[docker] ImageRemove: done err=%v", err)
	return err
}

func (s *imageService) Prune(ctx context.Context, opts PruneOptions) (PruneReport, error) {
	log.Printf("[docker] ImagesPrune: all=%v", opts.All)
	f := filters.Args{}
	if opts.All {
		f = filters.NewArgs(filters.Arg("dangling", "false"))
	}
	r, err := s.cli.ImagesPrune(ctx, f)
	if err != nil {
		return PruneReport{}, err
	}
	log.Printf("[docker] ImagesPrune: deleted=%d spaceReclaimed=%d", len(r.ImagesDeleted), r.SpaceReclaimed)
	return PruneReport{ItemsDeleted: len(r.ImagesDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

// Local Volume Service.
type volumeService struct {
	cli *client.Client
}

func (s *volumeService) List(ctx context.Context) ([]Volume, error) {
	log.Printf("[docker] DiskUsage (volumes)")
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

	log.Printf("[docker] DiskUsage: returned count=%d", len(result))
	return result, nil
}

func (s *volumeService) Remove(ctx context.Context, name string, force bool) error {
	log.Printf("[docker] VolumeRemove: name=%q force=%v", name, force)
	err := s.cli.VolumeRemove(ctx, name, force)
	log.Printf("[docker] VolumeRemove: done err=%v", err)
	return err
}

func (s *volumeService) Prune(ctx context.Context, opts PruneOptions) (PruneReport, error) {
	log.Printf("[docker] VolumesPrune: all=%v", opts.All)
	f := filters.Args{}
	if opts.All {
		f = filters.NewArgs(filters.Arg("all", "true"))
	}
	r, err := s.cli.VolumesPrune(ctx, f)
	if err != nil {
		return PruneReport{}, err
	}
	log.Printf("[docker] VolumesPrune: deleted=%d spaceReclaimed=%d", len(r.VolumesDeleted), r.SpaceReclaimed)
	return PruneReport{ItemsDeleted: len(r.VolumesDeleted), SpaceReclaimed: r.SpaceReclaimed}, nil
}

const (
	logsSinceHours = 2 // hours of log history to fetch
	repoTagParts   = 2 // parts when splitting repo:tag on ":"
)

// Local Network Service.
type networkService struct {
	cli *client.Client
}

func (s *networkService) List(ctx context.Context) ([]Network, error) {
	log.Printf("[docker] NetworkList")
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
	log.Printf("[docker] NetworkList: returned count=%d", len(result))
	return result, nil
}

func (s *networkService) Remove(ctx context.Context, id string) error {
	log.Printf("[docker] NetworkRemove: id=%q", id)
	err := s.cli.NetworkRemove(ctx, id)
	log.Printf("[docker] NetworkRemove: done err=%v", err)
	return err
}

func (s *networkService) Prune(ctx context.Context, _ PruneOptions) (PruneReport, error) {
	log.Printf("[docker] NetworksPrune")
	r, err := s.cli.NetworksPrune(ctx, filters.Args{})
	if err != nil {
		return PruneReport{}, err
	}
	log.Printf("[docker] NetworksPrune: deleted=%d", len(r.NetworksDeleted))
	return PruneReport{ItemsDeleted: len(r.NetworksDeleted), SpaceReclaimed: 0}, nil
}

// timeFromUnix converts Unix timestamp to time.Time.
func timeFromUnix(unix int64) time.Time {
	return time.Unix(unix, 0)
}
