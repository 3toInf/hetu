package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/3toInf/hetu/internal/policy"
)

func TestAllowAddRmList(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	t.Setenv("HETU_RULES", rulesPath)
	seed := policy.Rules{Allow: []string{"Read"}}
	policy.Save(rulesPath, seed)

	run := func(args ...string) (string, error) {
		cmd := NewRootCmd()
		cmd.SetArgs(append([]string{"allow"}, args...))
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		err := cmd.Execute()
		return out.String(), err
	}

	if _, err := run("add", "Shell(git status:*)"); err != nil {
		t.Fatal(err)
	}
	if _, err := run("add", "Shell(git push:*)", "--deny"); err != nil {
		t.Fatal(err)
	}
	got, err := policy.Load(rulesPath)
	if err != nil || len(got.Allow) != 2 || len(got.Deny) != 1 {
		t.Fatalf("file after add: %+v %v", got, err)
	}

	out, err := run("list")
	if err != nil || !strings.Contains(out, "allow") || !strings.Contains(out, "Shell(git status:*)") || !strings.Contains(out, "deny") {
		t.Fatalf("list output: %q %v", out, err)
	}

	if _, err := run("rm", "Shell(git status:*)"); err != nil {
		t.Fatal(err)
	}
	got, _ = policy.Load(rulesPath)
	if len(got.Allow) != 1 {
		t.Fatalf("rm did not remove: %+v", got)
	}

	if _, err := run("add", "Bogus(x)"); err == nil {
		t.Fatal("invalid rule must be rejected")
	}
	if _, err := run("rm", "Shell(nope:*)"); err == nil {
		t.Fatal("removing a non-existent rule must error")
	}
}

func TestAllowAddDenyRoundTrip(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	t.Setenv("HETU_RULES", rulesPath)
	if err := policy.Save(rulesPath, policy.DefaultRules()); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		cmd := NewRootCmd()
		cmd.SetArgs(append([]string{"allow"}, args...))
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		err := cmd.Execute()
		return out.String(), err
	}

	// add X --deny must store !X so the rule actually survives NewPolicy
	// (a bare rule in the deny array is skipped as effect-mismatched).
	if _, err := run("add", "Shell(git push:*)", "--deny"); err != nil {
		t.Fatal(err)
	}
	rs, err := policy.Load(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Deny) != 1 || rs.Deny[0] != "!Shell(git push:*)" {
		t.Fatalf("deny after add --deny = %#v, want [!Shell(git push:*)]", rs.Deny)
	}
	p, parseErrs := policy.NewPolicy(rs)
	if len(parseErrs) != 0 {
		t.Fatalf("NewPolicy returned parse errors: %v", parseErrs)
	}
	if d := p.Decide(policy.KindShell, "git push"); d != policy.DecisionDeny {
		t.Fatalf("Decide(Shell, git push) = %v, want DecisionDeny", d)
	}

	// add "!X" --deny is idempotent: stores exactly !X.
	if _, err := run("add", "!Shell(rm -rf *)", "--deny"); err != nil {
		t.Fatal(err)
	}
	rs, _ = policy.Load(rulesPath)
	if len(rs.Deny) != 2 || rs.Deny[1] != "!Shell(rm -rf *)" {
		t.Fatalf("deny after add '!X' --deny = %#v, want second entry !Shell(rm -rf *)", rs.Deny)
	}

	// add "!X" without --deny must be rejected, not silently saved as a
	// deny-effect rule in the allow array (which NewPolicy would skip).
	if _, err := run("add", "!Shell(touch /tmp/hetu-x)"); err == nil {
		t.Fatal("add '!X' without --deny must be rejected")
	}
	// Whitespace before '!' is still parsed as a deny rule.
	if _, err := run("add", " !Shell(touch /tmp/hetu-y)"); err == nil {
		t.Fatal("add ' !X' without --deny must be rejected")
	}
	rs, _ = policy.Load(rulesPath)
	if _, parseErrs := policy.NewPolicy(rs); len(parseErrs) != 0 {
		t.Fatalf("allow list must contain only clean rules, got parse errors: %v", parseErrs)
	}
	for _, a := range rs.Allow {
		if strings.HasPrefix(a, "!") {
			t.Fatalf("allow list must not contain '!'-prefixed rule, got %q", a)
		}
	}

	// rm X --deny: the user types the bare X, the file stores !X.
	if _, err := run("rm", "Shell(git push:*)", "--deny"); err != nil {
		t.Fatal(err)
	}
	rs, _ = policy.Load(rulesPath)
	if len(rs.Deny) != 1 || rs.Deny[0] != "!Shell(rm -rf *)" {
		t.Fatalf("deny after rm --deny = %#v, want [!Shell(rm -rf *)]", rs.Deny)
	}
}
