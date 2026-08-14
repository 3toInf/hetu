package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
)

// Helper functions for test assertions
func contains(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func envHas(env []string, target string) bool {
	for _, e := range env {
		if e == target {
			return true
		}
	}
	return false
}

func TestDriverInjectsSettingsAndSocket(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "claude-settings.json")
	socketPath := "/tmp/x.sock"

	d := &Driver{
		Binary:      "claude",
		SocketPath:  socketPath,
		SettingsPath: settings,
	}

	req := agent.StartRequest{
		Mode: agent.StartNew,
		CWD:  dir,
	}

	args, env, err := d.launchPlan(req)
	if err != nil {
		t.Fatalf("launchPlan failed: %v", err)
	}

	// Verify --settings flag and path are in args
	if !containsPair(args, "--settings", settings) {
		t.Errorf("args %v missing --settings %s", args, settings)
	}

	// Verify HETU_SOCKET is in env
	if !envHas(env, "HETU_SOCKET="+socketPath) {
		t.Errorf("env %v missing HETU_SOCKET=%s", env, socketPath)
	}

	// Call ensureHookSettings and verify file content
	if err := d.ensureHookSettings(); err != nil {
		t.Fatalf("ensureHookSettings failed: %v", err)
	}

	b, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("failed to read settings file: %v", err)
	}

	content := string(b)
	if !strings.Contains(content, "__hook-permission") {
		t.Errorf("settings file missing hook command: %s", content)
	}
	if !strings.Contains(content, "PreToolUse") {
		t.Errorf("settings file missing PreToolUse hook: %s", content)
	}
}

func TestDriverLaunchPlanWithoutSettings(t *testing.T) {
	d := &Driver{
		Binary:      "claude",
		SocketPath:  "/tmp/y.sock",
		SettingsPath: "", // Empty settings path
	}

	req := agent.StartRequest{
		Mode: agent.StartResume,
		CWD:  "/tmp",
	}

	args, env, err := d.launchPlan(req)
	if err != nil {
		t.Fatalf("launchPlan failed: %v", err)
	}

	// Should not contain --settings when SettingsPath is empty
	if contains(args, "--settings") {
		t.Errorf("args %v should not contain --settings when SettingsPath is empty", args)
	}

	// Should still have HETU_SOCKET when SocketPath is set
	if !envHas(env, "HETU_SOCKET=/tmp/y.sock") {
		t.Errorf("env %v missing HETU_SOCKET", env)
	}
}

func TestDriverLaunchPlanWithoutSocket(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "claude-settings.json")

	d := &Driver{
		Binary:      "claude",
		SocketPath:  "", // Empty socket path
		SettingsPath: settings,
	}

	req := agent.StartRequest{
		Mode: agent.StartNew,
		CWD:  dir,
	}

	args, env, err := d.launchPlan(req)
	if err != nil {
		t.Fatalf("launchPlan failed: %v", err)
	}

	// Should contain --settings when SettingsPath is set
	if !containsPair(args, "--settings", settings) {
		t.Errorf("args %v missing --settings %s", args, settings)
	}

	// Should not have HETU_SOCKET when SocketPath is empty
	socketFound := false
	for _, e := range env {
		if strings.HasPrefix(e, "HETU_SOCKET=") {
			socketFound = true
			break
		}
	}
	if socketFound {
		t.Errorf("env %v should not contain HETU_SOCKET when SocketPath is empty", env)
	}
}
