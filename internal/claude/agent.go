package claude

import "github.com/3toInf/hetu/internal/agent"

type Options struct {
	Binary       string
	ProjectsDir  string // optional override for DiscoverySource tests
	SocketPath   string // path to hetu socket for HETU_SOCKET env
	SettingsPath string // path to Claude settings file for --settings flag
}

type ClaudeAgent struct {
	disc *Discovery
	drv  *Driver
}

func NewAgent(opts Options) *ClaudeAgent {
	return &ClaudeAgent{
		disc: &Discovery{ProjectsDir: opts.ProjectsDir},
		drv:  &Driver{
			Binary:      opts.Binary,
			SocketPath:  opts.SocketPath,
			SettingsPath: opts.SettingsPath,
		},
	}
}

func (a *ClaudeAgent) Name() string                        { return "claude" }
func (a *ClaudeAgent) DiscoverySource() agent.DiscoverySource { return a.disc }
func (a *ClaudeAgent) Driver() agent.Driver                { return a.drv }
