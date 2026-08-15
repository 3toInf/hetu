package daemon

import (
	"context"
	"errors"
	"sync"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/notify"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
	"github.com/google/uuid"
)

type liveSession struct {
	sess        agent.Session
	subscribers []chan agent.Event
	pending     map[string]*pendingEntry
	mu          sync.Mutex
}

type pendingEntry struct {
	PendingApproval
	ch chan ApprovalDecision
}

type ApprovalDecision struct {
	Allow  bool
	Reason string
}

type PendingApproval struct {
	ToolUseID  string `json:"tool_use_id"`
	ToolName   string `json:"tool_name"`
	ToolInput  string `json:"tool_input"`
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
	store    *store.Store
	resolve  *project.Resolver
	agents   map[string]agent.Agent
	notifier notify.Notifier

	mu   sync.Mutex
	live map[string]*liveSession // hetuID -> live
	wg   sync.WaitGroup           // tracks pump goroutines
}

func NewSessionManager(st *store.Store, r *project.Resolver, agents map[string]agent.Agent) *SessionManager {
	return &SessionManager{
		store:    st,
		resolve:  r,
		agents:   agents,
		notifier: notify.NewDesktop(),
		live:     map[string]*liveSession{},
	}
}

// newSessionManagerWithNotifier is a test-only constructor that allows injecting a custom notifier
func newSessionManagerWithNotifier(st *store.Store, r *project.Resolver, agents map[string]agent.Agent, n notify.Notifier) *SessionManager {
	return &SessionManager{
		store:    st,
		resolve:  r,
		agents:   agents,
		notifier: n,
		live:     map[string]*liveSession{},
	}
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

	// Reuse the existing session's hetu_id (by agent+external_id) so resuming a
	// discovered session keeps the id users see in `hetu sessions`. Only mint a
	// new id for genuinely new sessions.
	hetuID := uuid.NewString()
	if externalID != "" {
		if existing, ok, _ := m.store.GetSessionByExternal(ctx, agentName, externalID); ok {
			hetuID = existing.HetuID
		}
	}
	proj, err := m.resolve.ResolveByCWD(ctx, cwd)
	if err != nil {
		return "", err
	}
	sess, err := a.Driver().Start(ctx, agent.StartRequest{Mode: agent.StartResume, ExternalID: externalID, CWD: cwd})
	if err != nil {
		return "", err
	}

	// Re-check under lock to prevent TOCTOU race - another caller may have registered this externalID while we were starting the session
	m.mu.Lock()
	for hid, l := range m.live {
		if l.sess.ExternalID() == externalID {
			m.mu.Unlock()
			// Close the duplicate session we just created and return the existing one
			_ = sess.Close()
			return hid, nil
		}
	}
	l := &liveSession{
		sess:    sess,
		pending: map[string]*pendingEntry{},
	}
	m.live[hetuID] = l
	m.mu.Unlock()

	// persist row
	_, _ = m.store.UpsertSession(ctx, store.Session{
		HetuID: hetuID, Agent: agentName, ExternalID: externalID, ProjectID: proj.ID,
		Host: "local", CWD: cwd, Status: session.StatusRunning, Driven: true,
	})
	// event pump
	m.wg.Add(1)
	go m.pump(ctx, hetuID, l)
	return hetuID, nil
}

