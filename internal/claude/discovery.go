package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

type Discovery struct {
	ProjectsDir string // overrides ClaudeProjectsDir() for tests
}

// DiscoverOptsAll is a convenience zero-value option meaning "no filter".
var DiscoverOptsAll = agent.DiscoverOpts{}

type transcriptLine struct {
	Type      string                 `json:"type"`
	Subtype   string                 `json:"subtype"`
	Timestamp string                 `json:"timestamp"`
	Message   map[string]interface{} `json:"message"`
}

func firstUserText(msg map[string]interface{}) string {
	if msg == nil {
		return ""
	}
	c, ok := msg["content"]
	if !ok {
		return ""
	}
	switch v := c.(type) {
	case string:
		return truncate(strings.TrimSpace(v), 80)
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if txt, ok := m["text"].(string); ok {
						return truncate(strings.TrimSpace(txt), 80)
					}
				}
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

func inferStatus(lines []transcriptLine) session.Status {
	if len(lines) == 0 {
		return session.StatusUnknown
	}
	last := lines[len(lines)-1]
	switch last.Type {
	case "result":
		if last.Subtype == "error" || last.Subtype == "error_during_execution" {
			return session.StatusError
		}
		return session.StatusCompleted
	case "user":
		return session.StatusIdle
	default:
		return session.StatusCompleted
	}
}

func (d *Discovery) projectsDir() string {
	if d.ProjectsDir != "" {
		return d.ProjectsDir
	}
	return ClaudeProjectsDir()
}

func (d *Discovery) Discover(ctx context.Context, _ agent.DiscoverOpts) (<-chan agent.DiscoveredSession, error) {
	out := make(chan agent.DiscoveredSession)
	go func() {
		defer close(out)
		root := d.projectsDir()
		_ = filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			select {
			case <-ctx.Done():
				return filepath.SkipDir
			default:
			}
			s, ok := parseTranscript(path)
			if !ok {
				return nil
			}
			out <- s
			return nil
		})
	}()
	return out, nil
}

func parseTranscript(path string) (agent.DiscoveredSession, bool) {
	f, err := os.Open(path)
	if err != nil {
		return agent.DiscoveredSession{}, false
	}
	defer f.Close()

	var lines []transcriptLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var l transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &l); err == nil {
			lines = append(lines, l)
		}
	}
	base := filepath.Base(path)
	externalID := strings.TrimSuffix(base, ".jsonl")
	slug := filepath.Base(filepath.Dir(path))
	s := agent.DiscoveredSession{
		Agent:      "claude",
		ExternalID: externalID,
		CWD:        DecodeProjectSlug(slug),
		Status:     inferStatus(lines),
	}
	count := 0
	for _, l := range lines {
		if l.Type == "user" || l.Type == "assistant" {
			count++
		}
		if s.Title == "" && l.Type == "user" {
			s.Title = firstUserText(l.Message)
		}
		if t := parseTimestamp(l.Timestamp); !t.IsZero() {
			if s.CreatedAt.IsZero() {
				s.CreatedAt = t
			}
			s.UpdatedAt = t
		}
	}
	s.MessageCount = count
	return s, true
}
