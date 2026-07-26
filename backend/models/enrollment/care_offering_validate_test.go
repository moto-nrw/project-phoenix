package enrollment

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure tests for CareOffering.Validate + HasUnlimitedCapacity.
// CareOffering.Validate runs at admin-create + admin-update time;
// missing/wrong values land as a 400 in the admin UI rather than as
// a Postgres CHECK violation.

func validCareOffering() *CareOffering {
	return &CareOffering{
		Name:           "OGS-Nachmittag",
		PhaseID:        42,
		DaysOfWeekMode: DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
	}
}

func TestCareOffering_Validate_HappyPath(t *testing.T) {
	assert.NoError(t, validCareOffering().Validate())
}

func TestCareOffering_Validate_RequiresName(t *testing.T) {
	c := validCareOffering()
	c.Name = "  "
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestCareOffering_Validate_TrimsName(t *testing.T) {
	c := validCareOffering()
	c.Name = "  Spätbetreuung  "
	require.NoError(t, c.Validate())
	assert.Equal(t, "Spätbetreuung", c.Name)
}

func TestCareOffering_Validate_RequiresPhaseID(t *testing.T) {
	c := validCareOffering()
	c.PhaseID = 0
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phase_id")
}

func TestCareOffering_Validate_DefaultsDaysModeToFixed(t *testing.T) {
	c := validCareOffering()
	c.DaysOfWeekMode = ""
	require.NoError(t, c.Validate())
	assert.Equal(t, DaysOfWeekModeFixed, c.DaysOfWeekMode)
}

func TestCareOffering_Validate_RejectsUnknownDaysMode(t *testing.T) {
	c := validCareOffering()
	c.DaysOfWeekMode = "wild_choice"
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "days_of_week_mode")
}

func TestCareOffering_Validate_AcceptsParentChoiceMode(t *testing.T) {
	c := validCareOffering()
	c.DaysOfWeekMode = DaysOfWeekModeParentChoice
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_RejectsUnknownAvailableDay(t *testing.T) {
	c := validCareOffering()
	c.AvailableDays = []string{"mon", "funday"}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "funday")
}

func TestCareOffering_Validate_AcceptsAllCanonicalDays(t *testing.T) {
	c := validCareOffering()
	c.AvailableDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_AvailableDaysIsCaseInsensitive(t *testing.T) {
	// canonicalDaySet check lowercases on the fly; mixed-case input
	// shouldn't reject just because of casing.
	c := validCareOffering()
	c.AvailableDays = []string{"Mon", "TUE"}
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_EmptyAvailableDaysRejected(t *testing.T) {
	// Business rule changed with #1885: the weekday selection is a
	// deliberate, required input. An offering without days silently
	// hid the misconfiguration that produced wrong enrollments in
	// production (Mo-Fr default vs. "Montags" offering).
	c := validCareOffering()
	c.AvailableDays = nil
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available_days")
}

func TestCareOffering_Validate_RejectsNegativeCapacity(t *testing.T) {
	c := validCareOffering()
	cap := -1
	c.Capacity = &cap
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capacity")
}

func TestCareOffering_Validate_AcceptsZeroCapacity(t *testing.T) {
	// Zero is legal and explicit — "full from the start, no enrollments
	// allowed until admin raises the cap".
	c := validCareOffering()
	cap := 0
	c.Capacity = &cap
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_RejectsNegativePrice(t *testing.T) {
	c := validCareOffering()
	price := -50
	c.PriceCents = &price
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "price_cents")
}

func TestCareOffering_Validate_AcceptsZeroPrice(t *testing.T) {
	c := validCareOffering()
	price := 0
	c.PriceCents = &price
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_AcceptsRequiredWithoutCapacity(t *testing.T) {
	c := validCareOffering()
	c.IsRequired = true
	c.Capacity = nil
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_RejectsRequiredWithCapacity(t *testing.T) {
	// A required offering must be available to every child, so a hard
	// capacity limit (which could fill up and block enrollments) is illegal.
	c := validCareOffering()
	c.IsRequired = true
	cap := 20
	c.Capacity = &cap
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capacity")
}

func TestCareOffering_Validate_AcceptsCapacityWhenNotRequired(t *testing.T) {
	c := validCareOffering()
	c.IsRequired = false
	cap := 20
	c.Capacity = &cap
	assert.NoError(t, c.Validate())
}

func TestCareOffering_Validate_NormalizesAutoAddGradeLevels(t *testing.T) {
	c := validCareOffering()
	c.AutoAddGradeLevels = []int{2, 1, 2, 4}

	require.NoError(t, c.Validate())

	assert.Equal(t, []int{2, 1, 4}, c.AutoAddGradeLevels)
}

func TestCareOffering_Validate_RejectsInvalidAutoAddGradeLevels(t *testing.T) {
	for _, level := range []int{0, -1, 14} {
		t.Run(fmt.Sprintf("grade_%d", level), func(t *testing.T) {
			c := validCareOffering()
			c.AutoAddGradeLevels = []int{level}

			err := c.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "auto_add_grade_levels")
		})
	}
}

func TestCareOffering_HasUnlimitedCapacity_NilIsUnlimited(t *testing.T) {
	c := validCareOffering()
	c.Capacity = nil
	assert.True(t, c.HasUnlimitedCapacity())
}

func TestCareOffering_HasUnlimitedCapacity_SetIsBounded(t *testing.T) {
	c := validCareOffering()
	cap := 30
	c.Capacity = &cap
	assert.False(t, c.HasUnlimitedCapacity())
}

// ---- selection group + rule ----------------------------------------------

func TestCareOffering_Validate_DefaultsRuleToOptional(t *testing.T) {
	c := validCareOffering() // SelectionRule left empty
	require.NoError(t, c.Validate())
	assert.Equal(t, SelectionRuleOptional, c.SelectionRule)
}

func TestCareOffering_Validate_AcceptsGroupedRule(t *testing.T) {
	c := validCareOffering()
	c.SelectionGroup = "Betreuungsumfang"
	c.SelectionRule = SelectionRuleExactlyOne
	require.NoError(t, c.Validate())
}

func TestCareOffering_Validate_TrimsGroup(t *testing.T) {
	c := validCareOffering()
	c.SelectionGroup = "  Betreuungsumfang  "
	c.SelectionRule = SelectionRuleAtLeastOne
	require.NoError(t, c.Validate())
	assert.Equal(t, "Betreuungsumfang", c.SelectionGroup)
}

func TestCareOffering_Validate_RejectsUnknownRule(t *testing.T) {
	c := validCareOffering()
	c.SelectionGroup = "G"
	c.SelectionRule = "pick_three"
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection_rule")
}

func TestCareOffering_Validate_RuleRequiresGroup(t *testing.T) {
	c := validCareOffering()
	c.SelectionRule = SelectionRuleExactlyOne // no group
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selection_group")
}
