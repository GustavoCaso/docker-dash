package service

import (
	"io"
	"time"

	"github.com/charmbracelet/lipgloss/tree"
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
}

type ContainerFileTree struct {
	Files []string
	Tree  *tree.Tree
}

type VolumeFileTree struct {
	Files []string
	Tree  *tree.Tree
}

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
	ID         string
	Repo       string
	Tag        string
	Size       int64
	Created    time.Time
	Dangling   bool
	Containers int
	UsedBy     []string // Container IDs using this image
	Layers     []Layer  // Image layers from history
	Config     *dockerspec.DockerOCIImageConfig
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
