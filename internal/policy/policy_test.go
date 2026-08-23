package policy

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestNewPolicyRejectsConflictingEffects(t *testing.T) {
	_, errs := NewPolicy(Rules{Allow: []string{"!Read"}})
	if len(errs) != 1 {
		t.Errorf("want 1 error for !Read in Allow list, got %d", len(errs))
	}
	_, errs2 := NewPolicy(Rules{Deny: []string{"Read"}})
	if len(errs2) != 1 {
		t.Errorf("want 1 error for Read in Deny list, got %d", len(errs2))
	}
}

func TestParseRejectsKindOther(t *testing.T) {
	if _, err := Parse("Other"); err == nil {
		t.Error(`Parse("Other") should error`)
	}
	if _, err := Parse("!Other(x)"); err == nil {
		t.Error(`Parse("!Other(x)") should error`)
	}
}

func TestMatchShellPrefix(t *testing.T) {
	p, _ := NewPolicy(Rules{Allow: []string{"Shell(git status:*)", "Shell(npm test)"}})
	cases := []struct{ subj string; want Decision }{
		{"git status", DecisionAllow},
		{"git status --short", DecisionAllow},
		{"git  status", DecisionAsk},       // prefix is byte-wise, no whitespace folding beyond TrimSpace of the subject
		{"npm test", DecisionAllow},        // exact
		{"npm test -- --watch", DecisionAsk}, // exact ≠ prefix
		{"npm tests", DecisionAsk},
	}
	for _, c := range cases {
		if got := p.Decide(KindShell, c.subj); got != c.want {
			t.Errorf("Shell %q = %v want %v", c.subj, got, c.want)
		}
	}
}

func TestMatchPathGlob(t *testing.T) {
	p, _ := NewPolicy(Rules{Allow: []string{"Edit(/home/me/repo/**)", "Read(/etc/*.conf)"}})
	cases := []struct{ kind Kind; subj string; want Decision }{
		{KindEdit, "/home/me/repo/a.go", DecisionAllow},
		{KindEdit, "/home/me/repo/sub/b.go", DecisionAllow}, // ** crosses segments
		{KindEdit, "/home/me/other.go", DecisionAsk},
		{KindRead, "/etc/hosts", DecisionAsk},               // *.conf doesn't match hosts
		{KindRead, "/etc/resolv.conf", DecisionAllow},       // * within a segment
		{KindEdit, "repo/a.go", DecisionAsk},                // relative subject never matches absolute rule
	}
	for _, c := range cases {
		if got := p.Decide(c.kind, c.subj); got != c.want {
			t.Errorf("%s %q = %v want %v", c.kind, c.subj, got, c.want)
		}
	}
}

func TestMatchDomain(t *testing.T) {
	p, _ := NewPolicy(Rules{Allow: []string{"Fetch(domain:github.com)"}})
	cases := []struct{ subj string; want Decision }{
		{"github.com", DecisionAllow},
		{"api.github.com", DecisionAllow},   // subdomains included
		{"notgithub.com", DecisionAsk},      // no suffix cheat
		{"gist.github.com.evil.io", DecisionAsk},
	}
	for _, c := range cases {
		if got := p.Decide(KindFetch, c.subj); got != c.want {
			t.Errorf("Fetch %q = %v want %v", c.subj, got, c.want)
		}
	}
}

func TestDefaultRules(t *testing.T) {
	if d := DefaultRules(); len(d.Allow) != 1 || d.Allow[0] != "Read" || len(d.Deny) != 0 {
		t.Fatalf("DefaultRules = %+v", d)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	rs := Rules{Allow: []string{"Read", "Shell(git status:*)"}, Deny: []string{"Shell(git push:*)"}}
	if err := Save(path, rs); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got.Allow) != 2 || got.Deny[0] != "Shell(git push:*)" {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("missing file must error (caller decides to seed defaults)")
	}
}

func TestLoadBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := Load(path); err == nil {
		t.Fatal("bad JSON must error (daemon degrades to ask-everything)")
	}
}
