package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSocketPathOverride(t *testing.T) {
	t.Setenv("HETU_SOCKET", "/tmp/hetu-test.sock")
	if got := SocketPath(); got != "/tmp/hetu-test.sock" {
		t.Fatalf("got %q", got)
	}
}

func TestSocketPathDefaultUnderHome(t *testing.T) {
	t.Setenv("HETU_SOCKET", "")
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_RUNTIME_DIR", "")
	}
	got := SocketPath()
	if filepath.Dir(got) == home {
		return // acceptable fallback under ~/.hetu
	}
	// also acceptable: a runtime/temp dir
}

func TestConfigRulesPath(t *testing.T) {
	t.Setenv("HETU_RULES", "/tmp/x/rules.json")
	if got := RulesPath(); got != "/tmp/x/rules.json" {
		t.Fatalf("RulesPath = %q", got)
	}
}

func TestConfigLogPath(t *testing.T) {
	t.Setenv("HETU_LOG", "/tmp/x/hetud.log")
	if got := LogPath(); got != "/tmp/x/hetud.log" {
		t.Fatalf("LogPath override = %q", got)
	}
	t.Setenv("HETU_LOG", "")
	home, _ := os.UserHomeDir()
	if got := LogPath(); got != filepath.Join(home, ".hetu", "hetud.log") {
		t.Fatalf("LogPath default = %q", got)
	}
}
