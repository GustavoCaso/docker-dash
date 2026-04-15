package client

import (
	"context"
	"time"
)

// Client provides the interface.
type Client interface {
	Containers() ContainerService
	Images() ImageService
	Volumes() VolumeService
	Networks() NetworkService
	Compose() ComposeProjectService
	Ping(ctx context.Context) error
	Close() error
}

// ComposeUpOptions configures how a Compose project is brought up.
type ComposeUpOptions struct {
	Build         bool
	RemoveOrphans bool
	Wait          bool
}

// ComposeDownOptions configures how a Compose project is brought down.
type ComposeDownOptions struct {
	RemoveOrphans bool
	Volumes       bool
	Images        string
}

// ComposeStartOptions configures how a Compose project is started.
type ComposeStartOptions struct{}

// ComposeStopOptions configures how a Compose project is stopped.
type ComposeStopOptions struct {
	Timeout *time.Duration
}

// ComposeRestartOptions configures how a Compose project is restarted.
type ComposeRestartOptions struct {
	Timeout *time.Duration
	NoDeps  bool
}

// ComposeProjectService manages Docker Compose projects detected from running containers.
type ComposeProjectService interface {
	List(ctx context.Context) ([]ComposeProject, error)
	Up(ctx context.Context, project ComposeProject, opts ComposeUpOptions) error
	Down(ctx context.Context, project ComposeProject, opts ComposeDownOptions) error
	Start(ctx context.Context, project ComposeProject, opts ComposeStartOptions) error
	Stop(ctx context.Context, project ComposeProject, opts ComposeStopOptions) error
	Restart(ctx context.Context, project ComposeProject, opts ComposeRestartOptions) error
}

// RunOptions configures how a container is created from an image.
type RunOptions struct {
	Name  string
	Ports []string // e.g. ["8080:80", "443:443"]
	Env   []string // e.g. ["KEY=VAL", "FOO=BAR"]
}

// ContainerService manages Docker containers.
type ContainerService interface {
	List(ctx context.Context) ([]Container, error)
	Run(ctx context.Context, image Image, opts RunOptions) (string, error)
	Get(ctx context.Context, id string) (*Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Remove(ctx context.Context, id string, force bool) error
	FileTree(ctx context.Context, id string) (*FileNode, error)
	Logs(ctx context.Context, id string, opts LogOptions) (*LogsSession, error)
	Exec(ctx context.Context, id string) (*ExecSession, error)
	Stats(ctx context.Context, is string) (*StatsSession, error)
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
	Pause(ctx context.Context, id string) error
	Unpause(ctx context.Context, id string) error
}

// ImageService manages Docker images.
type ImageService interface {
	List(ctx context.Context) ([]Image, error)
	Pull(ctx context.Context, image string, platform string) error
	FetchLayers(ctx context.Context, id string) []Layer
	Remove(ctx context.Context, id string, force bool) error
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}

// VolumeService manages Docker volumes.
type VolumeService interface {
	List(ctx context.Context) ([]Volume, error)
	Remove(ctx context.Context, name string, force bool) error
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}

// NetworkService manages Docker networks.
type NetworkService interface {
	List(ctx context.Context) ([]Network, error)
	Remove(ctx context.Context, id string) error
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}
