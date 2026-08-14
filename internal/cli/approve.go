package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newApproveCmd() *cobra.Command {
	var tool, reason string
	c := &cobra.Command{
		Use:   "approve <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Approve a pending tool call in a session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := newClient().Approve(cmd.Context(), args[0], tool, true, reason); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "approved")
			return nil
		},
	}
	c.Flags().StringVar(&tool, "tool", "", "tool_use_id (required if multiple pending)")
	c.Flags().StringVar(&reason, "reason", "", "reason (deny only)")
	return c
}

func newDenyCmd() *cobra.Command {
	var tool, reason string
	c := &cobra.Command{
		Use:   "deny <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Deny a pending tool call in a session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := newClient().Approve(cmd.Context(), args[0], tool, false, reason); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "denied")
			return nil
		},
	}
	c.Flags().StringVar(&tool, "tool", "", "tool_use_id (required if multiple pending)")
	c.Flags().StringVar(&reason, "reason", "", "denial reason shown to the agent")
	return c
}
