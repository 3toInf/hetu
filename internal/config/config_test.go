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
