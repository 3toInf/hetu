package notify

type Notification struct{ Title, Body string }

type Notifier interface{ Notify(n Notification) error }

type NoopNotifier struct{}

func NewNoop() Notifier                       { return NoopNotifier{} }
func (NoopNotifier) Notify(Notification) error { return nil }

// RecordingNotifier is a test double.
type RecordingNotifier struct{ Sent []Notification }
func (r *RecordingNotifier) Notify(n Notification) error { r.Sent = append(r.Sent, n); return nil }

// NewDesktop returns a notifier using platform desktop tools; missing tool ⇒ Noop.
func NewDesktop() Notifier { return desktopNotifier{} }
