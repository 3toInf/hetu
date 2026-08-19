package notify

import "testing"

func TestNoopNotifierNeverErrors(t *testing.T) {
	if err := NewNoop().Notify(Notification{Title: "x", Body: "y"}); err != nil {
		t.Fatalf("noop returned %v", err)
	}
}

func TestRecordingNotifierCaptures(t *testing.T) {
	r := &RecordingNotifier{}
	_ = r.Notify(Notification{Title: "T", Body: "B"})
	sent := r.GetSent()
	if len(sent) != 1 || sent[0].Title != "T" {
		t.Fatalf("expected one captured notification, got %+v", sent)
	}
}
