package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Grade-level eligibility (#1663): the representation for a phase aimed at a
// whole grade, which the concrete-class list cannot express.

func TestPhaseValidate_CoalescesNilEligibleGradeLevels(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.EligibleGradeLevels = nil
	require.NoError(t, p.Validate())
	assert.NotNil(t, p.EligibleGradeLevels, "nil would bind NULL on a NOT NULL jsonb column")
	assert.Empty(t, p.EligibleGradeLevels)
}

// A whole-grade phase needs NO concrete classes — that is the entire point of
// the field. Validate must not force a class list or the mandatory pick.
func TestPhaseValidate_AcceptsGradeRestrictionWithoutConcreteClasses(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.EligibleGradeLevels = []int{3}
	require.NoError(t, p.Validate())
	assert.Equal(t, []int{3}, p.EligibleGradeLevels)
	assert.Empty(t, p.EligibleSchoolClasses, "a grade restriction must not imply a class list")
	assert.False(t, p.RequireSchoolClass, "a grade restriction must not force a concrete-class pick")
}

func TestPhaseValidate_DedupesEligibleGradeLevelsKeepingOrder(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.EligibleGradeLevels = []int{3, 1, 3, 4}
	require.NoError(t, p.Validate())
	assert.Equal(t, []int{3, 1, 4}, p.EligibleGradeLevels)
}

func TestPhaseValidate_RejectsOutOfRangeEligibleGradeLevels(t *testing.T) {
	t.Parallel()

	for _, level := range []int{0, -1, 14} {
		p := makeEligibilityTestPhase()
		p.EligibleGradeLevels = []int{level}
		err := p.Validate()
		require.Error(t, err, "grade %d must be rejected", level)
		assert.Contains(t, err.Error(), "eligible_grade_levels")
	}
}

// Both restrictions apply together, so a class outside the eligible grades can
// never be declared — the child would have to be in two grades at once. Reject
// the unsatisfiable pair up front rather than letting every submission fail.
func TestPhaseValidate_RejectsClassOutsideEligibleGradeLevels(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.AvailableSchoolClasses = []string{"2a"}
	p.EligibleSchoolClasses = []string{"2a"}
	p.EligibleGradeLevels = []int{3}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eligible_grade_levels")
}

func TestPhaseValidate_AcceptsClassMatchingEligibleGradeLevel(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.AvailableSchoolClasses = []string{"3a", "3b"}
	p.EligibleSchoolClasses = []string{"3a"}
	p.EligibleGradeLevels = []int{3}
	require.NoError(t, p.Validate())
}

// A class without a numeric grade ("Bienen") carries no derivable grade, so it
// stays compatible with any grade restriction — same rule the form's
// class-per-grade filter uses.
func TestPhaseValidate_AcceptsPrefixlessClassWithGradeRestriction(t *testing.T) {
	t.Parallel()

	p := makeEligibilityTestPhase()
	p.AvailableSchoolClasses = []string{"Bienen"}
	p.EligibleSchoolClasses = []string{"Bienen"}
	p.EligibleGradeLevels = []int{3}
	require.NoError(t, p.Validate())
}
