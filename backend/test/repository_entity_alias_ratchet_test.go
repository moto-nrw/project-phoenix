package test

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var repositoryEntityNameAcronymPattern = regexp.MustCompile(`base\.NewRepository\[[^\]]+\]\([^,\n]+,\s*[^,\n]+,\s*"[^"]*[A-Z]{2,}[^"]*"\)`)

// TestRepositoryEntityAliasRatchet guards the generic repository alias
// derivation against acronym-cased EntityName strings. base.Repository derives
// SQL aliases with a naive snake-caser, so "MFACredential" becomes
// "m_f_a_credential" while bun's model alias is "mfa_credential". Use
// alias-safe casing like "MfaCredential", "RfidCard", or
// "OperatorMfaEmailChallenge" instead.
func TestRepositoryEntityAliasRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := scanRatchetPattern(backendRoot, "database/repositories", repositoryEntityNameAcronymPattern, func(string) bool {
		return false
	})
	if err != nil {
		t.Fatalf("scanning repository EntityName literals failed: %v", err)
	}

	var violations []string
	for file, count := range counts {
		violations = append(violations, fmt.Sprintf(
			"%s: %d acronym-cased EntityName literal(s).\n"+
				"  Use alias-safe casing such as MfaCredential, RfidCard, or OperatorMfaEmailChallenge.",
			file, count))
	}
	if len(violations) == 0 {
		return
	}

	sort.Strings(violations)
	t.Errorf("Repository EntityName alias ratchet failed (%d file(s)):\n\n%s",
		len(violations), strings.Join(violations, "\n\n"))
}
