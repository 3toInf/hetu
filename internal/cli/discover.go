package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Trigger agent discovery",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := newClient().Discover(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "discovery triggered")
			return nil
		},
	}
}
