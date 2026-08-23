// Package policy parses and evaluates hetu approval rules.
// Rules act on canonical tool kinds (agent-agnostic); per-agent extractors
// map native tools to (kind, subject). See the v0.3 spec §3.
package policy

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindShell  Kind = "Shell"
	KindEdit   Kind = "Edit"
	KindRead   Kind = "Read"
	KindFetch  Kind = "Fetch"
	KindSearch Kind = "Search"
	KindOther  Kind = "Other"
)

func kindValid(k Kind) bool {
	switch k {
	case KindShell, KindEdit, KindRead, KindFetch, KindSearch:
		return true
	}
	return false // Other is never a valid rule target — it always asks
}

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Rule struct {
	Kind      Kind
	Specifier string // "" = whole kind
	Effect    Effect
}

func (r Rule) String() string {
	s := string(r.Kind)
	if r.Specifier != "" {
		s += "(" + r.Specifier + ")"
	}
	if r.Effect == EffectDeny {
		s = "!" + s
	}
	return s
}

type Decision int

const (
	DecisionAsk Decision = iota
	DecisionAllow
	DecisionDeny
)

// Parse accepts "[!]Kind" or "[!]Kind(specifier)".
func Parse(rule string) (Rule, error) {
	s := strings.TrimSpace(rule)
	if s == "" {
		return Rule{}, fmt.Errorf("empty rule")
	}
	effect := EffectAllow
	if strings.HasPrefix(s, "!") {
		effect = EffectDeny
		s = strings.TrimSpace(strings.TrimPrefix(s, "!"))
		if s == "" {
			return Rule{}, fmt.Errorf("rule %q: '!' with no rule", rule)
		}
	}
	kind, spec := s, ""
	if i := strings.IndexByte(s, '('); i >= 0 {
		if !strings.HasSuffix(s, ")") {
			return Rule{}, fmt.Errorf("rule %q: unclosed '('", rule)
		}
		kind, spec = s[:i], s[i+1:len(s)-1]
		if spec == "" {
			return Rule{}, fmt.Errorf("rule %q: empty specifier", rule)
		}
	} else if strings.ContainsAny(s, ")") {
		return Rule{}, fmt.Errorf("rule %q: unexpected ')'", rule)
	}
	kind = strings.TrimSpace(kind)
	spec = strings.TrimSpace(spec)
	k := Kind(kind)
	if !kindValid(k) {
		return Rule{}, fmt.Errorf("rule %q: unknown kind %q (kinds: Shell/Edit/Read/Fetch/Search)", rule, kind)
	}
	if k == KindSearch && spec != "" {
		return Rule{}, fmt.Errorf("rule %q: Search takes no specifier", rule)
	}
	if spec != "" && !specifierValid(k, spec) {
		return Rule{}, fmt.Errorf("rule %q: bad %s specifier %q", rule, k, spec)
	}
	return Rule{Kind: k, Specifier: spec, Effect: effect}, nil
}

type Rules struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

type Policy struct {
	rules []Rule
}

// NewPolicy parses all rule lines; unparseable lines are skipped and returned.
func NewPolicy(rs Rules) (*Policy, []error) {
	var errs []error
	p := &Policy{}
	add := func(lines []string, eff Effect) {
		for _, l := range lines {
			r, err := Parse(l)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if r.Effect != eff { // line carried its own '!' that disagrees with list
				errs = append(errs, fmt.Errorf("rule %q in %s list carries '%s' effect", l, eff, r.Effect))
				continue
			}
			p.rules = append(p.rules, r)
		}
	}
	add(rs.Deny, EffectDeny)   // deny first: Decide scans in order, deny wins ties
	add(rs.Allow, EffectAllow)
	return p, errs
}

// Decide: deny rules beat allow rules; no hit ⇒ Ask.
func (p *Policy) Decide(kind Kind, subject string) Decision {
	_, r, ok := p.match(kind, subject)
	if !ok {
		return DecisionAsk
	}
	if r.Effect == EffectDeny {
		return DecisionDeny
	}
	return DecisionAllow
}

// MatchedRule returns the fired rule (for logging).
func (p *Policy) MatchedRule(kind Kind, subject string) (Rule, bool) {
	_, r, ok := p.match(kind, subject)
	return r, ok
}

func (p *Policy) match(kind Kind, subject string) (int, Rule, bool) {
	for i, r := range p.rules {
		if r.Kind != kind {
			continue
		}
		if r.Specifier == "" || matchSubject(r.Kind, r.Specifier, subject) {
			return i, r, true
		}
	}
	return 0, Rule{}, false
}
