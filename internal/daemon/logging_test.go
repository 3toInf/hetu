package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/notify"
	"github.com/3toInf/hetu/internal/policy"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/store"
)

// recorderHandler is a tiny slog.Handler test double that stores every record
// it receives (with attrs). It is not the stdlib slogtest package.
type recorderHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func newRecorder() *recorderHandler { return &recorderHandler{} }

func (r *recorderHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (r *recorderHandler) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec.Clone())
	return nil
}
func (r *recorderHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return r }
func (r *recorderHandler) WithGroup(name string) slog.Handler        { return r }

// Find returns the first record with the given message carrying attr key==val,
// or nil.
func (r *recorderHandler) Find(msg, key, val string) *foundRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.records {
		rec := &r.records[i]
		if rec.Message != msg {
			continue
		}
		var got string
		rec.Attrs(func(a slog.Attr) bool {
			if a.Key == key {
				got = attrString(a.Value)
				return false
			}
			return true
		})
		if got == val {
			return &foundRecord{rec: *rec}
		}
	}
	return nil
}

// foundRecord wraps a slog.Record for the Attr helper.
type foundRecord struct{ rec slog.Record }

// Attr returns the string value of the first attr with the given key ("" if
// the record has no such attr).
func (f *foundRecord) Attr(key string) string {
	var out string
	f.rec.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			out = attrString(a.Value)
			return false
		}
		return true
	})
	return out
}

func attrString(v slog.Value) string {
	if s, ok := v.Any().(string); ok {
		return s
	}
	return v.String()
}

// newDrivenManagerOpts builds a manager via NewSessionManagerOpts with an
// explicit policy loader and logger, plus a live session.
func newDrivenManagerOpts(t *testing.T, rulesPath string, logger *slog.Logger) (*SessionManager, string) {
	t.Helper()
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	fs := fake.NewSession("h1", "ext1", statusRunningForTest())
	fa := &fake.Agent{N: "claude", Drv: &fake.Driver{OnStart: func(_ context.Context, _ agent.StartRequest) (agent.Session, error) {
		return fs, nil
	}}}
	m := NewSessionManagerOpts(st, r, map[string]agent.Agent{"claude": fa}, ManagerOptions{
		Policy:   policy.NewLoader(rulesPath),
		Logger:   logger,
		Notifier: notify.NewNoop(),
	})
	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.LiveStatus(hid); !ok {
		t.Fatal("session not live after Ensure")
	}
	return m, hid
}

// TestRequestPermissionLogsDecisions asserts that a rule-allowed
// RequestPermission logs a single Info record naming the fired rule.
func TestRequestPermissionLogsDecisions(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	if err := policy.Save(rulesPath, policy.Rules{Allow: []string{"Shell(echo:*)"}}); err != nil {
		t.Fatal(err)
	}
	mem := newRecorder()
	mgr, hid := newDrivenManagerOpts(t, rulesPath, slog.New(mem))

	d, err := mgr.RequestPermission(context.Background(), hid, "t1", "Bash", `{"command":"echo hi"}`)
	if err != nil || !d.Allow {
		t.Fatalf("rule-allow: %+v %v", d, err)
	}

	rec := mem.Find("hetu permission", "decision", "allow")
	if rec == nil {
		t.Fatalf("expected allow decision record, got none")
	}
	if got := rec.Attr("rule"); got != "Shell(echo:*)" {
		t.Fatalf("rule = %q, want %q", got, "Shell(echo:*)")
	}
	if got := rec.Attr("hetu_id"); got != hid {
		t.Fatalf("hetu_id = %q, want %q", got, hid)
	}
}

// TestRequestPermissionLogsPolicyWarn asserts that a bad rules file surfaces a
// Warn record (the decision still degrades to ask).
func TestRequestPermissionLogsPolicyWarn(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(rulesPath, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	mem := newRecorder()
	mgr, hid := newDrivenManagerOpts(t, rulesPath, slog.New(mem))

	ctx := context.Background()
	go func() {
		mgr.RequestPermission(ctx, hid, "t1", "Bash", `{"command":"echo hi"}`)
	}()

	waitForTrue(t, func() bool { return mem.Find("policy", "hetu_id", hid) != nil }, "expected policy warn record")
	waitForApprovalPending(t, mgr, hid, "t1")
	mgr.ResolveApproval(hid, "t1", ApprovalDecision{Allow: true})
}
