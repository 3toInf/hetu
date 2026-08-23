package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/web"
)

// queryLimit parses ?limit=, ignoring empty or invalid values (0 = default).
func queryLimit(r *http.Request) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return 0
}

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

// decodeBody decodes a JSON request body into v, replying 400 on malformed
// input. Returns false when the handler must stop.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid JSON body"}`))
		return false
	}
	return true
}

// handleEvents streams a driven session's events over SSE. It bypasses
// ws.call/httpWriter on purpose: that adapter assumes exactly one response
// write per op, while this stream writes per event for as long as the client
// stays connected. Unnamed data events carry agent.Event JSON; `event: end`
// marks session completion (the pump closed our channel); a comment heartbeat
// every 15s keeps idle proxies from dropping the connection.
func (ws *WebServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	se, ok, err := ws.srv.st.ResolveSession(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	sub, cancel, err := ws.srv.mgr.Subscribe(se.HetuID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"session not driven"}`))
		return
	}
	defer cancel() // client disconnect, session end, or handler exit: unsubscribe

	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n") // fast EventSource reconnect
	flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case ev, open := <-sub:
			if !open {
				// pump closed subscribers: session finished. Tell the browser to
				// stop reconnecting (it would only get 400s for a dead session).
				fmt.Fprint(w, "event: end\ndata: {}\n\n")
				flush()
				return
			}
			body, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", body)
			flush()
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flush()
		case <-r.Context().Done():
			return
		}
	}
}

// requireJSON is the write-side CSRF guard: every API POST must carry an
// application/json Content-Type. A cross-site HTML form can only produce
// urlencoded/multipart/text-plain bodies; a cross-site fetch sending a custom
// type triggers a CORS preflight this server never answers — so in both cases
// the write is blocked without any token machinery.
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api") {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_, _ = w.Write([]byte(`{"error":"content-type must be application/json"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
			Limit:       queryLimit(r),
		})
	})
	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "get_session", api.GetSessionReq{ID: r.PathValue("id"), Limit: queryLimit(r)})
	})
	mux.HandleFunc("GET /api/sessions/{id}/events", ws.handleEvents)
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "search", api.SearchReq{Q: r.URL.Query().Get("q")})
	})
	mux.HandleFunc("POST /api/discover", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "discover", api.DiscoverReq{})
	})
	mux.HandleFunc("POST /api/sessions/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Allow     bool   `json:"allow"`
			ToolUseID string `json:"tool_use_id"`
			Reason    string `json:"reason"`
		}
		if !decodeBody(w, r, &b) {
			return
		}
		ws.call(r.Context(), w, "approve", api.ApproveReq{ID: r.PathValue("id"), ToolUseID: b.ToolUseID, Allow: b.Allow, Reason: b.Reason})
	})
	mux.HandleFunc("POST /api/sessions/{id}/send", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Prompt string `json:"prompt"`
		}
		if !decodeBody(w, r, &b) {
			return
		}
		ws.call(r.Context(), w, "send", api.SendReq{ID: r.PathValue("id"), Prompt: b.Prompt})
	})
	mux.HandleFunc("POST /api/sessions/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "resume", api.ResumeReq{ID: r.PathValue("id")})
	})
	mux.HandleFunc("POST /api/sessions/{id}/mark_read", func(w http.ResponseWriter, r *http.Request) {
		ws.call(r.Context(), w, "mark_read", api.MarkReadReq{ID: r.PathValue("id")})
	})
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ProjectPath string `json:"project_path"`
		}
		if !decodeBody(w, r, &b) {
			return
		}
		ws.call(r.Context(), w, "create", api.CreateReq{ProjectPath: b.ProjectPath})
	})
	mux.Handle("/", ws.staticHandler())
	// Wrap the mux with a Host check: a malicious page can re-bind its domain to
	// 127.0.0.1 (DNS rebinding) and issue same-origin fetches to this server, so
	// only loopback names — and the configured listen host — may reach it.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ws.hostAllowed(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		requireJSON(mux).ServeHTTP(w, r)
	})
}

// hostAllowed reports whether the request's Host header names a host we serve.
// Loopback names are always allowed; a non-wildcard listen host is allowed too.
func (ws *WebServer) hostAllowed(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	if ah := ws.listenHost(); ah != "" {
		return host == ah
	}
	return false
}

// listenHost extracts the host part of the configured listen address
// ("127.0.0.1:19191" → "127.0.0.1"); "" for wildcard binds or when unset.
func (ws *WebServer) listenHost() string {
	h, _, err := net.SplitHostPort(ws.addr)
	if err != nil {
		return ""
	}
	if h == "" || h == "0.0.0.0" || h == "::" {
		return ""
	}
	return h
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
