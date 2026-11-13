package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A registration asks for the children of the family only where a care
// sibling rule of the Träger counts them for the school and has not ended
// (#6335, data minimisation).
func TestRegistrationAsksOnlyWhereASiblingRuleCountsTheFamily(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 9)
	ended := calendar.NewDate(2026, 10, 31)
	counting := parentbilling.ContributionRule{Service: parentbilling.ServiceCare, Kind: parentbilling.RuleSibling,
		CountsFamilyChildren: true, SchoolIDs: []int64{7}}
	change := func(edit func(*parentbilling.ContributionRule)) parentbilling.ContributionRule {
		rule := counting
		edit(&rule)
		return rule
	}
	for name, tc := range map[string]struct {
		rule parentbilling.ContributionRule
		asks bool
	}{
		"a rule that counts the family at the school": {counting, true},
		"a rule of another school":                    {change(func(r *parentbilling.ContributionRule) { r.SchoolIDs = []int64{8} }), false},
		"a rule that counts only contracts":           {change(func(r *parentbilling.ContributionRule) { r.CountsFamilyChildren = false }), false},
		"a rule that ended":                           {change(func(r *parentbilling.ContributionRule) { r.ValidUntil = &ended }), false},
		"a rule of another kind":                      {change(func(r *parentbilling.ContributionRule) { r.Kind = "cap" }), false},
	} {
		assert.Equal(t, tc.asks, countsFamilyAt([]parentbilling.ContributionRule{tc.rule}, 7, today), name)
	}
	assert.False(t, countsFamilyAt(nil, 7, today), "no rule asks nothing")
}
