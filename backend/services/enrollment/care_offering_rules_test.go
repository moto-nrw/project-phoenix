package enrollment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// validateOfferingGroupRules is the server-side defense-in-depth check
// for per-group care-offering selection rules. It mirrors the client
// rule enforcement in enrollment-form.tsx.

func offering(id int64, group, rule string) *enrollmentModels.CareOffering {
	o := &enrollmentModels.CareOffering{Name: "O", SelectionGroup: group, SelectionRule: rule}
	o.ID = id
	return o
}

func catalog(offerings ...*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	m := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, o := range offerings {
		m[o.ID] = o
	}
	return m
}

func TestValidateOfferingGroupRules_NoGroups(t *testing.T) {
	open := catalog(
		offering(1, "", enrollmentModels.SelectionRuleOptional),
		offering(2, "", ""),
	)
	children := []SubmitChild{{OfferingIDs: []int64{1, 2}}, {OfferingIDs: nil}}
	assert.NoError(t, validateOfferingGroupRules(children, open))
}

func TestValidateOfferingGroupRules_ExactlyOne(t *testing.T) {
	open := catalog(
		offering(1, "umfang", enrollmentModels.SelectionRuleExactlyOne),
		offering(2, "umfang", enrollmentModels.SelectionRuleExactlyOne),
	)

	// 0 selected → error.
	require.Error(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: nil}}, open))
	// exactly 1 → ok.
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1}}}, open))
	// 2 → error.
	require.Error(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_AtLeastOne(t *testing.T) {
	open := catalog(
		offering(1, "module", enrollmentModels.SelectionRuleAtLeastOne),
		offering(2, "module", enrollmentModels.SelectionRuleAtLeastOne),
	)
	require.Error(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: nil}}, open))
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1}}}, open))
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_AtMostOne(t *testing.T) {
	open := catalog(
		offering(1, "extra", enrollmentModels.SelectionRuleAtMostOne),
		offering(2, "extra", enrollmentModels.SelectionRuleAtMostOne),
	)
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: nil}}, open))
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{2}}}, open))
	require.Error(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1, 2}}}, open))
}

func TestValidateOfferingGroupRules_IgnoresUngroupedAndOtherGroups(t *testing.T) {
	open := catalog(
		offering(1, "umfang", enrollmentModels.SelectionRuleExactlyOne),
		offering(2, "umfang", enrollmentModels.SelectionRuleExactlyOne),
		offering(3, "", enrollmentModels.SelectionRuleOptional), // ungrouped, no constraint
	)
	// One from the exactly-one group + an ungrouped extra → satisfied.
	assert.NoError(t, validateOfferingGroupRules([]SubmitChild{{OfferingIDs: []int64{1, 3}}}, open))
}

func TestValidateOfferingGroupRules_ErrorNamesChildAndWrapsInvalidSubmission(t *testing.T) {
	open := catalog(
		offering(1, "umfang", enrollmentModels.SelectionRuleExactlyOne),
		offering(2, "umfang", enrollmentModels.SelectionRuleExactlyOne),
	)
	children := []SubmitChild{
		{OfferingIDs: []int64{1}},    // ok
		{OfferingIDs: []int64{1, 2}}, // violates exactly_one
	}
	err := validateOfferingGroupRules(children, open)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "child 1")
	assert.True(t, errors.Is(err, ErrCareOfferingRule))
	assert.True(t, errors.Is(err, ErrInvalidSubmission), "maps to 400 via ErrInvalidSubmission")
}
