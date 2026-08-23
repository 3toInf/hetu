package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/notify"
	"github.com/3toInf/hetu/internal/policy"
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

// ManagerOptions configures a SessionManager. Zero values select the defaults:
// Policy → a seeded-DefaultRules loader, Logger → slog.Default(), Notifier →
// notify.NewDesktop().
type ManagerOptions struct {
	Policy   *policy.Loader
	Logger   *slog.Logger
	Notifier notify.Notifier
}

type SessionManager struct {
	store    *store.Store
	resolve  *project.Resolver
	agents   map[string]agent.Agent
	notifier notify.Notifier
	policy   *policy.Loader
	logger   *slog.Logger

	mu   sync.Mutex
	live map[string]*liveSession // hetuID -> live
	wg   sync.WaitGroup           // tracks pump goroutines
}

func NewSessionManager(st *store.Store, r *project.Resolver, agents map[string]agent.Agent) *SessionManager {
	return NewSessionManagerOpts(st, r, agents, ManagerOptions{})
}

// NewSessionManagerOpts builds a manager with injectable policy loader, logger
// and notifier; any nil option falls back to its default.
func NewSessionManagerOpts(st *store.Store, r *project.Resolver, agents map[string]agent.Agent, o ManagerOptions) *SessionManager {
	if o.Policy == nil {
		o.Policy = defaultPolicyLoader()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Notifier == nil {
		o.Notifier = notify.NewDesktop()
	}
	return &SessionManager{
		store:    st,
		resolve:  r,
		agents:   agents,
		notifier: o.Notifier,
		policy:   o.Policy,
		logger:   o.Logger,
		live:     map[string]*liveSession{},
	}
}

// NewSessionManagerWithPolicy is NewSessionManager but with an explicit policy
// loader (used by main.go with the rules file; NewSessionManager seeds the
// v0.2-compatible default in-memory instead).
func NewSessionManagerWithPolicy(st *store.Store, r *project.Resolver, agents map[string]agent.Agent, pol *policy.Loader) *SessionManager {
	return NewSessionManagerOpts(st, r, agents, ManagerOptions{Policy: pol})
}

// defaultPolicyLoader seeds an in-memory loader with DefaultRules() so a
// manager constructed without a rules file keeps the v0.2 behavior (Read
// auto-allowed, everything else asks the human).
func defaultPolicyLoader() *policy.Loader {
	l := policy.NewLoader("") // no file ⇒ ask-everything
	p, _ := policy.NewPolicy(policy.DefaultRules())
	l.Seed(p)
	return l
}

// newSessionManagerWithNotifier is a test-only constructor that allows injecting a custom notifier
func newSessionManagerWithNotifier(st *store.Store, r *project.Resolver, agents map[string]agent.Agent, n notify.Notifier) *SessionManager {
	return NewSessionManagerOpts(st, r, agents, ManagerOptions{Notifier: n})
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
			if err := m.store.UpdateSessionStatus(ctx, hetuID, ev.Status, true); err != nil {
				m.logger.Warn("pump update status", "hetu_id", hetuID, "status", string(ev.Status), "err", err)
			}
			m.maybeNotify(hetuID, ev.Status, &last)
			last = ev.Status
		case agent.EventApproval:
			if err := m.store.AppendEvent(ctx, hetuID, ev.Seq, "approval", ev.ToolName+":"+ev.ApprovalState); err != nil {
				m.logger.Warn("pump append approval event", "hetu_id", hetuID, "err", err)
			}
			m.markUnread(ctx, hetuID)
		case agent.EventMeta:
			// Persist the learned external_id (claude mints its session id
			// only after start). Bookkeeping — not broadcast to subscribers.
			if ev.ExternalID != "" {
				if existing, ok, err := m.store.GetSession(ctx, hetuID); err == nil && ok {
					if err := m.store.UpdateExternalID(ctx, existing.Agent, hetuID, ev.ExternalID); err != nil {
						m.logger.Warn("pump update external id", "hetu_id", hetuID, "err", err)
					}
				}
			}
		default:
			m.markUnread(ctx, hetuID)
		}
	}
	// session closed: final status
	if err := m.store.UpdateSessionStatus(ctx, hetuID, l.sess.Status(), false); err != nil {
		m.logger.Warn("pump final status", "hetu_id", hetuID, "status", string(l.sess.Status()), "err", err)
	}
	m.mu.Lock()
	delete(m.live, hetuID)
	m.mu.Unlock()
	// Wake any lingering subscribers so `hetu watch` exits when the session
	// completes, instead of blocking on a channel that will never fill again.
	// Safe against a concurrent cancel→unsubscribe: a cancel after the delete
	// sees the session is gone and no-ops; a cancel already inside l.mu finds
	// the nil slice (no identity match, no close); both closers serialize under
	// l.mu so there is never a double-close.
	l.mu.Lock()
	for _, s := range l.subscribers {
		close(s)
	}
	l.subscribers = nil
	l.mu.Unlock()
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

