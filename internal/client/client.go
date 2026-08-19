package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os/exec"
	"syscall"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/api"
)

type Client struct {
	Socket     string
	NoAutostart bool
}

func New(socket string) *Client { return &Client{Socket: socket} }

func (c *Client) ensureConn(ctx context.Context) (net.Conn, error) {
	conn, err := net.Dial("unix", c.Socket)
	if err == nil {
		return conn, nil
	}
	if c.NoAutostart {
		return nil, err
	}
	// autostart hetud detached
	if err := startDaemon(); err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		if deadline.Err() != nil {
			return nil, errors.New("daemon did not come up")
		}
		conn, err := net.Dial("unix", c.Socket)
		if err == nil {
			return conn, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *Client) call(ctx context.Context, op string, body any) (api.Response, error) {
	conn, err := c.ensureConn(ctx)
	if err != nil {
		return api.Response{}, err
	}
	defer conn.Close()
	raw, err := json.Marshal(body)
	if err != nil {
		return api.Response{}, err
	}
	if err := api.Encode(conn, api.Request{Op: op, Body: raw}); err != nil {
		return api.Response{}, err
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return api.Response{}, err
	}
	var resp api.Response
	return resp, json.Unmarshal(line, &resp)
}

func (c *Client) ListProjects(ctx context.Context) ([]api.ProjectDTO, error) {
	r, err := c.call(ctx, "list_projects", api.ListProjectsReq{})
	if err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, errors.New(r.Err)
	}
	var out []api.ProjectDTO
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListSessions(ctx context.Context, projectPath, status, agentName string) ([]api.SessionDTO, error) {
	r, err := c.call(ctx, "list_sessions", api.ListSessionsReq{ProjectPath: projectPath, Status: status, Agent: agentName})
	if err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, errors.New(r.Err)
	}
	var out []api.SessionDTO
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Resume(ctx context.Context, id string) error {
	r, err := c.call(ctx, "resume", api.ResumeReq{ID: id})
	if err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Err)
	}
	return nil
}

func (c *Client) Send(ctx context.Context, id, prompt string) error {
	r, err := c.call(ctx, "send", api.SendReq{ID: id, Prompt: prompt})
	if err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Err)
	}
	return nil
}

func (c *Client) Create(ctx context.Context, projectPath, prompt string) (string, error) {
	r, err := c.call(ctx, "create", api.CreateReq{ProjectPath: projectPath, Prompt: prompt})
	if err != nil {
		return "", err
	}
	if !r.OK {
		return "", errors.New(r.Err)
	}
	var res struct{ ID string `json:"id"` }
	if err := json.Unmarshal(r.Body, &res); err != nil {
		return "", err
	}
	return res.ID, nil
}

func (c *Client) Search(ctx context.Context, q string) ([]api.SessionDTO, error) {
	r, err := c.call(ctx, "search", api.SearchReq{Q: q})
	if err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, errors.New(r.Err)
	}
	var out []api.SessionDTO
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListAgents(ctx context.Context) ([]api.AgentDTO, error) {
	r, err := c.call(ctx, "list_agents", api.ListAgentsReq{})
	if err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, errors.New(r.Err)
	}
	var out []api.AgentDTO
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Discover(ctx context.Context) error {
	r, err := c.call(ctx, "discover", api.DiscoverReq{})
	if err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Err)
	}
	return nil
}

func (c *Client) Approve(ctx context.Context, id, toolUseID string, allow bool, reason string) error {
	r, err := c.call(ctx, "approve", api.ApproveReq{ID: id, ToolUseID: toolUseID, Allow: allow, Reason: reason})
	if err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Err)
	}
	return nil
}

func (c *Client) MarkRead(ctx context.Context, id string) error {
	r, err := c.call(ctx, "mark_read", api.MarkReadReq{ID: id})
	if err != nil {
		return err
	}
	if !r.OK {
		return errors.New(r.Err)
	}
	return nil
}

func (c *Client) RequestPermission(ctx context.Context, sessionID, toolName, toolInput, toolUseID string) (bool, string, error) {
	r, err := c.call(ctx, "permission_request", api.PermissionRequestReq{SessionID: sessionID, ToolName: toolName, ToolInput: toolInput, ToolUseID: toolUseID})
	if err != nil {
		return false, "", err
	}
	if !r.OK {
		return false, "", errors.New(r.Err)
	}
	var res api.PermissionRequestResp
	if err := json.Unmarshal(r.Body, &res); err != nil {
		return false, "", err
	}
	return res.Allow, res.Reason, nil
}

func (c *Client) GetSession(ctx context.Context, id string) (api.SessionDTO, []api.PendingApprovalDTO, error) {
	r, err := c.call(ctx, "get_session", api.GetSessionReq{ID: id})
	if err != nil {
		return api.SessionDTO{}, nil, err
	}
	if !r.OK {
		return api.SessionDTO{}, nil, errors.New(r.Err)
	}
	var res struct {
		Session api.SessionDTO          `json:"session"`
		Pending []api.PendingApprovalDTO `json:"pending"`
	}
	if err := json.Unmarshal(r.Body, &res); err != nil {
		return api.SessionDTO{}, nil, err
	}
	return res.Session, res.Pending, nil
}

func (c *Client) Watch(ctx context.Context, id string) (<-chan agent.Event, error) {
	conn, err := c.ensureConn(ctx)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(api.WatchReq{ID: id})
	if err != nil {
		conn.Close()
		return nil, err
	}

	if err := api.Encode(conn, api.Request{Op: "watch", Body: body}); err != nil {
		conn.Close()
		return nil, err
	}

	out := make(chan agent.Event, 64)
	go func() {
		defer conn.Close()
		defer close(out)
		dec := json.NewDecoder(conn)
		for {
			var resp api.Response
			if err := dec.Decode(&resp); err != nil {
				// EOF or decode error
				return
			}
			if !resp.OK {
				return
			}
			var ev agent.Event
			if err := json.Unmarshal(resp.Body, &ev); err == nil {
				select {
				case out <- ev:
				default: // buffer full: drop (consumer too slow; matches broadcast semantics)
				}
			} else {
				// failed to unmarshal event body
				return
			}
		}
	}()

	// Close connection when context is cancelled to avoid goroutine leak
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return out, nil
}

// startDaemon spawns hetud detached (best-effort).
func startDaemon() error {
	exe, err := exec.LookPath("hetud")
	if err != nil {
		return errors.New("hetud not found; run `hetu serve`")
	}
	cmd := exec.Command(exe, "serve")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
