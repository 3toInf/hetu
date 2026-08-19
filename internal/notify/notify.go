package notify

import "sync"

type Notification struct{ Title, Body string }

type Notifier interface{ Notify(n Notification) error }

type NoopNotifier struct{}

func NewNoop() Notifier                       { return NoopNotifier{} }
func (NoopNotifier) Notify(Notification) error { return nil }

// RecordingNotifier is a test double.
type RecordingNotifier struct {
	mu   sync.Mutex
	sent []Notification
}

func (r *RecordingNotifier) Notify(n Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, n)
	return nil
}

// GetSent returns a copy of the sent notifications (thread-safe)
func (r *RecordingNotifier) GetSent() []Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Notification(nil), r.sent...)
}

// NewDesktop returns a notifier using platform desktop tools; missing tool ⇒ Noop.
func NewDesktop() Notifier { return desktopNotifier{} }
