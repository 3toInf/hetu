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
