package claude

import (
	"context"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
)

func TestDiscoverYieldsSessions(t *testing.T) {
	d := &Discovery{ProjectsDir: "testdata"}
	ch, err := d.Discover(context.Background(), DiscoverOptsAll)
	if err != nil {
		t.Fatal(err)
	}
	var got []agent.DiscoveredSession
	for s := range ch {
		got = append(got, s)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 session, got %d", len(got))
	}
	s := got[0]
	if s.ExternalID != "a1" {
		t.Errorf("ExternalID=%q want a1", s.ExternalID)
	}
	if s.CWD != "/home/u/code/alpha" {
		t.Errorf("CWD=%q want /home/u/code/alpha", s.CWD)
	}
	if s.Title != "Refactor auth" {
		t.Errorf("Title=%q want 'Refactor auth'", s.Title)
	}
	if s.MessageCount != 2 {
		t.Errorf("MessageCount=%d want 2", s.MessageCount)
	}
}
