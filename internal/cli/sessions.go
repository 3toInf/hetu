package cli

import (
	"os"

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
	list, err := cl.ListSessions(cmd.Context(), *project, *status, *agent)
	if err != nil {
		return err
	}
	printSessions(os.Stdout, list)
	return nil
}
