package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// minimal valid phase for eligibility-field validation tests (#1663).
func makeEligibilityTestPhase() *Phase {
	return &Phase{
		Name:             "Eligibility Testphase",
		Kind:             PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
	}
}

func TestPhaseValidate_AudienceDefaultsToOpen(t *testing.T) {
	p := makeEligibilityTestPhase()
	require.NoError(t, p.Validate())
	assert.Equal(t, PhaseAudienceOpen, p.Audience)
}

func TestPhaseValidate_AcceptsKnownAudiences(t *testing.T) {
	for _, audience := range []string{PhaseAudienceOpen, PhaseAudienceNewStudents, PhaseAudienceExistingStudents, PhaseAudienceLinkedParents} {
		p := makeEligibilityTestPhase()
		p.Audience = audience
		require.NoError(t, p.Validate(), "audience %q must validate", audience)
		assert.Equal(t, audience, p.Audience)
	}
}

func TestPhaseValidate_RejectsUnknownAudience(t *testing.T) {
	p := makeEligibilityTestPhase()
	p.Audience = "everyone"
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience")
}

func TestPhaseValidate_RejectsGrade1EligibleClass(t *testing.T) {
	// A grade-1 concrete class is never collected by the form, so a
	// restriction targeting one is unsatisfiable — reject at save (#1663).
	for _, cls := range []string{"1a", "Klasse 1b", "1"} {
		p := makeEligibilityTestPhase()
		p.EligibleSchoolClasses = []string{cls}
		err := p.Validate()
		require.Error(t, err, "grade-1 class %q must be rejected", cls)
		assert.Contains(t, err.Error(), "grade 1")
	}
}

func TestPhaseValidate_AcceptsGrade2PlusEligibleClasses(t *testing.T) {
	p := makeEligibilityTestPhase()
	p.EligibleSchoolClasses = []string{"2a", "3b", "Bienen"}
	require.NoError(t, p.Validate())
}

func TestPhaseValidate_CoalescesNilEligibleSchoolClasses(t *testing.T) {
	p := makeEligibilityTestPhase()
	p.EligibleSchoolClasses = nil
	require.NoError(t, p.Validate())
	// nil would bind NULL on the NOT NULL jsonb column — must coalesce
	// to an empty slice, mirroring AvailableSchoolClasses.
	assert.NotNil(t, p.EligibleSchoolClasses)
	assert.Empty(t, p.EligibleSchoolClasses)
}
