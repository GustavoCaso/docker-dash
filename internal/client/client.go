package client

import (
	"context"
)

// Client provides the interface.
type Client interface {
	Containers() ContainerService
	Images() ImageService
	Volumes() VolumeService
	Networks() NetworkService
	Ping(ctx context.Context) error
	Close() error
}

// ContainerService manages Docker containers.
type ContainerService interface {
	List(ctx context.Context) ([]Container, error)
	Run(ctx context.Context, image Image) (string, error)
	Get(ctx context.Context, id string) (*Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Remove(ctx context.Context, id string, force bool) error
	FileTree(ctx context.Context, id string) (ContainerFileTree, error)
	Logs(ctx context.Context, id string, opts LogOptions) (*LogsSession, error)
	Exec(ctx context.Context, id string) (*ExecSession, error)
	Stats(ctx context.Context, is string) (*StatsSession, error)
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}

// ImageService manages Docker images.
type ImageService interface {
	List(ctx context.Context) ([]Image, error)
	Get(ctx context.Context, id string) (Image, error)
	Remove(ctx context.Context, id string, force bool) error
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}

// VolumeService manages Docker volumes.
type VolumeService interface {
	List(ctx context.Context) ([]Volume, error)
	Remove(ctx context.Context, name string, force bool) error
	FileTree(ctx context.Context, name string) (VolumeFileTree, error)
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}

// NetworkService manages Docker networks.
type NetworkService interface {
	List(ctx context.Context) ([]Network, error)
	Remove(ctx context.Context, id string) error
	Prune(ctx context.Context, opts PruneOptions) (PruneReport, error)
}
