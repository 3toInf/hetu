package daemon

import (
	"context"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/store"
	"github.com/google/uuid"
)

type DiscoveryScheduler struct {
	store   *store.Store
	resolve *project.Resolver
	sources map[string]agent.DiscoverySource
}

func NewDiscoveryScheduler(st *store.Store, r *project.Resolver, sources map[string]agent.DiscoverySource) *DiscoveryScheduler {
	return &DiscoveryScheduler{store: st, resolve: r, sources: sources}
}

func (s *DiscoveryScheduler) Run(ctx context.Context) error {
	for name, src := range s.sources {
		ch, err := src.Discover(ctx, agent.DiscoverOpts{})
		if err != nil {
			continue
		}
		for d := range ch {
			proj, _ := s.resolve.ResolveByCWD(ctx, d.CWD)
			_, _ = s.store.UpsertSession(ctx, store.Session{
				HetuID:      uuid.NewString(),
				Agent:       name,
				ExternalID:  d.ExternalID,
				ProjectID:   proj.ID,
				Host:        "local",
				CWD:         d.CWD,
				Title:       d.Title,
				Status:      d.Status,
				CreatedAt:   d.CreatedAt,
				UpdatedAt:   d.UpdatedAt,
			})
		}
	}
	return nil
}
