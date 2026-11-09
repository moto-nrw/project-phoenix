package owners

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The registration asks for the children of the family only where a current
// care sibling rule of the Träger counts them for the school (#6335).
func TestRegistrationAsksOnlyWhereASiblingRuleCountsTheFamily(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 9)
	ended := calendar.NewDate(2026, 10, 31)
	counting := parentbilling.ContributionRule{Service: parentbilling.ServiceCare, Kind: parentbilling.RuleSibling,
		CountsFamilyChildren: true, SchoolIDs: []int64{7}}
	for name, tc := range map[string]struct {
		rule parentbilling.ContributionRule
		asks bool
	}{
		"a rule that counts the family at the school": {counting, true},
		"a rule of another school":                    {withSchools(counting, 8), false},
		"a rule that counts only contracts":           {withoutFamily(counting), false},
		"a rule that ended":                           {withEnd(counting, ended), false},
		"a holiday rule":                              {withService(counting, parentbilling.ServiceHoliday), false},
	} {
		assert.Equal(t, tc.asks, countsFamilyAt([]parentbilling.ContributionRule{tc.rule}, 7, today), name)
	}
	assert.False(t, countsFamilyAt(nil, 7, today), "no rule asks nothing")
}

func withSchools(rule parentbilling.ContributionRule, ids ...int64) parentbilling.ContributionRule {
	rule.SchoolIDs = ids
	return rule
}

func withoutFamily(rule parentbilling.ContributionRule) parentbilling.ContributionRule {
	rule.CountsFamilyChildren = false
	return rule
}

func withEnd(rule parentbilling.ContributionRule, end calendar.Date) parentbilling.ContributionRule {
	rule.ValidUntil = &end
	return rule
}

func withService(rule parentbilling.ContributionRule, service string) parentbilling.ContributionRule {
	rule.Service = service
	return rule
}
