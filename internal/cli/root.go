package cli

import (
	"fmt"
	"os"

	"github.com/3toInf/hetu/internal/client"
	"github.com/3toInf/hetu/internal/config"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "hetu", Short: "Unified AI Coding session workspace"}
	root.AddCommand(newSessionsCmd(), newSessionCmd(), newNewCmd(), newResumeCmd(),
		newSendCmd(), newSearchCmd(), newProjectsCmd(), newAgentsCmd(), newDiscoverCmd(), newHookPermissionCmd())
	return root
}

func newClient() *client.Client { return client.New(config.SocketPath()) }

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
