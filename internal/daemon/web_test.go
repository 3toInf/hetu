package daemon

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

// getJSON GETs url and returns the body, failing the test on any non-2xx status.
func getJSON(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s: read body: %v", url, err)
	}
	return string(b)
}

// TestWebAPIEndpoints covers the read-only HTTP JSON API: each endpoint reuses
// Server.dispatch, so a request that works over the unix socket must work here.
func TestWebAPIEndpoints(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertProject(ctx, "Alpha", "/x")
	se, _ := st.UpsertSession(ctx, store.Session{HetuID: "s1", Agent: "claude", ExternalID: "e1", CWD: "/x", Title: "T1", Status: session.StatusCompleted})
	_ = st.SyncMessages(ctx, se.HetuID, []store.Message{{Seq: 0, Role: "user", Content: "websocket drops", TS: 0}})
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	// GET /api/projects
	proj := getJSON(t, ts.URL+"/api/projects")
	if !strings.Contains(proj, "Alpha") {
		t.Fatalf("projects missing: %s", proj)
	}

	// GET /api/sessions
	sess := getJSON(t, ts.URL+"/api/sessions")
	if !strings.Contains(sess, "s1") {
		t.Fatalf("sessions missing: %s", sess)
	}

	// GET /api/sessions/{id} — includes messages
	det := getJSON(t, ts.URL+"/api/sessions/s1")
	if !strings.Contains(det, "websocket drops") || !strings.Contains(det, `"role":"user"`) {
		t.Fatalf("detail missing messages: %s", det)
	}

	// GET /api/search — FTS hit
	hit := getJSON(t, ts.URL+"/api/search?q=websocket")
	if !strings.Contains(hit, "s1") || !strings.Contains(hit, `"hits":1`) {
		t.Fatalf("search wrong: %s", hit)
	}

	// unknown /api path → 404
	resp, _ := http.Get(ts.URL + "/api/nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// missing session id → 404 (ResolveSession miss mapped via "not found" sentinel)
	resp, _ = http.Get(ts.URL + "/api/sessions/missing")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing session, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `"error"`) {
		t.Fatalf("expected JSON error body, got %q", b)
	}
}

// TestWebStaticServe covers the frontend serving: the embedded dist by default, a
// disk dir when webDir is set, SPA fallback for unknown non-API paths, and a hard
// 404 for /api/* so the JSON API is never masked by the fallback.
func TestWebStaticServe(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)

	// embedded dist: placeholder index.html must be served at /
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()
	body := getJSON(t, ts.URL+"/")
	if !strings.Contains(body, "Hetu") {
		t.Fatalf("index not served: %q", body)
	}

	// disk override: --web-dir serves that dir instead of the embed
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>disk</html>"), 0o644)
	ws2 := NewWebServer(srv, "", dir)
	ts2 := httptest.NewServer(ws2.Handler())
	defer ts2.Close()
	body2 := getJSON(t, ts2.URL+"/")
	if !strings.Contains(body2, "disk") {
		t.Fatalf("disk override not served: %q", body2)
	}

	// SPA fallback: unknown non-API path returns index.html
	body3 := getJSON(t, ts.URL+"/sessions/abc")
	if !strings.Contains(body3, "Hetu") {
		t.Fatalf("SPA fallback broken: %q", body3)
	}

	// /api/* must NOT fall back to the SPA (stays 404)
	for _, p := range []string{"/api/not-a-real-endpoint", "/api"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 for %s, got %d", p, resp.StatusCode)
		}
	}
}

// TestWebRejectsForeignHost verifies the DNS-rebinding defense: a request with a
// non-loopback Host header (what a rebinding attacker's page would send) is 403.
func TestWebRejectsForeignHost(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/projects", nil)
	req.Host = "evil.example.com:19191"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign Host, got %d", resp.StatusCode)
	}

	// loopback Host stays allowed
	req2, _ := http.NewRequest("GET", ts.URL+"/api/projects", nil)
	req2.Host = "127.0.0.1:19191"
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for loopback Host, got %d", resp2.StatusCode)
	}
}

