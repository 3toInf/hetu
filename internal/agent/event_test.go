package agent

import "testing"

func TestEventTextPayload(t *testing.T) {
	e := Event{Type: EventText, Text: "hello"}
	if e.Type.String() != "text" {
		t.Fatalf("got %q", e.Type.String())
	}
}
