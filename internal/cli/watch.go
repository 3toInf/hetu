package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/client"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Stream a session's events live; approve/deny inline",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cl := newClient()
			return runWatch(ctx, cl, args[0], cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// runWatch is the testable core of watch: it streams events and prompts for approval
func runWatch(ctx context.Context, cl *client.Client, id string, in io.Reader, out io.Writer) error {
	ch, err := cl.Watch(ctx, id)
	if err != nil {
		return err
	}

	br := bufio.NewReader(in)
	for ev := range ch {
		renderEvent(out, ev)
		if ev.Type == agent.EventApproval && ev.ApprovalState == "pending" {
			fmt.Fprintf(out, "  approve %q? [y/n]: ", ev.ToolName)
			line, err := br.ReadString('\n')
			if err != nil {
				return err
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "y" {
				if err := cl.Approve(ctx, id, ev.ToolUseID, true, ""); err != nil {
					return err
				}
				fmt.Fprintln(out, "approved")
			} else if trimmed == "n" {
				if err := cl.Approve(ctx, id, ev.ToolUseID, false, "denied in watch"); err != nil {
					return err
				}
				fmt.Fprintln(out, "denied")
			} else {
				fmt.Fprintln(out, "skipped")
			}
		}
	}
	return nil
}

// renderEvent prints a human-readable one-line representation of an event
func renderEvent(w io.Writer, ev agent.Event) {
	switch ev.Type {
	case agent.EventText:
		fmt.Fprintln(w, ev.Text)
	case agent.EventTool:
		fmt.Fprintf(w, "[tool] %s\n", ev.ToolName)
	case agent.EventStatus:
		fmt.Fprintf(w, "→ %s\n", ev.Status)
	case agent.EventApproval:
		if ev.ApprovalState == "pending" {
			fmt.Fprintf(w, "[approval pending] %s (tool: %s)\n", ev.ToolUseID, ev.ToolName)
		} else if ev.ApprovalState == "allowed" {
			fmt.Fprintf(w, "[approval allowed] %s\n", ev.ToolUseID)
		} else if ev.ApprovalState == "denied" {
			fmt.Fprintf(w, "[approval denied] %s\n", ev.ToolUseID)
		}
	case agent.EventError:
		fmt.Fprintf(w, "error: %s\n", ev.Err)
	}
}
