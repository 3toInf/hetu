package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newAgentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "List available agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentsRun(cmd)
		},
	}
}

func agentsRun(cmd *cobra.Command) error {
	cl := newClient()
	list, err := cl.ListAgents(cmd.Context())
	if err != nil {
		return err
	}
	printAgents(os.Stdout, list)
	return nil
}
