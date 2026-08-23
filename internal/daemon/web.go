package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/web"
)

// WebServer serves the HTTP JSON API + static frontend on top of the unix-socket
// RPC: each /api/* request is turned into an api.Request and dispatched through
// Server.dispatch, with an io.Writer adapter translating the newline-JSON reply
// into a standard HTTP response. This reuses every op's logic verbatim.
type WebServer struct {
	srv  *Server
	addr string // listen address ("127.0.0.1:19191"); "" = serve via caller's net/http
	// static dir to serve instead of the embedded dist ("" = embed)
	webDir string
}

func NewWebServer(srv *Server, addr, webDir string) *WebServer {
	return &WebServer{srv: srv, addr: addr, webDir: webDir}
}

// httpWriter adapts the newline-JSON api.Encode stream (one api.Response per op)
// into HTTP: OK → 200 + raw Body; error → a JSON {"error": ...} body with a
// status derived from the message. The RPC layer carries no status kind, so a
// missing resource is recognised by its "not found" sentinel (404); everything
// else is a client error → 400.
type httpWriter struct{ w http.ResponseWriter }

func (h *httpWriter) Write(p []byte) (int, error) {
	var resp api.Response
	if err := json.Unmarshal(bytes.TrimSpace(p), &resp); err != nil {
		http.Error(h.w, "bad server response", http.StatusInternalServerError)
		return len(p), nil
	}
	if !resp.OK {
		status := http.StatusBadRequest
		if resp.Err == "not found" {
			status = http.StatusNotFound
		}
		body, _ := json.Marshal(map[string]string{"error": resp.Err})
		h.w.Header().Set("Content-Type", "application/json")
		h.w.WriteHeader(status)
		_, _ = h.w.Write(body)
		return len(p), nil
	}
	h.w.Header().Set("Content-Type", "application/json")
	h.w.WriteHeader(http.StatusOK)
	_, _ = h.w.Write(resp.Body)
	return len(p), nil
}

func (ws *WebServer) call(ctx context.Context, w http.ResponseWriter, op string, body any) {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	ws.srv.dispatch(ctx, &httpWriter{w: w}, api.Request{Op: op, Body: raw})
}

func (ws *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "list_projects", api.ListProjectsReq{})
	})
	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "list_sessions", api.ListSessionsReq{
			ProjectPath: r.URL.Query().Get("project_path"),
			Status:      r.URL.Query().Get("status"),
			Agent:       r.URL.Query().Get("agent"),
		})
	})
	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "get_session", api.GetSessionReq{ID: r.PathValue("id")})
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "search", api.SearchReq{Q: r.URL.Query().Get("q")})
	})
	mux.HandleFunc("POST /api/discover", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "discover", api.DiscoverReq{})
	})
	mux.Handle("/", ws.staticHandler())
	return mux
}

// staticHandler serves the frontend: the embedded dist by default, a disk dir
// when webDir is set. Unknown non-API paths fall back to index.html (SPA routes);
// /api/* paths stay 404 so they never reach the SPA fallback.
func (ws *WebServer) staticHandler() http.Handler {
	var fsys http.FileSystem
	if ws.webDir != "" {
		fsys = http.Dir(ws.webDir)
	} else {
		// go:embed nests the files under dist/, so root the file system there.
		dist, err := fs.Sub(web.Dist, "dist")
		if err != nil {
			panic(err) // dist is embedded at build time; cannot fail
		}
		fsys = http.FS(dist)
	}
	fh := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := fsys.Open(p); err == nil {
			f.Close()
			fh.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for any unknown non-API path. The exact
		// path /api is also API territory — without it the fallback would 200 the
		// SPA for a bare /api.
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Rewrite to the root so FileServer serves index.html directly; rewriting
		// to /index.html would 301-redirect to ./ and loop on nested SPA paths.
		r2 := new(http.Request)
		*r2 = *r
		r2.URL.Path = "/"
		fh.ServeHTTP(w, r2)
	})
}
