package claude

import (
	"os"
	"path/filepath"
	"strings"
)

func ClaudeConfigDir() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func ClaudeProjectsDir() string { return filepath.Join(ClaudeConfigDir(), "projects") }

// DecodeProjectSlug reverses Claude Code's projects-dir slug for an absolute path.
// Claude stores cwd as the path with '/' replaced by '-' (leading '-' marks absolute).
func DecodeProjectSlug(slug string) string {
	s := slug
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	return "/" + strings.ReplaceAll(s, "-", "/")
}

// EncodeProjectSlug is the best-effort inverse used only for tests.
func EncodeProjectSlug(cwd string) string {
	s := strings.ReplaceAll(cwd, "/", "-")
	return s
}