// Subscribe registers a subscriber; the returned cancel removes and closes
// it. Cancel is idempotent and safe from any goroutine.
func (m *SessionManager) Subscribe(hetuID string) (<-chan agent.Event, func(), error) {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return nil, nil, errNotDriven
	}
	sub := make(chan agent.Event, 64)
	// l.subscribers is read by broadcast under l.mu — the append must take
	// the same lock (previously only m.mu was held: data race with pump).
	l.mu.Lock()
	l.subscribers = append(l.subscribers, sub)
	l.mu.Unlock()
	// The cancel closure captures the bidirectional channel: it is what
	// unsubscribe compares against l.subscribers entries, so closing the
	// receive-only view returned to the caller would be the wrong identity.
	var once sync.Once
	cancel := func() {
		once.Do(func() { m.unsubscribe(hetuID, sub) })
	}
	return sub, cancel, nil
}

// unsubscribe removes and closes a subscriber channel (identity match). It is
// a no-op if the session is gone or the channel is not registered — safe to
// call when the session ended between Subscribe and cancel. Unexported: only
// the sync.Once cancel closure may call it (a direct double call would panic
// on double-close).
func (m *SessionManager) unsubscribe(hetuID string, sub chan agent.Event) {
	m.mu.Lock()
	l, ok := m.live[hetuID]
	m.mu.Unlock()
	if !ok {
		return
	}
	l.mu.Lock()
	for i, s := range l.subscribers {
		if s == sub {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			close(s) // close only when found; double-close panics
			break
		}
	}
	l.mu.Unlock()
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

func (m *SessionManager) RequestPermission(ctx context.Context, hetuID, toolUseID, toolName, toolInput string) (ApprovalDecision, error) {
	kind, subject := policy.Extract("claude", toolName, toolInput)
	dec, rule, matched, err := m.policy.Decide(kind, subject)
	if err != nil {
		// Decision still made from last-good; surface the load/parse problem.
		m.logger.Warn("policy", "hetu_id", hetuID, "err", err)
	}
	ruleStr := ""
	if matched {
		ruleStr = rule.String()
	}
	m.logger.Info("hetu permission", "hetu_id", hetuID, "tool", toolName, "kind", string(kind),
		"subject", subject, "decision", decisionString(dec), "rule", ruleStr)
	switch dec {
	case policy.DecisionAllow:
		return ApprovalDecision{Allow: true}, nil
	case policy.DecisionDeny:
		return ApprovalDecision{Allow: false, Reason: "denied by rule " + rule.String()}, nil
	}
	return m.RequestApproval(ctx, hetuID, toolUseID, toolName, toolInput)
}

func decisionString(d policy.Decision) string {
	switch d {
	case policy.DecisionAllow:
		return "allow"
	case policy.DecisionDeny:
		return "deny"
	default:
		return "ask"
	}
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
		m.logger.Info("approval", "hetu_id", hetuID, "tool_use_id", toolUseID, "decision", "resolved", "allow", d.Allow)
		l.broadcast(agent.Event{Type: agent.EventApproval, ToolUseID: toolUseID, ApprovalState: state})
		m.setLiveStatus(l, hetuID, session.StatusRunning)
		return d, nil
	case <-ctx.Done():
		l.mu.Lock()
		delete(l.pending, toolUseID)
		l.mu.Unlock()
		m.logger.Info("approval", "hetu_id", hetuID, "tool_use_id", toolUseID, "decision", "timeout")
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
	if err := l.sess.SetStatus(st); err != nil {
		m.logger.Warn("set live status", "hetu_id", hetuID, "status", string(st), "err", err)
	}
	if err := m.store.UpdateSessionStatus(context.Background(), hetuID, st, true); err != nil {
		m.logger.Warn("update session status", "hetu_id", hetuID, "status", string(st), "err", err)
	}
	l.broadcast(agent.Event{Type: agent.EventStatus, Status: st})
}

// markUnread marks the session as unread
func (m *SessionManager) markUnread(ctx context.Context, hetuID string) {
	if err := m.store.MarkUnread(ctx, hetuID); err != nil {
		m.logger.Warn("mark unread", "hetu_id", hetuID, "err", err)
	}
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
		if err := m.notifier.Notify(notify.Notification{Title: "hetu", Body: body}); err != nil {
			m.logger.Warn("notify", "hetu_id", hetuID, "status", string(cur), "err", err)
		}
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
