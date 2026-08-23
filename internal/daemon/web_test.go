package daemon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
}
