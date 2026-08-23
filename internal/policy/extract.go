package policy

import (
	"encoding/json"
	"net/url"
	"strings"
	"sync"
)

// Extractor maps a native (toolName, toolInput JSON) pair to a canonical
// (Kind, subject). Failures must return (KindOther, "").
type Extractor func(toolName, toolInput string) (Kind, string)

var (
	exMu   sync.RWMutex
	exRegs = map[string]Extractor{}
)

func RegisterExtractor(agentName string, x Extractor) {
	exMu.Lock()
	defer exMu.Unlock()
	exRegs[agentName] = x
}

// Extract resolves via the agent's registered extractor; anything unknown
// falls back to (Other, "") which Decide always asks.
func Extract(agentName, toolName, toolInput string) (Kind, string) {
	exMu.RLock()
	x := exRegs[agentName]
	exMu.RUnlock()
	if x == nil {
		return KindOther, ""
	}
	return x(toolName, toolInput)
}

func init() { RegisterExtractor("claude", claudeExtract) }

func claudeExtract(toolName, toolInput string) (Kind, string) {
	var in struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
		URL          string `json:"url"`
	}
	if err := json.Unmarshal([]byte(toolInput), &in); err != nil {
		return KindOther, ""
	}
	switch toolName {
	case "Bash":
		if in.Command == "" {
			return KindOther, ""
		}
		return KindShell, in.Command
	case "Write", "Edit":
		if in.FilePath == "" {
			return KindOther, ""
		}
		return KindEdit, in.FilePath
	case "NotebookEdit":
		if in.NotebookPath == "" {
			return KindOther, ""
		}
		return KindEdit, in.NotebookPath
	case "Read":
		if in.FilePath == "" {
			return KindOther, ""
		}
		return KindRead, in.FilePath
	case "Glob", "Grep", "LS":
		return KindRead, in.Path // may be "" ⇒ whole-kind semantics on match
	case "WebFetch":
		if in.URL == "" {
			return KindOther, ""
		}
		if u, err := url.Parse(in.URL); err == nil && u.Host != "" {
			return KindFetch, strings.ToLower(u.Hostname())
		}
		return KindOther, ""
	case "WebSearch":
		return KindSearch, ""
	}
	return KindOther, ""
}
