package client

import (
	"context"
	"log"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Local Volume Service.
type volumeService struct {
	cli *client.Client
}

func (s *volumeService) List(ctx context.Context) ([]Volume, error) {
	log.Printf("[docker] VolumeList")
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
