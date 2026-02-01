package service

import "time"

// Container represents a Docker container
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

// Image represents a Docker image
type Image struct {
	ID       string
	Repo     string
	Tag      string
	Size     int64
	Created  time.Time
	Dangling bool
	UsedBy   []string // Container IDs using this image
}

// Volume represents a Docker volume
type Volume struct {
	Name      string
	Driver    string
	MountPath string
	Size      int64
	Created   time.Time
	UsedBy    []string // Container IDs using this volume
}

// FileEntry represents a file in a volume
type FileEntry struct {
	Name  string
	IsDir bool
	Size  int64
	Mode  string
}

// LogOptions configures log streaming
type LogOptions struct {
	Follow     bool
	Tail       string
	Timestamps bool
}
