package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/3toInf/hetu/internal/api"
	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "session <id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sessionRun(cmd, args[0])
		},
	}
	c.Flags().Int("limit", 20, "show up to N recent messages; 0 = all (server returns 20, so this only truncates)")
	return c
}

func sessionRun(cmd *cobra.Command, id string) error {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	cl := newClient()
	s, pending, msgs, err := cl.GetSession(cmd.Context(), id)
	if err != nil {
		return err
	}
	printSessionDetailsWithMessages(os.Stdout, s, msgs, limit)

	// Mark the session as read (best-effort)
	_ = cl.MarkRead(cmd.Context(), s.HetuID)

	// Show pending approvals if any
	if len(pending) > 0 {
		fmt.Fprintf(os.Stdout, "\nPending Approvals:\n")
		for _, p := range pending {
			fmt.Fprintf(os.Stdout, "  %s  %s  %s\n", p.ToolUseID, p.ToolName, p.ToolInput)
		}
		fmt.Fprintf(os.Stdout, "→ hetu approve|deny %s --tool <tool_use_id>\n", s.HetuID)
	}

	return nil
}

func printSessionDetails(w io.Writer, s api.SessionDTO) {
	fmt.Fprintf(w, "ID:          %s\n", s.HetuID)
	fmt.Fprintf(w, "Agent:        %s\n", s.Agent)
	fmt.Fprintf(w, "Title:        %s\n", s.Title)
	fmt.Fprintf(w, "Status:       %s\n", s.Status)
	fmt.Fprintf(w, "Project:      %s\n", s.ProjectPath)
	fmt.Fprintf(w, "CWD:          %s\n", s.CWD)
	fmt.Fprintf(w, "External ID:  %s\n", s.ExternalID)
	fmt.Fprintf(w, "Driven:       %v\n", s.Driven)
	fmt.Fprintf(w, "Updated:      %s\n", reltime(s.UpdatedAt))
}

// printSessionDetailsWithMessages prints the session details block followed by
// the transcript. Messages arrive newest-first (RecentMessages orders seq
// DESC), so truncate to the newest `limit` and render oldest-first.
func printSessionDetailsWithMessages(w io.Writer, s api.SessionDTO, msgs []api.MessageDTO, limit int) {
	printSessionDetails(w, s)
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[:limit]
	}
	if len(msgs) == 0 {
		return
	}
	fmt.Fprintf(w, "\n── Messages ──\n")
	for i := len(msgs) - 1; i >= 0; i-- {
		fmt.Fprintf(w, "%s: %s\n", msgs[i].Role, msgs[i].Content)
	}
}
