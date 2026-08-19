package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/3toInf/hetu/internal/api"
)

func printSessions(w io.Writer, list []api.SessionDTO) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENT\tTITLE\tSTATUS\tUPDATED\tID")

	// Partition into three groups
	needsAttention := []api.SessionDTO{}
	running := []api.SessionDTO{}
	recent := []api.SessionDTO{}

	for _, s := range list {
		if s.NeedsAttention {
			needsAttention = append(needsAttention, s)
		} else if s.Status == "Running" {
			running = append(running, s)
		} else {
			recent = append(recent, s)
		}
	}

	// Print each group with header
	printSessionGroup(tw, "Needs Attention", needsAttention)
	printSessionGroup(tw, "Running", running)
	printSessionGroup(tw, "Recent", recent)

	tw.Flush()
}

func printSessionGroup(tw *tabwriter.Writer, header string, sessions []api.SessionDTO) {
	if len(sessions) == 0 {
		return
	}
	fmt.Fprintln(tw, header)
	for _, s := range sessions {
		title := s.Title
		if s.Unread {
			title = title + " ●"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Agent, title, s.Status, reltime(s.UpdatedAt), short(s.HetuID))
	}
}

func printProjects(w io.Writer, list []api.ProjectDTO) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPATH\tSESSIONS\tATTN")
	for _, p := range list {
		badge := ""
		if p.AttentionCount > 0 {
			badge = fmt.Sprintf("⚑%d", p.AttentionCount)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", p.Name, p.Path, p.SessionCount, badge)
	}
	tw.Flush()
}

func printAgents(w io.Writer, list []api.AgentDTO) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAVAILABLE\tBINARY")
	for _, a := range list {
		fmt.Fprintf(tw, "%s\t%v\t%s\n", a.Name, a.Available, a.Binary)
	}
	tw.Flush()
}

func reltime(unix int64) string {
	if unix == 0 {
		return "-"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
