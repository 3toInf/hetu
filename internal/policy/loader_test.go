package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoaderHotReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	_ = Save(path, Rules{Allow: []string{"Shell(echo:*)"}})
	l := NewLoader(path)

	if d, _, _, err := l.Decide(KindShell, "echo hi"); err != nil || d != DecisionAllow {
		t.Fatalf("initial: %v %v", d, err)
	}
	if d, _, _, _ := l.Decide(KindShell, "touch x"); d != DecisionAsk {
		t.Fatalf("unruled command should ask, got %v", d)
	}

	// rewrite; mtime must change (explicit future mtime for fast testdata)
	newRules := Rules{Allow: []string{"Shell(echo:*)", "Shell(touch:*)"}}
	_ = Save(path, newRules)
	os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second))

	if d, _, _, err := l.Decide(KindShell, "touch x"); err != nil || d != DecisionAllow {
		t.Fatalf("after reload: %v %v", d, err)
	}
}

func TestLoaderBadFileKeepsLastGood(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	_ = Save(path, Rules{Allow: []string{"Read"}})
	l := NewLoader(path)

	// establish a last-good policy before corrupting the file
	if d, _, _, err := l.Decide(KindRead, "/x"); err != nil || d != DecisionAllow {
		t.Fatalf("initial load: %v %v", d, err)
	}

	os.WriteFile(path, []byte("{bad"), 0o600)
	os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second))
	d, _, _, err := l.Decide(KindRead, "/x")
	if err == nil {
		t.Fatal("expected load error surfaced")
	}
	if d != DecisionAllow {
		t.Fatal("last-good policy must keep serving")
	}
}

func TestLoaderMissingFileAsksEverything(t *testing.T) {
	l := NewLoader(filepath.Join(t.TempDir(), "nope.json"))
	if d, _, _, _ := l.Decide(KindRead, "/x"); d != DecisionAsk {
		t.Fatal("no file ⇒ ask everything")
	}
}

func TestLoaderSeedDefaultsBeforeFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	l := NewLoader(path)
	p, _ := NewPolicy(Rules{Allow: []string{"Read"}})
	l.Seed(p)

	// before any file exists, the seeded policy serves (stat error is
	// surfaced but the decision still comes from the seeded policy)
	if d, _, _, _ := l.Decide(KindRead, "/x"); d != DecisionAllow {
		t.Fatalf("seeded default should allow Read, got %v", d)
	}

	// a real file still takes over on the next Decide (Seed resets mtime)
	_ = Save(path, Rules{Allow: []string{"Edit"}})
	os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second))
	if d, _, _, err := l.Decide(KindEdit, "/y"); err != nil || d != DecisionAllow {
		t.Fatalf("file should override seed: %v %v", d, err)
	}
	if d, _, _, _ := l.Decide(KindRead, "/x"); d != DecisionAsk {
		t.Fatalf("seed rule must not persist after file load, got %v", d)
	}
}

func TestLoaderSurfacesParseSkips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	// bad rule line is skipped; good rule still applies
	_ = Save(path, Rules{Allow: []string{"Read", "Bogus(x)"}})
	l := NewLoader(path)
	d, _, _, err := l.Decide(KindRead, "/x")
	if d != DecisionAllow {
		t.Fatalf("good rule must still apply, got %v", d)
	}
	if err == nil {
		t.Fatal("parse-skip lines must surface as an error")
	}
	if !strings.Contains(err.Error(), "Bogus") {
		t.Fatalf("error should mention skipped line, got %v", err)
	}
}

func TestLoaderCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	_ = Save(path, Rules{Allow: []string{"Read"}})
	l := NewLoader(path)
	p, err := l.Current()
	if err != nil || p == nil {
		t.Fatalf("Current: %v %v", p, err)
	}
	if d := p.Decide(KindRead, "/x"); d != DecisionAllow {
		t.Fatalf("current policy should allow Read, got %v", d)
	}
}
