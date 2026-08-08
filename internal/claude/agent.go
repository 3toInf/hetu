package claude

import "github.com/3toInf/hetu/internal/agent"

type Options struct {
	Binary      string
	ProjectsDir string // optional override for DiscoverySource tests
}

type ClaudeAgent struct {
	disc *Discovery
	drv  *Driver
}

func NewAgent(opts Options) *ClaudeAgent {
	return &ClaudeAgent{
		disc: &Discovery{ProjectsDir: opts.ProjectsDir},
		drv:  &Driver{Binary: opts.Binary},
	}
}

func (a *ClaudeAgent) Name() string                        { return "claude" }
func (a *ClaudeAgent) DiscoverySource() agent.DiscoverySource { return a.disc }
func (a *ClaudeAgent) Driver() agent.Driver                { return a.drv }
