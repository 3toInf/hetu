package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send <id> <prompt>",
		Short: "Send a prompt to a session",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := newClient().Send(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "sent")
			return nil
		},
	}
}
