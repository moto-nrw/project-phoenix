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

func TestPhaseValidate_AcceptsGrade1EligibleClassWhenOffered(t *testing.T) {
	// Grade-1 concrete classes ARE supported now (#1663): when the phase also
	// offers the class the form collects it, so a grade-1 restriction is
	// satisfiable and must validate. Each eligible class must be one the phase
	// offers, so available mirrors eligible here.
	for _, cls := range []string{"1a", "Klasse 1b", "1"} {
		p := makeEligibilityTestPhase()
		p.AvailableSchoolClasses = []string{cls}
		p.EligibleSchoolClasses = []string{cls}
		require.NoError(t, p.Validate(), "grade-1 class %q must validate when offered", cls)
		// A non-empty eligibility list forces a concrete class to be collected.
		assert.True(t, p.RequireSchoolClass, "eligibility restriction must require a class pick")
	}
}

func TestPhaseValidate_RejectsGrade1EligibleClassNotOffered(t *testing.T) {
	// The disjoint-list rule still applies to grade 1: an eligible class the
	// phase never offers rejects every submission, so it is rejected at save
	// (#1663) — but now for the "not offered" reason, not a blanket grade-1 ban.
	p := makeEligibilityTestPhase()
	p.AvailableSchoolClasses = []string{"1a"}
	p.EligibleSchoolClasses = []string{"1b"}
	err := p.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available_school_classes")
}

func TestPhaseValidate_AcceptsGrade2PlusEligibleClasses(t *testing.T) {
	p := makeEligibilityTestPhase()
	// Every eligible class must also be offered by the phase (#1663), so the
	// pick list mirrors the eligibility list here.
	p.AvailableSchoolClasses = []string{"2a", "3b", "Bienen"}
	p.EligibleSchoolClasses = []string{"2a", "3b", "Bienen"}
	require.NoError(t, p.Validate())
}

func TestPhaseValidate_RejectsEligibleClassNotAvailable(t *testing.T) {
	// A class listed as eligible but not offered by available_school_classes
	// is unsatisfiable: the form never presents it, so every submission is
	// rejected with class_not_eligible. Reject the disjoint config up front
	// (#1663).
	t.Run("disjoint lists", func(t *testing.T) {
		p := makeEligibilityTestPhase()
		p.AvailableSchoolClasses = []string{"2b"}
		p.EligibleSchoolClasses = []string{"2a"}
		err := p.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "available_school_classes")
	})
	t.Run("eligible without any available", func(t *testing.T) {
		p := makeEligibilityTestPhase()
		p.EligibleSchoolClasses = []string{"2a"}
		err := p.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "available_school_classes")
	})
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
