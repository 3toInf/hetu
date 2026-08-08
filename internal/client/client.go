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
	raw, _ := json.Marshal(body)
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
	return out, json.Unmarshal(r.Body, &out)
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
	return out, json.Unmarshal(r.Body, &out)
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
	_ = json.Unmarshal(r.Body, &res)
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
	return out, json.Unmarshal(r.Body, &out)
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