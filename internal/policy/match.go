package policy

import (
	"net/url"
	"path"
	"strings"
)

// specifierValid rejects structurally impossible specifiers per kind.
func specifierValid(k Kind, spec string) bool {
	switch k {
	case KindShell:
		return spec != "" && !strings.ContainsAny(spec, "\n")
	case KindEdit, KindRead:
		return strings.HasPrefix(spec, "/") // absolute paths only (spec §3.2)
	case KindFetch:
		return strings.HasPrefix(spec, "domain:") && len(spec) > len("domain:")
	}
	return false
}

// matchSubject reports whether the rule's specifier matches the subject.
func matchSubject(k Kind, spec, subject string) bool {
	switch k {
	case KindShell:
		if strings.HasSuffix(spec, ":*") {
			return strings.HasPrefix(strings.TrimSpace(subject), strings.TrimSuffix(spec, ":*"))
		}
		return strings.TrimSpace(subject) == spec
	case KindEdit, KindRead:
		return globMatch(spec, subject)
	case KindFetch:
		domain := strings.TrimPrefix(spec, "domain:")
		host := hostOf(subject)
		return host == domain || strings.HasSuffix(host, "."+domain)
	}
	return false
}

// globMatch supports '*' (within one segment) and '**' (across segments),
// matched against an absolute path.
func globMatch(pattern, name string) bool {
	if !strings.HasPrefix(pattern, "/") || !strings.HasPrefix(name, "/") {
		return false
	}
	return segGlob(strings.Split(strings.Trim(pattern, "/"), "/"), strings.Split(strings.Trim(name, "/"), "/"))
}

func segGlob(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		// '**' consumes zero or more segments
		for i := 0; i <= len(name); i++ {
			if segGlob(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if ok, _ := path.Match(pat[0], name[0]); !ok {
		return false
	}
	return segGlob(pat[1:], name[1:])
}

// hostOf extracts the host from a URL-ish subject; returns the subject as-is
// when there is no scheme (callers pass bare hosts for Fetch).
func hostOf(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		return strings.ToLower(s)
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return strings.ToLower(u.Hostname())
	}
	return ""
}
