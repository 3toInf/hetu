package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

type Server struct {
	st  *store.Store
	mgr *SessionManager
	dsc *DiscoveryScheduler

	ln   net.Listener
	done chan struct{}
}

func NewServer(st *store.Store, mgr *SessionManager, dsc *DiscoveryScheduler) *Server {
	return &Server{st: st, mgr: mgr, dsc: dsc, done: make(chan struct{})}
}

func (s *Server) Serve(ctx context.Context, socketPath string) error {
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	s.ln = ln
	go func() {
		<-ctx.Done()
		s.Shutdown(ctx)
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				return err
			}
		}
		go s.handle(ctx, c)
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if s.ln != nil {
		_ = s.ln.Close()
	}
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *Server) handle(ctx context.Context, c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			return
		}
		var req api.Request
		if err := json.Unmarshal(line, &req); err != nil {
			api.Encode(c, api.Response{OK: false, Err: "bad request"})
			continue
		}
		s.dispatch(ctx, c, req)
	}
}

func (s *Server) dispatch(ctx context.Context, w io.Writer, req api.Request) {
	switch req.Op {
	case "list_projects":
		var b api.ListProjectsReq
		_ = json.Unmarshal(req.Body, &b)
		ps, err := s.st.ListProjects(ctx)
		if err != nil {
			replyErr(w, err)
			return
		}
		out := make([]api.ProjectDTO, 0, len(ps))
		for _, p := range ps {
			sess, err := s.st.ListSessions(ctx, store.ListFilter{ProjectID: p.ID})
			if err != nil {
				replyErr(w, err)
				return
			}
				attentionCount, _ := s.st.CountAttention(ctx, p.ID)
				out = append(out, api.ProjectDTO{ID: p.ID, Name: p.Name, Path: p.Path, SessionCount: len(sess), AttentionCount: attentionCount})
		}
		replyOK(w, out)
	case "list_sessions":
		var b api.ListSessionsReq
		_ = json.Unmarshal(req.Body, &b)
		f := store.ListFilter{Status: b.Status, Agent: b.Agent}
		if b.ProjectPath != "" {
			if p, ok, _ := s.st.GetProjectByPath(ctx, b.ProjectPath); ok {
				f.ProjectID = p.ID
			}
		}
		sess, err := s.st.ListSessions(ctx, f)
		if err != nil {
			replyErr(w, err)
			return
		}
		out := make([]api.SessionDTO, 0, len(sess))
		for _, se := range sess {
			if st, live := s.mgr.LiveStatus(se.HetuID); live {
				se.Status = st
			}
			out = append(out, toDTO(s.st, se))
		}
		replyOK(w, out)
	case "get_session":
		var b api.GetSessionReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil {
			replyErr(w, err)
			return
		}
		if !ok {
			replyErr(w, errors.New("not found"))
			return
		}
		// Fetch pending approvals for driven sessions
		var pendingDTOs []api.PendingApprovalDTO
		if se.Driven {
			pending := s.mgr.PendingApprovals(se.HetuID)
			pendingDTOs = make([]api.PendingApprovalDTO, 0, len(pending))
			for _, p := range pending {
				pendingDTOs = append(pendingDTOs, api.PendingApprovalDTO{
					ToolUseID: p.ToolUseID,
					ToolName:  p.ToolName,
					ToolInput: p.ToolInput,
				})
			}
		}
		replyOK(w, map[string]any{
			"session": toDTO(s.st, se),
			"pending": pendingDTOs,
		})
	case "resume":
		var b api.ResumeReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil {
			replyErr(w, err)
			return
		}
		if !ok {
			replyErr(w, errors.New("not found"))
			return
		}
		_, err = s.mgr.Ensure(ctx, se.Agent, se.ExternalID, se.CWD)
		if err != nil {
			replyErr(w, err)
			return
		}
		replyOK(w, map[string]any{"id": se.HetuID, "driven": true})
	case "send":
		var b api.SendReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil {
			replyErr(w, err)
			return
		}
		if !ok {
			replyErr(w, errors.New("not found"))
			return
		}
		if err := s.mgr.Send(ctx, se.HetuID, b.Prompt); err != nil {
			replyErr(w, err)
			return
		}
		replyOK(w, map[string]any{"ok": true})
	case "create":
		var b api.CreateReq
		_ = json.Unmarshal(req.Body, &b)
		// create uses resume with a fresh external id (empty) — Driver.Start new
		// For v0.1 minimal: treat create like a new driven session with empty external id.
		id, err := s.mgr.Ensure(ctx, "claude", "", b.ProjectPath)
		if err != nil {
			replyErr(w, err)
			return
		}
		replyOK(w, map[string]any{"id": id})
	case "search":
		var b api.SearchReq
		_ = json.Unmarshal(req.Body, &b)
		// P1: simple LIKE over title
		rows, err := s.st.DB().QueryContext(ctx, `SELECT hetu_id, agent, external_id, COALESCE(project_id,0), host, cwd, COALESCE(title,''), status, driven, unread, created_at, updated_at, last_event_at FROM sessions WHERE title LIKE ? ORDER BY updated_at DESC LIMIT 100`, "%"+b.Q+"%")
		if err != nil {
			replyErr(w, err)
			return
		}
		out := []api.SessionDTO{}
		for rows.Next() {
			var dr int
			var un int
			var ct, ut int64
			var lev *int64
			var se store.Session
			rows.Scan(&se.HetuID, &se.Agent, &se.ExternalID, &se.ProjectID, &se.Host, &se.CWD, &se.Title, &se.Status, &dr, &un, &ct, &ut, &lev)
			out = append(out, toDTO(s.st, se))
		}
		rows.Close()
		replyOK(w, out)
	case "list_agents":
		out := []api.AgentDTO{{Name: "claude", Available: true}}
		replyOK(w, out)
	case "discover":
		if s.dsc != nil {
			_ = s.dsc.Run(ctx)
		}
		replyOK(w, map[string]any{"ok": true})
	case "subscribe", "watch":
		var b api.SubscribeReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil {
			replyErr(w, err)
			return
		}
		if !ok {
			replyErr(w, errors.New("not found"))
			return
		}
		sub, err := s.mgr.Subscribe(se.HetuID)
		if err != nil {
			replyErr(w, err)
			return
		}
		for ev := range sub {
			body, err := json.Marshal(ev)
			if err != nil {
				break
			}
			if err := api.Encode(w, api.Response{OK: true, Body: body}); err != nil {
				break
			}
		}
	case "approve":
		var b api.ApproveReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil || !ok {
			replyErr(w, errOrNotFound(err, ok))
			return
		}
		pending := s.mgr.PendingApprovals(se.HetuID)
		toolID := b.ToolUseID
		if toolID == "" {
			if len(pending) == 0 {
				replyErr(w, errors.New("no pending approval"))
				return
			}
			if len(pending) > 1 {
				replyErr(w, fmt.Errorf("multiple pending; specify --tool: %v", toolIDs(pending)))
				return
			}
			toolID = pending[0].ToolUseID
		}
		found := s.mgr.ResolveApproval(se.HetuID, toolID, ApprovalDecision{Allow: b.Allow, Reason: b.Reason})
		if !found {
			replyErr(w, errors.New("no matching pending approval"))
			return
		}
		replyOK(w, map[string]any{"ok": true})
	case "mark_read":
		var b api.MarkReadReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, err := s.st.ResolveSession(ctx, b.ID)
		if err != nil || !ok {
			replyErr(w, errOrNotFound(err, ok))
			return
		}
		if err := s.st.MarkRead(ctx, se.HetuID); err != nil {
			replyErr(w, err)
			return
		}
		replyOK(w, map[string]any{"ok": true})
	case "permission_request":
		var b api.PermissionRequestReq
		_ = json.Unmarshal(req.Body, &b)
		se, ok, _ := s.st.GetSessionByExternal(ctx, "claude", b.SessionID)
		if !ok {
			replyErr(w, errors.New("session not driven"))
			return
		}
		// Use a ctx capped below the hook's 540s so we return before claude times out.
		rctx, cancel := context.WithTimeout(ctx, 535*time.Second)
		defer cancel()
		d, err := s.mgr.RequestPermission(rctx, se.HetuID, b.ToolUseID, b.ToolName, b.ToolInput)
		allow := d.Allow
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			allow = false
		}
		replyOK(w, api.PermissionRequestResp{Allow: allow, Reason: d.Reason})
	default:
		replyErr(w, errors.New("unknown op: "+req.Op))
	}
}

