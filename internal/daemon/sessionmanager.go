package daemon

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
	"github.com/google/uuid"
)

type liveSession struct {
	sess        agent.Session
	subscribers []chan agent.Event
	mu          sync.Mutex
}

func (l *liveSession) broadcast(ev agent.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, sub := range l.subscribers {
		select {
		case sub <- ev:
		default:
		}
	}
}

type SessionManager struct {
	store   *store.Store
	resolve *project.Resolver
	agents  map[string]agent.Agent

	mu   sync.Mutex
	live map[string]*liveSession // hetuID -> live
}

func NewSessionManager(st *store.Store, r *project.Resolver, agents map[string]agent.Agent) *SessionManager {
	return &SessionManager{store: st, resolve: r, agents: agents, live: map[string]*liveSession{}}
}

// Ensure makes the session driven (resume-or-new) and returns its Hetu id.
func (m *SessionManager) Ensure(ctx context.Context, agentName, externalID, cwd string) (string, error) {
	a, ok := m.agents[agentName]
	if !ok {
		return "", errUnknownAgent
	}
	m.mu.Lock()
	// already live?
	for hid, l := range m.live {
		if l.sess.ExternalID() == externalID {
			m.mu.Unlock()
			return hid, nil
		}
	}
	m.mu.Unlock()

	hetuID := uuid.NewString()
	proj, _ := m.resolve.ResolveByCWD(ctx, cwd)
	sess, err := a.Driver().Start(ctx, agent.StartRequest{Mode: agent.StartResume, ExternalID: externalID, CWD: cwd})
	if err != nil {
		return "", err
	}
	l := &liveSession{sess: sess}
	m.mu.Lock()
	m.live[hetuID] = l
	m.mu.Unlock()

	// persist row
	_, _ = m.store.UpsertSession(ctx, store.Session{
		HetuID: hetuID, Agent: agentName, ExternalID: externalID, ProjectID: proj.ID,
		Host: "local", CWD: cwd, Status: session.StatusRunning, Driven: true,
	})
	// event pump
	go m.pump(ctx, hetuID, l)
	return hetuID, nil
}

func (m *SessionManager) pump(ctx context.Context, hetuID string, l *liveSession) {
	for ev := range l.sess.Events() {
		l.broadcast(ev)
		if ev.Type == agent.EventStatus {
			_ = m.store.UpdateSessionStatus(ctx, hetuID, ev.Status, true)
			_ = m.store.AppendEvent(ctx, hetuID, ev.Seq, string(ev.Type), ev.Text+ev.ToolName)
		}
	}
	// session closed: final status
	_ = m.store.UpdateSessionStatus(ctx, hetuID, l.sess.Status(), false)
	m.mu.Lock()
	delete(m.live, hetuID)
	m.mu.Unlock()
}

func (m *SessionManager) Send(ctx context.Context, hetuID, prompt string) error {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return errNotDriven
	}
	return l.sess.Send(ctx, prompt)
}

func (m *SessionManager) Subscribe(hetuID string) (<-chan agent.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.live[hetuID]
	if !ok {
		return nil, errNotDriven
	}
	sub := make(chan agent.Event, 64)
	l.subscribers = append(l.subscribers, sub)
	return sub, nil
}

func (m *SessionManager) LiveStatus(hetuID string) (session.Status, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.live[hetuID]
	if !ok {
		return session.StatusUnknown, false
	}
	return l.sess.Status(), true
}

func (m *SessionManager) Close() error {
	m.mu.Lock()
	ids := make([]string, 0, len(m.live))
	for id, l := range m.live {
		ids = append(ids, id)
		_ = l.sess.Close()
	}
	m.live = map[string]*liveSession{}
	m.mu.Unlock()
	// give pumps a moment to flush
	time.Sleep(20 * time.Millisecond)
	return nil
}

var (
	errUnknownAgent = errors.New("unknown agent")
	errNotDriven    = errors.New("session is not currently driven")
)
