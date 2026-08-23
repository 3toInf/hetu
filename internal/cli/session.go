package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/3toInf/hetu/internal/api"
	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "session <id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sessionRun(cmd, args[0])
		},
	}
}

func sessionRun(cmd *cobra.Command, id string) error {
	cl := newClient()
	s, pending, _, err := cl.GetSession(cmd.Context(), id)
	if err != nil {
		return err
	}
	printSessionDetails(os.Stdout, s)

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
