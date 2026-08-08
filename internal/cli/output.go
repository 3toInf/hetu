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
	for _, s := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Agent, s.Title, s.Status, reltime(s.UpdatedAt), short(s.HetuID))
	}
	tw.Flush()
}

func printProjects(w io.Writer, list []api.ProjectDTO) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPATH\tSESSIONS")
	for _, p := range list {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", p.Name, p.Path, p.SessionCount)
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
