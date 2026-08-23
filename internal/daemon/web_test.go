package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