func replyOK(w io.Writer, body any) {
	b, _ := json.Marshal(body)
	api.Encode(w, api.Response{OK: true, Body: b})
}

func replyErr(w io.Writer, err error) {
	api.Encode(w, api.Response{OK: false, Err: err.Error()})
}

func errOrNotFound(err error, ok bool) error {
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("not found")
	}
	return nil
}

func toolIDs(pending []PendingApproval) []string {
	ids := make([]string, len(pending))
	for i, p := range pending {
		ids[i] = p.ToolUseID
	}
	return ids
}

func toDTO(st *store.Store, se store.Session) api.SessionDTO {
	var path string
	if se.ProjectID != 0 {
		if p, ok := st.ProjectPathByID(context.Background(), se.ProjectID); ok {
			path = p
		}
	}

	// Determine NeedsAttention: sessions that need user attention
	needsAttention := se.Status == session.StatusWaitingForApproval ||
		se.Status == session.StatusWaitingForInput ||
		se.Status == session.StatusError

	var lastViewed int64
	if se.LastViewedAt != nil {
		lastViewed = se.LastViewedAt.Unix()
	}

	return api.SessionDTO{
		HetuID:         se.HetuID,
		Agent:          se.Agent,
		ExternalID:     se.ExternalID,
		ProjectPath:    path,
		CWD:            se.CWD,
		Title:          se.Title,
		Status:         string(se.Status),
		Driven:         se.Driven,
		Unread:         se.Unread,
		NeedsAttention: needsAttention,
		LastViewedAt:   lastViewed,
		UpdatedAt:      se.UpdatedAt.Unix(),
	}
}
