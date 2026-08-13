package session

import "testing"

func TestStatusStringAndParse(t *testing.T) {
	cases := []struct {
		in   Status
		want string
	}{
		{StatusRunning, "Running"}, {StatusCompleted, "Completed"},
		{StatusError, "Error"}, {StatusIdle, "Idle"}, {StatusUnknown, "Unknown"},
	}
	for _, c := range cases {
		if c.in.String() != c.want {
			t.Errorf("%v.String() = %q want %q", c.in, c.in.String(), c.want)
		}
		got, ok := ParseStatus(c.want)
		if !ok || got != c.in {
			t.Errorf("ParseStatus(%q) = %v,%v want %v,true", c.want, got, ok, c.in)
		}
	}
	if _, ok := ParseStatus("Nonsense"); ok {
		t.Error("ParseStatus should reject unknown values")
	}
}

func TestParseStatusWaitingStates(t *testing.T) {
	for _, s := range []Status{StatusWaitingForApproval, StatusWaitingForInput} {
		got, ok := ParseStatus(string(s))
		if !ok || got != s {
			t.Fatalf("ParseStatus(%q) = %q,%v; want %q,true", s, got, ok, s)
		}
	}
}
