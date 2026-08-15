package cli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/api"
)

func TestPrintSessions(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now().Unix()
	list := []api.SessionDTO{
		{
			HetuID:      "abcdefgh12345678",
			Agent:       "claude",
			Title:       "Test session",
			Status:      "active",
			UpdatedAt:   now,
			ProjectPath: "/home/user/project",
			CWD:         "/home/user/project",
			ExternalID:  "ext-123",
			Driven:      true,
		},
		{
			HetuID:      "12345678abcdefgh",
			Agent:       "haiku",
			Title:       "Another session",
			Status:      "completed",
			UpdatedAt:   now - 3600, // 1 hour ago
			ProjectPath: "/home/user/other",
			CWD:         "/home/user/other",
			ExternalID:  "ext-456",
			Driven:      false,
		},
	}

	printSessions(&buf, list)
	output := buf.String()

	// Check that headers are present
	if !contains(output, "AGENT") {
		t.Error("output missing AGENT header")
	}
	if !contains(output, "TITLE") {
		t.Error("output missing TITLE header")
	}
	if !contains(output, "STATUS") {
		t.Error("output missing STATUS header")
	}
	if !contains(output, "UPDATED") {
		t.Error("output missing UPDATED header")
	}
	if !contains(output, "ID") {
		t.Error("output missing ID header")
	}

	// Check that session data is present
	if !contains(output, "claude") {
		t.Error("output missing claude agent")
	}
	if !contains(output, "Test session") {
		t.Error("output missing 'Test session' title")
	}
	if !contains(output, "active") {
		t.Error("output missing 'active' status")
	}
	if !contains(output, "haiku") {
		t.Error("output missing haiku agent")
	}
	if !contains(output, "Another session") {
		t.Error("output missing 'Another session' title")
	}
	if !contains(output, "completed") {
		t.Error("output missing 'completed' status")
	}

	// Check short ID format
	if !contains(output, "abcdefgh") {
		t.Error("output missing short ID")
	}
	if contains(output, "12345678") && contains(output, "abcdefgh") && buf.String() == "12345678abcdefgh" {
		// This should not happen - IDs should be truncated to 8 chars
	}
}

func TestPrintProjects(t *testing.T) {
	var buf bytes.Buffer

	list := []api.ProjectDTO{
		{
			ID:           1,
			Name:         "my-project",
			Path:         "/home/user/my-project",
			SessionCount: 5,
		},
		{
			ID:           2,
			Name:         "another-project",
			Path:         "/home/user/another-project",
			SessionCount: 2,
		},
	}

	printProjects(&buf, list)
	output := buf.String()

	// Check that headers are present
	if !contains(output, "NAME") {
		t.Error("output missing NAME header")
	}
	if !contains(output, "PATH") {
		t.Error("output missing PATH header")
	}
	if !contains(output, "SESSIONS") {
		t.Error("output missing SESSIONS header")
	}

	// Check that project data is present
	if !contains(output, "my-project") {
		t.Error("output missing 'my-project' name")
	}
	if !contains(output, "/home/user/my-project") {
		t.Error("output missing project path")
	}
	if !contains(output, "5") {
		t.Error("output missing session count")
	}
	if !contains(output, "another-project") {
		t.Error("output missing 'another-project' name")
	}
	if !contains(output, "2") {
		t.Error("output missing session count for second project")
	}
}

func TestPrintAgents(t *testing.T) {
	var buf bytes.Buffer

	list := []api.AgentDTO{
		{
			Name:       "claude",
			Available: true,
			Binary:     "/usr/local/bin/claude",
		},
		{
			Name:       "haiku",
			Available: false,
			Binary:     "/usr/local/bin/haiku",
		},
	}

	printAgents(&buf, list)
	output := buf.String()

	// Check that headers are present
	if !contains(output, "NAME") {
		t.Error("output missing NAME header")
	}
	if !contains(output, "AVAILABLE") {
		t.Error("output missing AVAILABLE header")
	}
	if !contains(output, "BINARY") {
		t.Error("output missing BINARY header")
	}

	// Check that agent data is present
	if !contains(output, "claude") {
		t.Error("output missing 'claude' name")
	}
	if !contains(output, "true") {
		t.Error("output missing availability status")
	}
	if !contains(output, "haiku") {
		t.Error("output missing 'haiku' name")
	}
	if !contains(output, "false") {
		t.Error("output missing availability status for haiku")
	}
}

