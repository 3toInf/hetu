package agent

import (
	"context"
	"time"

	"github.com/3toInf/hetu/internal/session"
)

type Agent interface {
	Name() string
	DiscoverySource() DiscoverySource
	Driver() Driver
}

type DiscoveredSession struct {
	Agent        string         `json:"agent"`
	ExternalID   string         `json:"external_id"`
	CWD          string         `json:"cwd"`
	Title        string         `json:"title"`
	Status       session.Status `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	MessageCount int            `json:"message_count"`
}

type DiscoverOpts struct {
	CWD string // optional: restrict to one working directory
}

type DiscoverySource interface {
	Discover(ctx context.Context, opts DiscoverOpts) (<-chan DiscoveredSession, error)
}

type StartMode string

const (
	StartNew    StartMode = "new"
	StartResume StartMode = "resume"
)

type StartRequest struct {
	Mode       StartMode
	CWD        string
	Prompt     string // optional initial prompt (StartNew)
	ExternalID string // required for StartResume
}

type Driver interface {
	Start(ctx context.Context, req StartRequest) (Session, error)
	StatusOf(ctx context.Context, externalID string) (session.Status, error)
}

type Session interface {
	ID() string
	ExternalID() string
	Send(ctx context.Context, prompt string) error
	Events() <-chan Event
	Status() session.Status
	Close() error
}
