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

			// Stable id: reuse the existing row's hetu_id; mint only for brand-new
			// sessions. On a lookup error skip the session rather than upsert with a
			// fresh uuid (ON CONFLICT would clobber the stable id users rely on).
			existing, ok, err := s.store.GetSessionByExternal(ctx, name, d.ExternalID)
			if err != nil {
				continue
			}
			var hetuID string
			if ok {
				hetuID = existing.HetuID
			} else {
				hetuID = uuid.NewString()
			}

			_, _ = s.store.UpsertSession(ctx, store.Session{
				HetuID: hetuID, Agent: name, ExternalID: d.ExternalID,
				ProjectID: proj.ID, Host: "local", CWD: d.CWD, Title: d.Title,
				Status: d.Status, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
			})

			// Sync messages only when the transcript file is newer than the last sync.
			// Compare in the same precision SetContentSynced stores (whole seconds):
			// a full-precision After against the truncated marker would re-sync every
			// discover because real filesystem mtimes carry sub-second components.
			syncedAt, synced, _ := s.store.ContentSyncedAt(ctx, hetuID)
			if !d.TranscriptMtime.IsZero() && (!synced || d.TranscriptMtime.Unix() > syncedAt) {
				if len(d.Messages) > 0 {
					msgs := make([]store.Message, 0, len(d.Messages))
					for _, dm := range d.Messages {
						msgs = append(msgs, store.Message{Seq: dm.Seq, Role: dm.Role, Content: dm.Content, TS: dm.TS})
					}
					if err := s.store.SyncMessages(ctx, hetuID, msgs); err == nil {
						_ = s.store.SetContentSynced(ctx, hetuID, d.TranscriptMtime.Unix())
					}
				}
			}
		}
	}
	return nil
}
