package claude

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

type Driver struct {
	Binary string
}

func (d *Driver) resolveBin() (string, error) {
	if d.Binary != "" {
		return d.Binary, nil
	}
	return exec.LookPath("claude")
}

func (d *Driver) Start(ctx context.Context, req agent.StartRequest) (agent.Session, error) {
	bin, err := d.resolveBin()
	if err != nil {
		return nil, err
	}
	proc, err := newExecProcess(bin, claudeArgs(bin, req.Mode, req.ExternalID, req.Prompt), req.CWD)
	if err != nil {
		return nil, err
	}
	return newSession(ctx, bin, req.CWD, req, proc)
}

// StatusOf infers last-known status from the transcript file for an external id.
func (d *Driver) StatusOf(ctx context.Context, externalID string) (session.Status, error) {
	// Scan the projects dir for a file matching externalID across all slugs.
	root := ClaudeProjectsDir()
	matches, err := filepathGlob(root, "*", externalID+".jsonl")
	if err != nil || len(matches) == 0 {
		return session.StatusUnknown, nil
	}
	s, ok := parseTranscript(matches[0])
	if !ok {
		return session.StatusUnknown, nil
	}
	return s.Status, nil
}

// filepathGlob is a helper that glob files under root/slugPattern/file
func filepathGlob(root, slugPattern, file string) ([]string, error) {
	return filepath.Glob(filepath.Join(root, slugPattern, file))
}
