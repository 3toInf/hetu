package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search sessions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return searchRun(cmd, args[0])
		},
	}
}

func searchRun(cmd *cobra.Command, query string) error {
	cl := newClient()
	results, err := cl.Search(cmd.Context(), query)
	if err != nil {
		return err
	}
	for _, r := range results {
		fmt.Fprintf(os.Stdout, "%s\t%s\t(%d hits)\n", r.Agent, r.Title, r.Hits)
	}
	return nil
}
