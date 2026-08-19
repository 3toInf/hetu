package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/3toInf/hetu/internal/client"
	"github.com/3toInf/hetu/internal/config"
	"github.com/spf13/cobra"
)

// newHookPermissionCmd creates the hidden __hook-permission command
func newHookPermissionCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__hook-permission",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHookPermission(cmd.Context(), config.SocketPath(), os.Stdin, os.Stdout)
		},
	}
}

// runHookPermission is the core logic for the permission hook (testable)
func runHookPermission(ctx context.Context, socketPath string, in io.Reader, out io.Writer) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		// On ANY error, print nothing and exit 0 (silent hook)
		return nil
	}

	var h struct {
		SessionID string          `json:"session_id"`
		ToolName  string          `json:"tool_name"`
		ToolUseID string          `json:"tool_use_id"`
		ToolInput json.RawMessage `json:"tool_input"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		// On ANY error, print nothing and exit 0 (silent hook)
		return nil
	}

	cl := client.New(socketPath)
	cl.NoAutostart = true
	allow, reason, err := cl.RequestPermission(ctx, h.SessionID, h.ToolName, string(h.ToolInput), h.ToolUseID)
	if err != nil {
		// On ANY error (daemon unreachable, session unknown), print nothing and exit 0
		return nil
	}

	// On success, print the decision JSON
	fmt.Fprintln(out, renderPermissionDecision(allow, reason))
	return nil
}

// renderPermissionDecision emits the PreToolUse decision JSON
func renderPermissionDecision(allow bool, reason string) string {
	decision := "allow"
	if !allow {
		decision = "deny"
	}

	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":         "PreToolUse",
			"permissionDecision":    decision,
			"permissionDecisionReason": reason,
		},
	}

	raw, _ := json.Marshal(out)
	return string(raw)
}
