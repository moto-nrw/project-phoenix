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
	for _, audience := range []string{PhaseAudienceOpen, PhaseAudienceNewStudents, PhaseAudienceLinkedParents} {
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

func TestPhaseValidate_CoalescesNilEligibleSchoolClasses(t *testing.T) {
	p := makeEligibilityTestPhase()
	p.EligibleSchoolClasses = nil
	require.NoError(t, p.Validate())
	// nil would bind NULL on the NOT NULL jsonb column — must coalesce
	// to an empty slice, mirroring AvailableSchoolClasses.
	assert.NotNil(t, p.EligibleSchoolClasses)
	assert.Empty(t, p.EligibleSchoolClasses)
}
