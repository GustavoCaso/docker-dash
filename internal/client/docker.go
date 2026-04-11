package client

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

// dockerClient connects to a local or remote Docker daemon.
type dockerClient struct {
	cli        *client.Client
	containers *containerService
	images     *imageService
	volumes    *volumeService
	networks   *networkService
	compose    *composeProjectService
}

// NewDockerClientFromConfig creates a dockerClient using settings from cfg.
//
// Connection logic:
//   - cfg.Host empty → client.FromEnv (reads DOCKER_HOST, etc. from environment)
//   - cfg.Host is ssh:// using docker GetConnectionHelper
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
	c.compose = &composeProjectService{cli: cli}
	return c, nil
}

// isSSHHost reports whether host is an ssh:// URL.
func isSSHHost(host string) bool {
	return strings.HasPrefix(host, "ssh://")
}

func (c *dockerClient) Containers() ContainerService        { return c.containers }
func (c *dockerClient) Images() ImageService                { return c.images }
func (c *dockerClient) Volumes() VolumeService              { return c.volumes }
func (c *dockerClient) Networks() NetworkService            { return c.networks }
func (c *dockerClient) Compose() ComposeProjectService      { return c.compose }

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

// timeFromUnix converts Unix timestamp to time.Time.
func timeFromUnix(unix int64) time.Time {
	return time.Unix(unix, 0)
}
