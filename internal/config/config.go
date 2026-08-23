package config

import (
	"os"
	"path/filepath"
	"runtime"
)

func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hetu")
}

func DBPath() string { return filepath.Join(DataDir(), "hetu.sqlite") }

// LockPath returns the daemon's single-instance advisory-lock file.
func LockPath() string { return filepath.Join(DataDir(), "hetud.lock") }

func EnsureDataDir() error { return os.MkdirAll(DataDir(), 0o700) }

// SocketPath returns the daemon unix-socket path, honoring HETU_SOCKET and GOOS conventions.
func SocketPath() string {
	if p := os.Getenv("HETU_SOCKET"); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "linux":
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			return filepath.Join(dir, "hetu", "hetu.sock")
		}
	case "darwin":
		if dir := os.Getenv("DARWIN_USER_TEMP_DIR"); dir != "" {
			return filepath.Join(dir, "hetu.sock")
		}
	}
	return filepath.Join(DataDir(), "hetu.sock")
}

// ClaudeSettingsPath returns the path to the Claude settings file for hook injection.
func ClaudeSettingsPath() string { return filepath.Join(DataDir(), "claude-settings.json") }

// RulesPath returns the approval-rules file path, honoring HETU_RULES.
func RulesPath() string {
	if p := os.Getenv("HETU_RULES"); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "rules.json")
}

// LogPath returns the daemon log file path, honoring HETU_LOG.
func LogPath() string {
	if p := os.Getenv("HETU_LOG"); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "hetud.log")
}
