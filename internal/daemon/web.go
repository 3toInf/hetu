package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/3toInf/hetu/internal/api"
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
// into HTTP: OK → 200 + raw Body; error → 4xx + {"error": ...}.
type httpWriter struct{ w http.ResponseWriter }

func (h *httpWriter) Write(p []byte) (int, error) {
	var resp api.Response
	if err := json.Unmarshal(bytes.TrimSpace(p), &resp); err != nil {
		http.Error(h.w, "bad server response", http.StatusInternalServerError)
		return len(p), nil
	}
	if !resp.OK {
		h.w.Header().Set("Content-Type", "application/json")
		http.Error(h.w, resp.Err, http.StatusBadRequest)
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

// staticHandler serves the frontend. Task 1 ships no static assets yet, so any
// non-API path is a 404; Task 2 replaces this with the embedded/disk file server.
func (ws *WebServer) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
