package selection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateOfferingGroupRules is the server-side defense-in-depth check
// for per-group care-offering selection rules. It mirrors the client
// rule enforcement in enrollment-form.tsx.

func offering(id int64, group, rule string) *Offering {
	o := &Offering{SelectionGroup: group, SelectionRule: rule}
	o.ID = id
	return o
}

func catalog(offerings ...*Offering) map[int64]*Offering {
	m := make(map[int64]*Offering, len(offerings))
	for _, o := range offerings {
		m[o.ID] = o
	}
	return m
}

func TestValidateOfferingGroupRules_NoGroups(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "", selectionRuleOptional),
		offering(2, "", ""),
	)
	children := []Child{{OfferingIDs: []int64{1, 2}}, {OfferingIDs: nil}}
	assert.NoError(t, validateOfferingGroupRules(children, open))
}

func TestValidateOfferingGroupRules_ExactlyOne(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "umfang", selectionRuleExactlyOne),
		offering(2, "umfang", selectionRuleExactlyOne),
	)

	// 0 selected → error.
	require.Error(t, validateOfferingGroupRules([]Child{{OfferingIDs: nil}}, open))
	// exactly 1 → ok.
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1}}}, open))
	// 2 → error.
	require.Error(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_AtLeastOne(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "module", selectionRuleAtLeastOne),
		offering(2, "module", selectionRuleAtLeastOne),
	)
	require.Error(t, validateOfferingGroupRules([]Child{{OfferingIDs: nil}}, open))
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1}}}, open))
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_AtMostOne(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "extra", selectionRuleAtMostOne),
		offering(2, "extra", selectionRuleAtMostOne),
	)
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: nil}}, open))
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{2}}}, open))
	require.Error(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_IgnoresUngroupedAndOtherGroups(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "umfang", selectionRuleExactlyOne),
		offering(2, "umfang", selectionRuleExactlyOne),
		offering(3, "", selectionRuleOptional), // ungrouped, no constraint
	)
	// One from the exactly-one group + an ungrouped extra → satisfied.
	assert.NoError(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1, 3}}}, open))
}

func TestValidateOfferingGroupRules_ErrorNamesChildAndWrapsInvalidSubmission(t *testing.T) {
	t.Parallel()

	open := catalog(
		offering(1, "umfang", selectionRuleExactlyOne),
		offering(2, "umfang", selectionRuleExactlyOne),
	)
	children := []Child{
		{OfferingIDs: []int64{1}},    // ok
		{OfferingIDs: []int64{1, 2}}, // violates exactly_one
	}
	err := validateOfferingGroupRules(children, open)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "child 1")
	assert.True(t, errors.Is(err, ErrCareOfferingRule))
	assert.True(t, errors.Is(err, ErrInvalidSubmission), "maps to 400 via ErrInvalidSubmission")
}

func TestValidateOfferingGroupRules_RejectsMixedRulesInGroup(t *testing.T) {
	t.Parallel()

	// Two offerings in the same group declare different non-optional rules.
	// This is an admin misconfiguration: the chosen rule must not depend on
	// map iteration order, so we reject it deterministically rather than
	// silently picking one.
	open := catalog(
		offering(1, "umfang", selectionRuleExactlyOne),
		offering(2, "umfang", selectionRuleAtLeastOne),
	)
	err := validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1}}}, open)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting rules")
	assert.True(t, errors.Is(err, ErrCareOfferingRule))
}

func TestValidateOfferingGroupRules_DeterministicAcrossManyRuns(t *testing.T) {
	t.Parallel()

	// Same conflicting catalog evaluated repeatedly must always error (never
	// flip to "valid" depending on map iteration order).
	for i := 0; i < 50; i++ {
		open := catalog(
			offering(1, "umfang", selectionRuleExactlyOne),
			offering(2, "umfang", selectionRuleAtMostOne),
		)
		require.Error(t, validateOfferingGroupRules([]Child{{OfferingIDs: []int64{1}}}, open))
	}
}
