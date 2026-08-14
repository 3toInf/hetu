//go:build darwin

package notify

import (
	"os/exec"
)

type desktopNotifier struct{}

func (d desktopNotifier) Notify(n Notification) error {
	// Try terminal-notifier first (more user-friendly)
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		_ = exec.Command(path, "-title", n.Title, "-message", n.Body).Run()
		return nil
	}

	// Fall back to osascript
	if _, err := exec.LookPath("osascript"); err != nil {
		// osascript not available, silently degrade to no-op
		return nil
	}

	// Use osascript for system notifications
	_ = exec.Command("osascript", "-e", `display notification "` + n.Body + `" with title "` + n.Title + `"`).Run()
	return nil
}
