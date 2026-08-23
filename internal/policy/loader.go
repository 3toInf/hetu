package policy

import (
	"errors"
	"os"
	"sync"
	"time"
)

// Loader caches the parsed rules and reloads when the file mtime changes.
type Loader struct {
	path string

	mu        sync.Mutex
	pol       *Policy
	modTime   time.Time
	lastErr   error
	parseErrs []error
}

// NewLoader returns a Loader watching path. Until the file is first read
// successfully the in-memory policy is empty, so everything asks.
func NewLoader(path string) *Loader {
	return &Loader{path: path, pol: &Policy{}} // empty ⇒ ask everything
}

// Seed installs an in-memory policy (e.g. the daemon's default rules) and
// resets the reload state so the next Decide/Current refreshes from the file.
func (l *Loader) Seed(p *Policy) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pol = p
	l.modTime = time.Time{} // zero ⇒ the next file stat always reloads
	l.lastErr = nil
	l.parseErrs = nil
}

// Current returns the freshest policy, reloading when the file changed.
// On a load error it returns the last-good policy and the error (caller logs).
func (l *Loader) Current() (*Policy, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.reloadIfChanged(); err != nil {
		return l.pol, err
	}
	return l.pol, nil
}

// Decide evaluates under the freshest policy the file can provide. The error
// return surfaces a load failure (the decision still comes from the last-good
// policy) as well as any bad rule lines skipped by the most recent successful
// parse, joined via errors.Join.
func (l *Loader) Decide(kind Kind, subject string) (Decision, Rule, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.reloadIfChanged(); err != nil {
		// keep last-good; surface the error for logging
		d := l.pol.Decide(kind, subject)
		r, ok := l.pol.MatchedRule(kind, subject)
		return d, r, ok, err
	}
	d := l.pol.Decide(kind, subject)
	r, ok := l.pol.MatchedRule(kind, subject)
	return d, r, ok, errors.Join(l.parseErrs...)
}

func (l *Loader) reloadIfChanged() error {
	fi, err := os.Stat(l.path)
	if err != nil {
		l.lastErr = err
		return err // no file: keep current (initially empty ⇒ ask-all)
	}
	if !fi.ModTime().After(l.modTime) && l.pol != nil {
		return nil
	}
	rs, err := Load(l.path)
	if err != nil {
		l.lastErr = err
		return err
	}
	p, parseErrs := NewPolicy(rs)
	l.pol = p
	l.modTime = fi.ModTime()
	l.lastErr = nil
	l.parseErrs = parseErrs
	return nil
}
