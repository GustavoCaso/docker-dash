package client

import (
	"context"
	"log"
	"sync"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
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
	resultMap := sync.Map{}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallelInspectLimit)

	for i, n := range networks {
		idx := i
		group.Go(func() error {
			inspectResponse, inspectErr := s.cli.NetworkInspect(groupCtx, n.ID, network.InspectOptions{})
			if inspectErr != nil {
				return inspectErr
			}

			subnet := ""
			gateway := ""
			if len(inspectResponse.IPAM.Config) > 0 {
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
			resultMap.Store(idx, Network{
				ID:                  n.ID,
				Name:                n.Name,
				Driver:              n.Driver,
				Scope:               n.Scope,
				Internal:            n.Internal,
				Created:             n.Created,
				ConnectedContainers: connected,
				IPAM:                NetworkIPAM{Subnet: subnet, Gateway: gateway},
			})
			return nil
		})
	}

	groupErr := group.Wait()
	if groupErr != nil {
		return nil, groupErr
	}

	resultMap.Range(func(key, value any) bool {
		idx, _ := key.(int)
		net, _ := value.(Network)
		result[idx] = net
		return true
	})

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
