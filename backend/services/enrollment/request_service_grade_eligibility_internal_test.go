package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Grade-level eligibility enforcement (#1663). The class gate cannot stand in
// for this: a school that collects only the grade level has no concrete class
// to compare against, so a whole-grade phase would otherwise be unenforceable.

func gradeEligibilityPhase(levels ...int) *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		Audience:            enrollmentModels.PhaseAudienceOpen,
		EligibleGradeLevels: levels,
	}
}

func gradeLevelPtr(v int16) *int16 { return &v }

func TestValidateChildGradeEligibility_UnrestrictedPhaseAcceptsEverything(t *testing.T) {
	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(2)}}))
	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(),
		[]SubmitChild{{FirstName: "Kim"}}), "no restriction must not require a grade")
}

func TestValidateChildGradeEligibility_AcceptsEligibleGrade(t *testing.T) {
	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)}}))
}

func TestValidateChildGradeEligibility_RejectsOtherGrade(t *testing.T) {
	err := validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(4)}})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
	require.ErrorIs(t, err, ErrInvalidSubmission, "must keep the 400 category")
}

// A missing grade is a rejection, not a pass: with grade collection off every
// child's grade is nil, and treating that as eligible would silently disable
// the restriction instead of surfacing the misconfiguration.
func TestValidateChildGradeEligibility_RejectsMissingGrade(t *testing.T) {
	err := validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim"}})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
}

// Every child in a multi-child submission is checked, not just the first.
func TestValidateChildGradeEligibility_RejectsIneligibleSibling(t *testing.T) {
	err := validateChildGradeEligibility(gradeEligibilityPhase(3, 4), []SubmitChild{
		{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)},
		{FirstName: "Alex", TargetGradeLevel: gradeLevelPtr(1)},
	})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
}

// The gate runs on the shared child-eligibility path, so submit AND the
// editable-request path enforce it — the same wiring the class gate uses.
func TestValidatePhaseChildEligibility_AppliesGradeRestriction(t *testing.T) {
	svc := &requestService{}
	phase := gradeEligibilityPhase(3)

	err := svc.validatePhaseChildEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(2)}},
	})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)

	require.NoError(t, svc.validatePhaseChildEligibility(context.Background(), phase, SubmitRequest{
		Children: []SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)}},
	}))
}

// A late invite / admin manual submission sets AllowClosedPhase and bypasses
// every eligibility gate, grades included — same override the class gate honors.
func TestValidatePhaseEligibility_AllowClosedPhaseBypassesGradeRestriction(t *testing.T) {
	svc := &requestService{}
	err := svc.validatePhaseEligibility(context.Background(), gradeEligibilityPhase(3), SubmitRequest{
		AllowClosedPhase: true,
		Children:         []SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(1)}},
	})
	require.NoError(t, err)
}
