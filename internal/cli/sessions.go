package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newSessionsCmd() *cobra.Command {
	var project, status, agent string
	c := &cobra.Command{
		Use:   "sessions",
		Short: "List sessions (project-first, flattened)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return sessionsRun(cmd, &project, &status, &agent)
		},
	}
	c.Flags().StringVarP(&project, "project", "p", "", "project path filter")
	c.Flags().StringVarP(&status, "status", "s", "", "status filter")
	c.Flags().StringVar(&agent, "agent", "", "agent filter")
	return c
}

func sessionsRun(cmd *cobra.Command, project, status, agent *string) error {
	cl := newClient()
	list, err := cl.ListSessions(cmd.Context(), expandPath(*project), *status, *agent)
	if err != nil {
		return err
	}
	printSessions(os.Stdout, list)
	return nil
}

// expandPath normalizes a user-supplied path filter: expands a leading ~
// (covers quoted "~" the shell won't expand) and strips trailing slashes, so
// -p ~/repo/ matches the stored "/home/user/repo" form.
func expandPath(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return filepath.Clean(p)
}
