package fake

import (
	"context"
	"sync"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

// Session is a controllable in-memory agent.Session.
type Session struct {
	id         string
	externalID string
	status     session.Status
	events     chan agent.Event
	sendLog    []string
	mu         sync.Mutex
	closed     bool
}

func NewSession(id, ext string, status session.Status) *Session {
	return &Session{id: id, externalID: ext, status: status, events: make(chan agent.Event, 16)}
}

func (s *Session) ID() string         { return s.id }
func (s *Session) ExternalID() string { return s.externalID }
func (s *Session) Status() session.Status {
	s.mu.Lock(); defer s.mu.Unlock()
	return s.status
}
func (s *Session) SetStatus(st session.Status) error { s.mu.Lock(); s.status = st; s.mu.Unlock(); return nil }
func (s *Session) Events() <-chan agent.Event   { return s.events }
func (s *Session) Emit(e agent.Event)           { s.events <- e }
func (s *Session) Send(_ context.Context, p string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	s.sendLog = append(s.sendLog, p)
	return nil
}
func (s *Session) Sends() []string { s.mu.Lock(); defer s.mu.Unlock(); return append([]string(nil), s.sendLog...) }
func (s *Session) Close() error    { s.mu.Lock(); defer s.mu.Unlock(); if !s.closed { close(s.events); s.closed = true }; return nil }

// Driver implements agent.Driver for tests.
type Driver struct {
	OnStart   func(ctx context.Context, req agent.StartRequest) (agent.Session, error)
	StatusOf_ func(ctx context.Context, ext string) (session.Status, error)
}

func (d *Driver) Start(ctx context.Context, req agent.StartRequest) (agent.Session, error) {
	if d.OnStart != nil {
		return d.OnStart(ctx, req)
	}
	return nil, nil
}
func (d *Driver) StatusOf(ctx context.Context, ext string) (session.Status, error) {
	if d.StatusOf_ != nil {
		return d.StatusOf_(ctx, ext)
	}
	return session.StatusUnknown, nil
}

// Agent implements agent.Agent for tests.
type Agent struct {
	N     string
	Drv   *Driver
	Disc  agent.DiscoverySource
}

func (a *Agent) Name() string                  { return a.N }
func (a *Agent) Driver() agent.Driver          { return a.Drv }
func (a *Agent) DiscoverySource() agent.DiscoverySource { return a.Disc }

// DiscoverySource yields a fixed list of sessions.
type DiscoverySource struct {
	Sessions []agent.DiscoveredSession
	Err      error
}

func (d *DiscoverySource) Discover(ctx context.Context, _ agent.DiscoverOpts) (<-chan agent.DiscoveredSession, error) {
	ch := make(chan agent.DiscoveredSession, len(d.Sessions))
	for _, s := range d.Sessions {
		ch <- s
	}
	close(ch)
	return ch, d.Err
}

// keep time referenced to avoid unused import if extended later
var _ = time.Now
