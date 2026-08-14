//go:build linux

package notify

import (
	"os/exec"
)

type desktopNotifier struct{}

func (d desktopNotifier) Notify(n Notification) error {
	// Check if notify-send is available
	if _, err := exec.LookPath("notify-send"); err != nil {
		// notify-send not available, silently degrade to no-op
		return nil
	}

	// Try to send notification, but never return an error
	_ = exec.Command("notify-send", n.Title, n.Body).Run()
	return nil
}
