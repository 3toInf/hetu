package policy

import "testing"

func TestClaudeExtract(t *testing.T) {
	cases := []struct {
		tool, in string
		wantKind Kind
		wantSubj string
	}{
		{"Bash", `{"command":"git status"}`, KindShell, "git status"},
		{"Write", `{"file_path":"/a/b.go"}`, KindEdit, "/a/b.go"},
		{"NotebookEdit", `{"notebook_path":"/n.ipynb"}`, KindEdit, "/n.ipynb"},
		{"Read", `{"file_path":"/a"}`, KindRead, "/a"},
		{"Glob", `{"path":"/repo"}`, KindRead, "/repo"},
		{"WebFetch", `{"url":"https://api.github.com/x"}`, KindFetch, "api.github.com"},
		{"WebSearch", `{"query":"x"}`, KindSearch, ""},
		{"Mystery", `{"x":1}`, KindOther, ""},
		{"Bash", `{not json`, KindOther, ""}, // bad JSON ⇒ Other
		{"Bash", `{}`, KindOther, ""},        // missing field ⇒ Other
		{"Task", `{"prompt":"x"}`, KindOther, ""},
	}
	for _, c := range cases {
		k, s := Extract("claude", c.tool, c.in)
		if k != c.wantKind || s != c.wantSubj {
			t.Errorf("Extract(claude,%s,%s) = (%s,%q) want (%s,%q)", c.tool, c.in, k, s, c.wantKind, c.wantSubj)
		}
	}
}

func TestExtractUnknownAgent(t *testing.T) {
	if k, _ := Extract("nope", "Bash", `{"command":"ls"}`); k != KindOther {
		t.Fatalf("unknown agent must yield Other, got %s", k)
	}
}

// Bare-kind rules (no specifier) match any subject, including the empty subject
// produced by Glob/Grep/LS when the input has no "path" field.
func TestDecideBareKindReadAllowsEmptySubject(t *testing.T) {
	p, errs := NewPolicy(Rules{Allow: []string{"Read"}})
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if got := p.Decide(KindRead, ""); got != DecisionAllow {
		t.Errorf("Decide(Read,\"\") = %v, want %v", got, DecisionAllow)
	}
}
