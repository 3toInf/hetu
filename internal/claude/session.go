package claude

import (
	"bufio"
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
	"github.com/google/uuid"
)

type claudeSession struct {
	id         string
	externalID string
	cwd        string
	bin        string
	proc       process
	events     chan agent.Event
	status     atomic.Value // session.Status
	seq        atomic.Int64
	cancel     context.CancelFunc
	done       chan struct{}
	mu         sync.Mutex
	closed     bool
}

// claudeArgs builds the stream-json command line for new/resume.
func claudeArgs(bin string, mode agent.StartMode, externalID, prompt string) []string {
	args := []string{"-p", "--output-format", "stream-json", "--input-format", "stream-json", "--verbose"}
	if mode == agent.StartResume && externalID != "" {
		args = append(args, "--resume", externalID)
	}
	return args
}

func newSession(ctx context.Context, bin, cwd string, req agent.StartRequest, proc process) (*claudeSession, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &claudeSession{
		id:         uuid.NewString(),
		externalID: req.ExternalID,
		cwd:        cwd,
		bin:        bin,
		proc:       proc,
		events:     make(chan agent.Event, 64),
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	s.status.Store(session.StatusRunning)
	if err := proc.Start(); err != nil {
		cancel()
		return nil, err
	}
	go s.readLoop()
	if req.Prompt != "" {
		_ = s.Send(ctx, req.Prompt)
	}
	return s, nil
}

func (s *claudeSession) push(e agent.Event) {
	s.seq.Add(1)
	e.Seq = int(s.seq.Load())
	select {
	case s.events <- e:
	default: // drop on overflow; daemon persists recent events
	}
}

func (s *claudeSession) readLoop() {
	defer close(s.done)
	defer close(s.events)
	sc := bufio.NewScanner(s.proc.Stdout())
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		ps, ok := ParseStreamLine(sc.Bytes())
		if !ok {
			continue
		}
		ev := ps.Event
		if ev.Type == agent.EventStatus {
			s.status.Store(ev.Status)
		}

		// If we learned a session_id and it differs from our current externalID,
		// update it and emit a meta event so the daemon can persist it.
		if ps.SessionID != "" && ps.SessionID != s.externalID {
			s.mu.Lock()
			s.externalID = ps.SessionID
			s.mu.Unlock()
			s.push(agent.Event{Type: agent.EventMeta, ExternalID: ps.SessionID})
		}

		s.push(ev)
	}
	// process ended
	if st, _ := s.status.Load().(session.Status); st == session.StatusRunning {
		if err := s.proc.Wait(); err != nil {
			s.status.Store(session.StatusError)
			s.push(agent.Event{Type: agent.EventError, Err: err.Error()})
		} else {
			s.status.Store(session.StatusCompleted)
		}
	}
}

func (s *claudeSession) ID() string         { return s.id }
func (s *claudeSession) ExternalID() string { return s.externalID }
func (s *claudeSession) Events() <-chan agent.Event { return s.events }
func (s *claudeSession) Status() session.Status {
	if v, ok := s.status.Load().(session.Status); ok {
		return v
	}
	return session.StatusUnknown
}

func (s *claudeSession) SetStatus(st session.Status) error {
	s.status.Store(st)
	return nil
}

func (s *claudeSession) Send(ctx context.Context, prompt string) error {
	// stream-json user message envelope
	msg := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":` + jsonQuote(prompt) + `}]}}` + "\n"
	_, err := io.WriteString(s.proc.Stdin(), msg)
	return err
}

func (s *claudeSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	_ = s.proc.Kill()
	<-s.done
	return nil
}

// jsonQuote is a minimal JSON string encoder for prompt text.
func jsonQuote(s string) string {
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, []byte(string(r))...)
		}
	}
	b = append(b, '"')
	return string(b)
}
