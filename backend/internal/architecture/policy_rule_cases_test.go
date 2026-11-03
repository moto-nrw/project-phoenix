package architecture

import (
	"slices"
	"strings"
	"testing"
)

// The epoch tables assert named rule diagnostics, which only ruleLoosenings
// emits. Compare the complete policies once with all extra rules, retaining
// the production dispatch and every existing diagnostic assertion. Also check
// each rule alone: another invalid rule must not be what rejects this case.
// All other strictness guards, including the expensive package-pair import
// comparison, still run in the integrated comparison rather than per case.
func assertIsolatedRuleLoosening(t *testing.T, base, candidate *Policy, rule Rule) {
	t.Helper()
	widened := *candidate
	widened.Rules = append(slices.Clone(candidate.Rules), rule)
	problems := ruleLoosenings(base, &widened, candidateOnlyRolePoints(&widened, nil))
	if !strings.Contains(strings.Join(problems, "\n"), "rule "+rule.ID+" ") {
		t.Fatalf("isolated rule %s escaped the guard: %v", rule.ID, problems)
	}
}
