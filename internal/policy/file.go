package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func DefaultRules() Rules { return Rules{Allow: []string{"Read"}} }

// Load reads and JSON-decodes the rules file. Missing file and bad JSON are
// errors; the daemon decides how to degrade (seed defaults / ask-everything).
func Load(path string) (Rules, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Rules{}, err
	}
	var rs Rules
	if err := json.Unmarshal(b, &rs); err != nil {
		return Rules{}, fmt.Errorf("rules %s: %w", path, err)
	}
	return rs, nil
}

// Save writes atomically (temp file + rename in the same dir).
func Save(path string, rs Rules) error {
	b, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
