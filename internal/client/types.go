package client

import (
	"io"
	"io/fs"
	"strings"
	"time"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
)

// Container represents a Docker container.
type Container struct {
	ID      string
	Name    string
	Image   string
	Status  string
	State   ContainerState
	Created time.Time
	Ports   []PortMapping
	Mounts  []Mount

	// Networking
	Hostname    string
	NetworkMode string
	Networks    []NetworkInfo

	// Runtime config
	Cmd        []string
	Entrypoint []string
	WorkingDir string
	Env        []string
	Labels     map[string]string
	Health     *HealthInfo

	// Resource limits
	MemoryLimit   int64  // bytes; 0 = unlimited
	CPUShares     int64  // relative weight; 0 = default (1024)
	RestartPolicy string // e.g. "no", "always", "unless-stopped", "on-failure:3"
	Privileged    bool
}

type FileNode struct {
	Name      string
	Path      string
	IsDir     bool
	Collapsed bool
	Linkname  string
	Size      int64
	Mode      fs.FileMode
	Children  []*FileNode
	Parent    *FileNode
	Depth     int
}

type HealthInfo struct {
	Status        HealthStatus
	FailingStreak int
	LastCheck     time.Time
	Output        string
}

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStarting  HealthStatus = "starting"
	HealthNone      HealthStatus = "none"
)

type ContainerState string

const (
	StateRunning    ContainerState = "running"
	StateStopped    ContainerState = "stopped"
	StatePaused     ContainerState = "paused"
	StateRestarting ContainerState = "restarting"
)

type PortMapping struct {
	HostPort      uint16
	ContainerPort uint16
	Protocol      string
}

type Mount struct {
	Type        string // "volume", "bind", "tmpfs"
	Source      string
	Destination string
}

// NetworkInfo holds a container's attachment to a single Docker network.
type NetworkInfo struct {
	Name      string
	IPAddress string
	Gateway   string
	Aliases   []string
}

// Layer represents a Docker image layer.
type Layer struct {
	ID      string    // Layer digest/ID
	Command string    // Dockerfile instruction that created this layer
	Size    int64     // Layer size in bytes
	Created time.Time // When the layer was created
}

const none = "<none>"

// Image represents a Docker image.
type Image struct {
	ID          string
	Repo        string
	Tag         string
	Size        int64
	Created     time.Time
	Dangling    bool
	Containers  int64
	UsedBy      []string // Container IDs using this image
	Config      *dockerspec.DockerOCIImageConfig
	RepoDigests []string // e.g. ["nginx@sha256:abc123..."]
}

func (i Image) Name() string {
	if i.Repo != none && i.Tag != none {
		return i.Repo + ":" + i.Tag
	}

	return i.ID
}

// Volume represents a Docker volume.
type Volume struct {
	Name      string
	Driver    string
	MountPath string
	Size      int64
	Created   time.Time
	UsedCount int
}

// NetworkIPAM holds IPAM configuration for a network.
type NetworkIPAM struct {
	Subnet  string
	Gateway string
}

// NetworkContainer represents a container endpoint connected to a network.
type NetworkContainer struct {
	Name        string
	IPv4Address string
	IPv6Address string
	MacAddress  string
}

// Network represents a Docker network.
type Network struct {
	ID                  string
	Name                string
	Driver              string
	Scope               string
	Internal            bool
	Created             time.Time
	ConnectedContainers []NetworkContainer
	IPAM                NetworkIPAM
}

// ComposeProject represents a Docker Compose project detected from container labels.
type ComposeProject struct {
	Name             string
	WorkingDir       string
	ConfigFiles      string
	EnvironmentFiles string
	Services         []ComposeServiceInfo
}

// ComposeServiceInfo holds information about a single service within a Compose project.
type ComposeServiceInfo struct {
	Name  string
	State string
	Image string
}

func (p ComposeProject) Identity() string {
	return strings.Join([]string{p.Name, p.WorkingDir, p.ConfigFiles, p.EnvironmentFiles}, "\x00")
}

// PruneReport summarises the result of a prune operation.
type PruneReport struct {
	ItemsDeleted   int
	SpaceReclaimed uint64 // bytes; 0 for networks
}

// PruneOptions controls what gets removed during a prune operation.
// All=true for images includes non-dangling unused images (dangling=false filter).
// All=true for volumes includes named unused volumes in addition to anonymous ones.
type PruneOptions struct {
	All bool
}

// LogOptions configures log streaming.
type LogOptions struct {
	Follow     bool
	Tail       string
	Timestamps bool
}

type LogsSession struct {
	Reader io.ReadCloser
	closer func()
}

func NewLogsSession(reader io.ReadCloser, closer func()) *LogsSession {
	return &LogsSession{Reader: reader, closer: closer}
}

func (e *LogsSession) Close() {
	if e.closer != nil {
		e.closer()
	}
}

// ExecSession represents an interactive exec session inside a container.
type ExecSession struct {
	Reader io.ReadCloser
	Writer io.WriteCloser
	closer func()
}

func NewExecSession(reader io.ReadCloser, writer io.WriteCloser, closer func()) *ExecSession {
	return &ExecSession{Reader: reader, Writer: writer, closer: closer}
}

func (e *ExecSession) Close() {
	if e.closer != nil {
		e.closer()
	}
}

// StatsSession represents an interactive exec session inside a container.
type StatsSession struct {
	Reader io.ReadCloser
	closer func()
}

func NewStatsSession(reader io.ReadCloser, closer func()) *StatsSession {
	return &StatsSession{Reader: reader, closer: closer}
}

func (e *StatsSession) Close() {
	if e.closer != nil {
		e.closer()
	}
}
