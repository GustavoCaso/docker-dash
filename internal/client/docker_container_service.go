package client

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// Local Container Service.
type containerService struct {
	cli *client.Client
}

func (s *containerService) List(ctx context.Context) ([]Container, error) {
	log.Printf("[docker] ContainerList: all=true")
	containers, listErr := s.cli.ContainerList(ctx, container.ListOptions{All: true})
	if listErr != nil {
		return nil, listErr
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

		ci, err := s.cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			return nil, err
		}

		health := buildContainerHealth(ci.State.Health)

		result[i] = Container{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   state,
			Health:  health,
			Created: timeFromUnix(c.Created),
			Ports:   ports,
			Mounts:  mounts,
		}
	}

	log.Printf("[docker] ContainerList: returned count=%d err=%v", len(result), listErr)
	return result, nil
}

func (s *containerService) Run(ctx context.Context, img Image, opts RunOptions) (string, error) {
	log.Printf("[docker] ContainerCreate+Start: image=%q name=%q", img.Name(), opts.Name)

	ports := nat.PortSet{}

	if img.Config != nil {
		for port := range img.Config.ExposedPorts {
			natPort, err := nat.NewPort(nat.SplitProtoPort(port))
			if err != nil {
				return "", err
			}
			ports[natPort] = struct{}{}
		}
	}

	// Parse port bindings from opts.Ports (format: "hostPort:containerPort").
	portBindings := nat.PortMap{}
	for _, p := range opts.Ports {
		parts := strings.SplitN(p, ":", 2) //nolint:mnd // splitting "hostPort:containerPort"
		if len(parts) != 2 {               //nolint:mnd // need exactly host:container
			continue
		}
		natPort, err := nat.NewPort("tcp", parts[1])
		if err != nil {
			return "", fmt.Errorf("invalid port mapping %q: %w", p, err)
		}
		ports[natPort] = struct{}{}
		portBindings[natPort] = append(portBindings[natPort], nat.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: parts[0],
		})
	}

	// Merge image env with user-supplied env (user values take precedence).
	var baseEnv []string
	if img.Config != nil {
		baseEnv = img.Config.Env
	}
	env := make([]string, 0, len(baseEnv)+len(opts.Env))
	env = append(env, baseEnv...)
	env = append(env, opts.Env...)

	var config *container.Config
	if img.Config != nil {
		config = &container.Config{
			User:         img.Config.User,
			WorkingDir:   img.Config.WorkingDir,
			Labels:       img.Config.Labels,
			Env:          env,
			Cmd:          img.Config.Cmd,
			Entrypoint:   img.Config.Entrypoint,
			Image:        img.Name(),
			Shell:        img.Config.Shell,
			OnBuild:      img.Config.OnBuild,
			Volumes:      img.Config.Volumes,
			ExposedPorts: ports,
			Healthcheck:  img.Config.Healthcheck,
		}
	} else {
		config = &container.Config{
			Env:          env,
			Image:        img.Name(),
			ExposedPorts: ports,
		}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
	}

	containerResponse, err := s.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, opts.Name)
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

	health := buildContainerHealth(c.State.Health)

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
		Health:     health,
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

func (s *containerService) Pause(ctx context.Context, id string) error {
	log.Printf("[docker] ContainerPause: id=%q", id)
	err := s.cli.ContainerPause(ctx, id)
	log.Printf("[docker] ContainerPause: done err=%v", err)
	return err
}

func (s *containerService) Unpause(ctx context.Context, id string) error {
	log.Printf("[docker] ContainerUnpause: id=%q", id)
	err := s.cli.ContainerUnpause(ctx, id)
	log.Printf("[docker] ContainerUnpause: done err=%v", err)
	return err
}

const logsSinceHours = 2 // hours of log history to fetch

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

func buildContainerHealth(h *container.Health) *HealthInfo {
	if h == nil {
		return &HealthInfo{
			Status: HealthNone,
		}
	}

	healthStatus := HealthNone
	switch h.Status {
	case container.Healthy:
		healthStatus = HealthHealthy
	case container.Unhealthy:
		healthStatus = HealthUnhealthy
	case container.Starting:
		healthStatus = HealthStarting
	}

	var (
		lastCheckTime time.Time
		lastOutput    string
	)

	if len(h.Log) > 0 {
		last := h.Log[len(h.Log)-1]
		lastCheckTime = last.End
		lastOutput = last.Output
	}

	return &HealthInfo{
		Status:        healthStatus,
		FailingStreak: h.FailingStreak,
		LastCheck:     lastCheckTime,
		Output:        lastOutput,
	}
}