// newWebTestServer builds a WebServer backed by a fake driven claude agent —
// POST action endpoints (create/send/…) hit a real SessionManager without
// spawning processes. Returns the test server, manager, and fake session.
func newWebTestServer(t *testing.T) (*httptest.Server, *SessionManager, *fake.Session) {
	t.Helper()
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	fs := fake.NewSession("h1", "ext1", statusRunningForTest())
	fa := &fake.Agent{N: "claude", Drv: &fake.Driver{OnStart: func(_ context.Context, _ agent.StartRequest) (agent.Session, error) {
		return fs, nil
	}}}
	m := NewSessionManager(st, r, map[string]agent.Agent{"claude": fa})
	srv := NewServer(st, m, nil)
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	t.Cleanup(ts.Close)
	return ts, m, fs
}

// postJSON POSTs body with an explicit Content-Type and returns the response
// (caller closes resp.Body). contentType lets tests send the guard-violating
// form type on purpose.
func postJSON(t *testing.T, url, contentType, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, contentType, strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// bodyString reads and closes a response body.
func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// TestWebCSRFGuard verifies the write-side CSRF defense: POSTs to /api/* are
// only accepted with an application/json Content-Type. Cross-site HTML forms
// can only send urlencoded/multipart/text-plain; a cross-site fetch sending a
// custom type triggers a CORS preflight this server never answers.
func TestWebCSRFGuard(t *testing.T) {
	ts, _, _ := newWebTestServer(t)

	resp := postJSON(t, ts.URL+"/api/discover", "application/x-www-form-urlencoded", "x=1")
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("form POST: expected 415, got %d", resp.StatusCode)
	}
	if b := bodyString(t, resp); !strings.Contains(b, "application/json") {
		t.Fatalf("415 body should name the required type, got %q", b)
	}

	resp = postJSON(t, ts.URL+"/api/discover", "application/json", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("JSON POST: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// GET is unaffected (no body to forge).
	if _, err := http.Get(ts.URL + "/api/projects"); err != nil {
		t.Fatal(err)
	}
}

// TestWebActionEndpoints covers the write endpoints' error mapping and the
// store-only happy path (mark_read). The approve round-trip with a live
// pending approval is covered separately below.
func TestWebActionEndpoints(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	se, _ := st.UpsertSession(ctx, store.Session{HetuID: "s1", Agent: "claude", ExternalID: "e1", CWD: "/x", Title: "T1", Status: session.StatusCompleted, Unread: true})

	// mark_read happy path: 200 + store flips unread off
	resp := postJSON(t, ts.URL+"/api/sessions/s1/mark_read", "application/json", "{}")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mark_read: expected 200, got %d (body %s)", resp.StatusCode, bodyString(t, resp))
	}
	got, ok, _ := st.GetSession(ctx, se.HetuID)
	if !ok || got.Unread {
		t.Fatal("mark_read did not clear unread in store")
	}

	// approve with no pending → 400 JSON error
	resp = postJSON(t, ts.URL+"/api/sessions/s1/approve", "application/json", `{"allow":true}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("approve no-pending: expected 400, got %d", resp.StatusCode)
	}
	if b := bodyString(t, resp); !strings.Contains(b, "no pending approval") {
		t.Fatalf("approve no-pending body: %q", b)
	}

	// send on a non-driven session → 400
	resp = postJSON(t, ts.URL+"/api/sessions/s1/send", "application/json", `{"prompt":"hi"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("send non-driven: expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// unknown session → 404
	resp = postJSON(t, ts.URL+"/api/sessions/missing/send", "application/json", `{"prompt":"hi"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing session: expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// malformed JSON body → 400
	resp = postJSON(t, ts.URL+"/api/sessions/s1/approve", "application/json", "{not json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad body: expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestWebApproveRoundTrip drives a pending approval through the fake agent,
// then resolves it over HTTP — the full M3 loop in miniature.
func TestWebApproveRoundTrip(t *testing.T) {
	ts, m, _ := newWebTestServer(t)
	ctx := context.Background()
	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}

	decided := make(chan bool, 1)
	go func() {
		d, err := m.RequestPermission(ctx, hid, "toolu_1", "Bash", `{"command":"ls"}`)
		decided <- err == nil && d.Allow
	}()
	waitForApprovalPending(t, m, hid, "toolu_1")

	resp := postJSON(t, ts.URL+"/api/sessions/"+hid+"/approve", "application/json",
		`{"allow":true,"tool_use_id":"toolu_1"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d (body %s)", resp.StatusCode, bodyString(t, resp))
	}
	resp.Body.Close()

	select {
	case ok := <-decided:
		if !ok {
			t.Fatal("approval resolved but decision was not allow")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("approval was never resolved by the HTTP call")
	}
}

// TestWebCreateSession: POST /api/sessions spawns a driven fake session.
func TestWebCreateSession(t *testing.T) {
	ts, _, _ := newWebTestServer(t)
	resp := postJSON(t, ts.URL+"/api/sessions", "application/json", `{"project_path":"/x"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: expected 200, got %d (body %s)", resp.StatusCode, bodyString(t, resp))
	}
	b := bodyString(t, resp)
	if !strings.Contains(b, `"id"`) {
		t.Fatalf("create body missing id: %s", b)
	}
}

// readSSELine reads one line from an SSE response body (blocking).
func readSSELine(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read SSE line: %v", err)
	}
	return strings.TrimRight(line, "\n")
}

// TestWebSSEStream subscribes over HTTP, sees an emitted event, then drops the
// connection — the subscriber must be cleaned up (no leak).
func TestWebSSEStream(t *testing.T) {
	ts, m, fs := newWebTestServer(t)
	ctx := context.Background()
	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(ts.URL + "/api/sessions/" + hid + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q, want text/event-stream", ct)
	}
	br := bufio.NewReader(resp.Body)

	// The handler subscribes before writing the headers, so once headers are
	// back this Emit must reach us.
	fs.Emit(agent.Event{Type: agent.EventText, Text: "hi", Seq: 1})

	got := ""
	deadline := time.Now().Add(2 * time.Second)
	for got == "" && time.Now().Before(deadline) {
		line := readSSELine(t, br)
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"text"`) {
			got = line
		}
	}
	if got == "" {
		t.Fatal("never saw the text event in the SSE stream")
	}
	if !strings.Contains(got, `"text":"hi"`) {
		t.Fatalf("event payload wrong: %q", got)
	}

	// Disconnect: close the body; the handler's r.Context() fires, cancel runs,
	// and the subscriber is removed from the live session.
	resp.Body.Close()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		l, ok := m.live[hid]
		m.mu.Unlock()
		if !ok {
			break // session gone entirely — also clean
		}
		l.mu.Lock()
		n := len(l.subscribers)
		l.mu.Unlock()
		if n == 0 {
			return // clean — test passes
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("subscriber leaked after SSE disconnect")
}

// TestWebSSEEndOnSessionClose: when the driven session finishes, the pump
// closes subscriber channels and the endpoint must emit `event: end`.
func TestWebSSEEndOnSessionClose(t *testing.T) {
	ts, m, fs := newWebTestServer(t)
	ctx := context.Background()
	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(ts.URL + "/api/sessions/" + hid + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)

	_ = fs.Close() // ends the pump → closes subscribers

	sawEnd := false
	deadline := time.Now().Add(2 * time.Second)
	for !sawEnd && time.Now().Before(deadline) {
		if line := readSSELine(t, br); line == "event: end" {
			sawEnd = true
		}
	}
	if !sawEnd {
		t.Fatal("never saw `event: end` after session close")
	}
}

// TestWebSSENotDriven: a stored (non-driven) session has no event stream.
func TestWebSSENotDriven(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertSession(ctx, store.Session{HetuID: "s1", Agent: "claude", ExternalID: "e1", CWD: "/x", Status: session.StatusCompleted})
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	ws := NewWebServer(srv, "", "")
	ts := httptest.NewServer(ws.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions/s1/events")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-driven session, got %d", resp.StatusCode)
	}
	if b := bodyString(t, resp); !strings.Contains(b, "not driven") {
		t.Fatalf("body should explain, got %q", b)
	}
	// (no defer — bodyString already closed it)

	// unknown session → 404
	resp2, err := http.Get(ts.URL + "/api/sessions/missing/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp2.StatusCode)
	}
}
