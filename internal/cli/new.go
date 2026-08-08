package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "new [prompt]",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			return newRun(cmd, &project, args)
		},
	}
	c.Flags().StringVarP(&project, "project", "p", "", "project path")
	return c
}

func newRun(cmd *cobra.Command, project *string, args []string) error {
	var prompt string
	if len(args) > 0 {
		prompt = args[0]
	}
	cl := newClient()
	id, err := cl.Create(cmd.Context(), *project, prompt)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Created session: %s\n", id)
	return nil
}
