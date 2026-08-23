package policy

import "testing"

func TestParseValid(t *testing.T) {
	cases := []struct{ in string; want Rule }{
		{"Read", Rule{KindRead, "", EffectAllow}},
		{"Shell(git status:*)", Rule{KindShell, "git status:*", EffectAllow}},
		{"!Shell(git push:*)", Rule{KindShell, "git push:*", EffectDeny}},
		{"Fetch(domain:github.com)", Rule{KindFetch, "domain:github.com", EffectAllow}},
		{"Edit(/repo/**)", Rule{KindEdit, "/repo/**", EffectAllow}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, bad := range []string{"", "Nope", "Shell(", "Shell)", "Shell()", "Read(", "!"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should error", bad)
		}
	}
}

func TestDecidePrecedence(t *testing.T) {
	p, errs := NewPolicy(Rules{
		Allow: []string{"Shell(git status:*)"},
		Deny:  []string{"!Shell(git push:*)", "!Read"},
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	cases := []struct {
		kind Kind; subject string; want Decision
	}{
		{KindShell, "git push origin main", DecisionDeny},  // deny beats ask
		{KindShell, "git status", DecisionAllow},            // allow rule
		{KindShell, "rm -rf /", DecisionAsk},                // no rule => ask
		{KindRead, "/etc/passwd", DecisionDeny},             // !Read whole-kind deny
		{KindFetch, "github.com", DecisionAsk},
	}
	for _, c := range cases {
		if got := p.Decide(c.kind, c.subject); got != c.want {
			t.Errorf("Decide(%s,%q) = %v, want %v", c.kind, c.subject, got, c.want)
		}
	}
}

func TestNewPolicySkipsBadLines(t *testing.T) {
	p, errs := NewPolicy(Rules{Allow: []string{"Read", "Bogus(x)"}})
	if len(errs) != 1 || len(p.rules) != 1 {
		t.Fatalf("want 1 err + 1 rule, got errs=%v rules=%d", errs, len(p.rules))
	}
}