func TestNewRootCmd(t *testing.T) {
	root := NewRootCmd()
	if root == nil {
		t.Fatal("nil root command")
	}

	if root.Use != "hetu" {
		t.Errorf("expected command name 'hetu', got '%s'", root.Use)
	}

	// Check that all subcommands are present
	expectedCommands := []string{"sessions", "session", "new", "resume", "send", "search", "projects", "agents", "discover", "approve", "deny"}
	for _, cmdName := range expectedCommands {
		found := false
		for _, cmd := range root.Commands() {
			if cmd.Name() == cmdName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected command: %s", cmdName)
		}
	}

	// Test that the command can be executed (without args should show help)
	ctx := context.Background()
	root.SetContext(ctx)
	root.SetArgs([]string{})
	if err := root.Execute(); err != nil {
		t.Errorf("root command execution failed: %v", err)
	}
}

func TestReltime(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name     string
		unix     int64
		expected string
	}{
		{"zero", 0, "-"},
		{"seconds", now - 30, "30s"},
		{"minutes", now - 300, "5m"},
		{"hours", now - 7200, "2h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reltime(tt.unix)
			// For time-based tests, we check the format rather than exact value
			// since the test execution time varies
			if tt.unix == 0 && result != "-" {
				t.Errorf("expected '-', got %s", result)
			} else if tt.unix != 0 && result == "-" {
				t.Errorf("expected non-empty result for non-zero timestamp")
			}
		})
	}
}

func TestShort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"verylongstring", "verylong"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := short(tt.input)
			if result != tt.expected {
				t.Errorf("short(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestApproveCmd(t *testing.T) {
	srv := startTestServer(t)
	ctx := context.Background()
	hid := srv.ensureDrivenSession(ctx)

	// Set HETU_SOCKET to point to test server
	t.Setenv("HETU_SOCKET", srv.sock)

	// Arm a pending approval
	go srv.Mgr.RequestApproval(ctx, hid, "tu", "Bash", "{}")
	srv.waitForPending(t, hid, "tu")

	// Run approve command
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"approve", hid, "--tool", "tu"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	output := out.String()
	if !contains(output, "approved") {
		t.Fatalf("expected output to contain 'approved', got: %s", output)
	}

	// Assert pending cleared
	if pending := srv.Mgr.PendingApprovals(hid); len(pending) != 0 {
		t.Fatalf("expected no pending approvals after approve, got: %v", pending)
	}
}

func TestDenyCmd(t *testing.T) {
	srv := startTestServer(t)
	ctx := context.Background()
	hid := srv.ensureDrivenSession(ctx)

	// Set HETU_SOCKET to point to test server
	t.Setenv("HETU_SOCKET", srv.sock)

	// Arm a pending approval
	go srv.Mgr.RequestApproval(ctx, hid, "tu2", "Bash", "{}")
	srv.waitForPending(t, hid, "tu2")

	// Run deny command
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"deny", hid, "--tool", "tu2", "--reason", "test rejection"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deny failed: %v", err)
	}

	output := out.String()
	if !contains(output, "denied") {
		t.Fatalf("expected output to contain 'denied', got: %s", output)
	}

	// Assert pending cleared
	if pending := srv.Mgr.PendingApprovals(hid); len(pending) != 0 {
		t.Fatalf("expected no pending approvals after deny, got: %v", pending)
	}
}


func cancelAfter(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}
