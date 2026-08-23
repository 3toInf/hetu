package logx_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/3toInf/hetu/internal/logx"
)

func TestLevelFromEnv(t *testing.T) {
	cases := map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError, "": slog.LevelInfo, "bogus": slog.LevelInfo}
	for in, want := range cases {
		t.Setenv("HETU_LOG_LEVEL", in)
		if got := logx.LevelFromEnv(); got != want {
			t.Errorf("level %q = %v want %v", in, got, want)
		}
	}
}

func TestSetupWritesAndLevels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hetud.log")
	lg, f, err := logx.Setup(logx.Options{Path: path, Level: slog.LevelInfo})
	if err != nil || f == nil {
		t.Fatalf("setup: %v %v", f, err)
	}
	lg.Info("hello", "key", "val")
	lg.Debug("hidden")
	f.Close()
	b, _ := os.ReadFile(path)
	s := string(b)
	if !strings.Contains(s, "hello") || !strings.Contains(s, "key=val") {
		t.Fatalf("log content: %q", s)
	}
	if strings.Contains(s, "hidden") {
		t.Fatal("debug must be filtered at info level")
	}
}

func TestRotation(t *testing.T) {
	old := logx.MaxBytes
	logx.MaxBytes = 64 * 1024
	t.Cleanup(func() { logx.MaxBytes = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "hetud.log")
	lg, f, err := logx.Setup(logx.Options{Path: path, Level: slog.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2000; i++ {
		lg.Info("filler-line-to-exceed-the-5mb-rotation-threshold-0123456789")
	}
	f.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatal("rotated file .1 must exist")
	}
	b, _ := os.ReadFile(path)
	if len(b) > 5*1024*1024 {
		t.Fatal("active log must be under the cap after rotation")
	}
}
