package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "projects",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			return projectsRun(cmd)
		},
	}
}

func projectsRun(cmd *cobra.Command) error {
	cl := newClient()
	list, err := cl.ListProjects(cmd.Context())
	if err != nil {
		return err
	}
	printProjects(os.Stdout, list)
	return nil
}
