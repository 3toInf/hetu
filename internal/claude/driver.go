package claude

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

type Driver struct {
	Binary       string
	SocketPath   string // path to hetu socket for HETU_SOCKET env
	SettingsPath string // path to Claude settings file for --settings flag
}

func (d *Driver) resolveBin() (string, error) {
	if d.Binary != "" {
		return d.Binary, nil
	}
	return exec.LookPath("claude")
}

func (d *Driver) Start(ctx context.Context, req agent.StartRequest) (agent.Session, error) {
	args, env, err := d.launchPlan(req)
	if err != nil {
		return nil, err
	}

	bin, err := d.resolveBin()
	if err != nil {
		return nil, err
	}

	proc, err := newExecProcess(bin, args, req.CWD, env)
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

// SPIKE-CONFIRMED: PermissionRequest does NOT fire in headless `claude -p`.
// PreToolUse fires on every tool call and its permissionDecision is honored.
const hookSettingsFmt = `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"hetu __hook-permission","timeout":540}]}]}}`

// launchPlan builds the command args and env for starting a Claude process.
// This is testable without actually executing the binary.
func (d *Driver) launchPlan(req agent.StartRequest) ([]string, []string, error) {
	// Ensure hook settings file exists if SettingsPath is provided
	if d.SettingsPath != "" {
		if err := d.ensureHookSettings(); err != nil {
			return nil, nil, err
		}
	}

	bin, err := d.resolveBin()
	if err != nil {
		return nil, nil, err
	}

	args := claudeArgs(bin, req.Mode, req.ExternalID, req.Prompt)

	// Add --settings flag if SettingsPath is provided
	if d.SettingsPath != "" {
		args = append(args, "--settings", d.SettingsPath)
	}

	// Build environment with HETU_SOCKET if SocketPath is provided
	env := os.Environ()
	if d.SocketPath != "" {
		env = append(env, "HETU_SOCKET="+d.SocketPath)
	}

	return args, env, nil
}

// ensureHookSettings writes the Claude settings JSON file idempotently.
func (d *Driver) ensureHookSettings() error {
	if d.SettingsPath == "" {
		return nil
	}
	// Create/truncate the file with the hook settings
	return os.WriteFile(d.SettingsPath, []byte(hookSettingsFmt), 0o600)
}