func (m *SessionManager) pump(ctx context.Context, hetuID string, l *liveSession) {
	defer m.wg.Done()
	var last session.Status
	for ev := range l.sess.Events() {
		l.broadcast(ev)
		switch ev.Type {
		case agent.EventStatus:
			_ = m.store.UpdateSessionStatus(ctx, hetuID, ev.Status, true)
			m.maybeNotify(hetuID, ev.Status, &last)
			last = ev.Status
		case agent.EventApproval:
			_ = m.store.AppendEvent(ctx, hetuID, ev.Seq, "approval", ev.ToolName+":"+ev.ApprovalState)
			m.markUnread(ctx, hetuID)
		case agent.EventMeta:
			// Persist the learned external_id. Meta events are bookkeeping, not user-visible.
			if ev.ExternalID != "" {
				// Fetch existing session row to preserve other fields.
				existing, ok, err := m.store.GetSession(ctx, hetuID)
				if err == nil && ok {
					// Update external_id by deleting and re-inserting.
					// The UpsertSession conflict clause on (agent, external_id) makes
					// it hard to update external_id directly, so we delete and recreate.
					_ = m.store.DeleteSession(ctx, hetuID)
					_, _ = m.store.UpsertSession(ctx, store.Session{
						HetuID:     existing.HetuID,
						Agent:      existing.Agent,
						ExternalID: ev.ExternalID,
						ProjectID:  existing.ProjectID,
						Host:       existing.Host,
						CWD:        existing.CWD,
						Title:      existing.Title,
						Status:     existing.Status,
						Driven:     existing.Driven,
						Unread:     existing.Unread,
						CreatedAt:  existing.CreatedAt,
						UpdatedAt:  existing.UpdatedAt,
						LastEventAt: existing.LastEventAt,
						LastViewedAt: existing.LastViewedAt,
					})
				}
			}
			// Do not broadcast meta events to subscribers (internal bookkeeping).
		default:
			m.markUnread(ctx, hetuID)
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
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return nil, errNotDriven
	}
	sub := make(chan agent.Event, 64)
	// l.subscribers is read by broadcast under l.mu — the append must take
	// the same lock (previously only m.mu was held: data race with pump).
	l.mu.Lock()
	l.subscribers = append(l.subscribers, sub)
	l.mu.Unlock()
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

// LiveSession returns the underlying agent.Session for a live session (test-only)
func (m *SessionManager) LiveSession(hetuID string) (agent.Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.live[hetuID]
	if !ok {
		return nil, false
	}
	return l.sess, true
}

// readOnlyTools auto-allow without bothering the human. Unknown tools are
// NOT here → they ask the human (safe default). Extend in v0.3 (allowlist).
var readOnlyTools = map[string]bool{"Read": true, "Glob": true, "Grep": true, "LS": true}

func (m *SessionManager) RequestPermission(ctx context.Context, hetuID, toolUseID, toolName, toolInput string) (ApprovalDecision, error) {
	if readOnlyTools[toolName] {
		return ApprovalDecision{Allow: true}, nil
	}
	return m.RequestApproval(ctx, hetuID, toolUseID, toolName, toolInput)
}

func (m *SessionManager) RequestApproval(ctx context.Context, hetuID, toolUseID, toolName, toolInput string) (ApprovalDecision, error) {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return ApprovalDecision{}, errNotDriven
	}
	entry := &pendingEntry{
		PendingApproval: PendingApproval{ToolUseID: toolUseID, ToolName: toolName, ToolInput: toolInput},
		ch:              make(chan ApprovalDecision, 1),
	}
	l.mu.Lock()
	if l.pending == nil {
		l.pending = map[string]*pendingEntry{}
	}
	l.pending[toolUseID] = entry
	l.mu.Unlock()

	l.broadcast(agent.Event{Type: agent.EventApproval, ToolName: toolName, ToolUseID: toolUseID,
		ToolJSON: toolInput, ApprovalState: "pending"})
	m.setLiveStatus(l, hetuID, session.StatusWaitingForApproval)

	select {
	case d := <-entry.ch:
		state := "allowed"
		if !d.Allow {
			state = "denied"
		}
		l.broadcast(agent.Event{Type: agent.EventApproval, ToolUseID: toolUseID, ApprovalState: state})
		m.setLiveStatus(l, hetuID, session.StatusRunning)
		return d, nil
	case <-ctx.Done():
		l.mu.Lock()
		delete(l.pending, toolUseID)
		l.mu.Unlock()
		l.broadcast(agent.Event{Type: agent.EventApproval, ToolUseID: toolUseID, ApprovalState: "timeout"})
		m.setLiveStatus(l, hetuID, session.StatusRunning)
		return ApprovalDecision{Allow: false, Reason: "approval timed out"}, ctx.Err()
	}
}

func (m *SessionManager) ResolveApproval(hetuID, toolUseID string, d ApprovalDecision) bool {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return false
	}
	l.mu.Lock()
	entry, found := l.pending[toolUseID]
	if found {
		delete(l.pending, toolUseID)
	}
	l.mu.Unlock()
	if !found {
		return false
	}
	entry.ch <- d
	return true
}

func (m *SessionManager) PendingApprovals(hetuID string) []PendingApproval {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]PendingApproval, 0, len(l.pending))
	for _, e := range l.pending {
		out = append(out, e.PendingApproval)
	}
	return out
}

// setLiveStatus is a helper that updates the session status in-memory, in the store, and via broadcast
func (m *SessionManager) setLiveStatus(l *liveSession, hetuID string, st session.Status) {
	_ = l.sess.SetStatus(st)
	_ = m.store.UpdateSessionStatus(context.Background(), hetuID, st, true)
	l.broadcast(agent.Event{Type: agent.EventStatus, Status: st})
}

// markUnread marks the session as unread
func (m *SessionManager) markUnread(ctx context.Context, hetuID string) {
	_ = m.store.MarkUnread(ctx, hetuID)
}

// maybeNotify sends a desktop notification when the session status transitions to a waiting/error/completed state
func (m *SessionManager) maybeNotify(hetuID string, cur session.Status, last *session.Status) {
	// No transition if current equals last (and last is set)
	if last != nil && cur == *last {
		return
	}

	switch cur {
	case session.StatusWaitingForApproval, session.StatusWaitingForInput, session.StatusCompleted, session.StatusError:
		se, ok, _ := m.store.GetSession(context.Background(), hetuID)
		body := string(cur)
		if ok && se.Title != "" {
			body = se.Title + " · " + string(cur)
		}
		_ = m.notifier.Notify(notify.Notification{Title: "hetu", Body: body})
	}
}

func (m *SessionManager) Close() error {
	m.mu.Lock()
	// Snapshot live sessions and clear the map under lock
	liveSessions := make([]*liveSession, 0, len(m.live))
	for _, l := range m.live {
		liveSessions = append(liveSessions, l)
	}
	m.live = map[string]*liveSession{}
	m.mu.Unlock()

	// Close each session (no longer holding manager mutex)
	for _, l := range liveSessions {
		_ = l.sess.Close()
	}

	// Wait for all pump goroutines to finish
	m.wg.Wait()
	return nil
}

var (
	errUnknownAgent = errors.New("unknown agent")
	errNotDriven    = errors.New("session is not currently driven")
)
