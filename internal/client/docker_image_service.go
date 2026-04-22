package client

import (
	"context"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

// Local Image Service.
type imageService struct {
	cli *client.Client
}

func (s *imageService) List(ctx context.Context) ([]Image, error) {
	log.Printf("[docker] ImageList")
	du, err := s.cli.DiskUsage(ctx, dockertypes.DiskUsageOptions{
		Types: []dockertypes.DiskUsageObject{dockertypes.ImageObject},
	})
	if err != nil {
		return nil, err
	}

	images := du.Images

	result := make([]Image, len(images))
	resultMap := sync.Map{}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallelInspectLimit)

	for i, img := range images {
		idx := i
		group.Go(func() error {
			imageData, imageErr := s.get(groupCtx, img.ID)
			if imageErr != nil {
				return imageErr
			}

			imageData.Containers = img.Containers
			resultMap.Store(idx, imageData)
			return nil
		})
	}

	groupErr := group.Wait()
	if groupErr != nil {
		return []Image{}, groupErr
	}

	resultMap.Range(func(key, value any) bool {
		idx, _ := key.(int)
		img, _ := value.(Image)
		result[idx] = img
		return true
	})

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

const repoTagParts = 2 // parts when splitting repo:tag on ":"

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
		ID:          img.ID,
		Repo:        repo,
		Tag:         tag,
		Size:        img.Size,
		Created:     created,
		Dangling:    len(img.RepoTags) == 0 || repo == none && tag == none,
		Config:      img.Config,
		RepoDigests: img.RepoDigests,
	}, nil
}

// CheckUpdate queries the remote registry to determine if a newer image is available.
// It returns true if the remote digest does not match any local RepoDigest entry.
func (s *imageService) CheckUpdate(ctx context.Context, img Image) (bool, error) {
	if img.Dangling || img.Repo == none || len(img.RepoDigests) == 0 {
		return false, nil
	}

	distribution, err := s.cli.DistributionInspect(ctx, img.Repo+":"+img.Tag, "")
	if err != nil {
		return false, err
	}

	remoteDigest := distribution.Descriptor.Digest.String()
	for _, repoDigest := range img.RepoDigests {
		// repoDigest format: "repo@sha256:..."
		atIdx := strings.Index(repoDigest, "@")
		if atIdx < 0 {
			continue
		}
		localDigest := repoDigest[atIdx+1:]
		if localDigest == remoteDigest {
			return false, nil
		}
	}

	return true, nil
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

func (s *imageService) Pull(ctx context.Context, imageRef, platform string) error {
	body, err := s.cli.ImagePull(ctx, imageRef, image.PullOptions{
		Platform: platform,
	})

	if err != nil {
		return err
	}

	if _, copyErr := io.Copy(io.Discard, body); copyErr != nil {
		return copyErr
	}

	return body.Close()
}
