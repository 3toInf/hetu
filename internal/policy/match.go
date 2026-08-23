// Task 2 fills in real matchers

package policy

import (
	"strings"
)

// specifierValid returns true if the specifier is valid for the given kind.
// Stub: returns true for all valid kinds with non-empty specifier.
func specifierValid(k Kind, spec string) bool {
	if spec == "" {
		return false
	}
	switch k {
	case KindShell, KindEdit, KindRead, KindFetch, KindSearch:
		return true
	default:
		return false
	}
}

// matchSubject returns true if the subject matches the specifier for the given kind.
// Stub: implements prefix match for Shell kind with "pattern:*" syntax.
func matchSubject(k Kind, spec, subject string) bool {
	if k != KindShell {
		return false
	}
	// Handle "pattern:*" syntax used in tests
	if strings.HasSuffix(spec, ":*") {
		prefix := strings.TrimSuffix(spec, ":*")
		return subject == prefix || strings.HasPrefix(subject, prefix+" ")
	}
	// Otherwise exact match
	return subject == spec
}
