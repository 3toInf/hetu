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
	list, err := cl.ListSessions(cmd.Context(), "", "", "")
	if err != nil {
		return err
	}
	for _, s := range list {
		if s.HetuID == id {
			printSessionDetails(os.Stdout, s)
			return nil
		}
	}
	return fmt.Errorf("session not found: %s", id)
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
