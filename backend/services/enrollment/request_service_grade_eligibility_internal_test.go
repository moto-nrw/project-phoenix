package enrollment

import (
	"context"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/require"
)

// Grade-level eligibility enforcement (#1663). The class gate cannot stand in
// for this: a school that collects only the grade level has no concrete class
// to compare against, so a whole-grade phase would otherwise be unenforceable.

func gradeEligibilityPhase(levels ...int) *capability.Phase {
	return &capability.Phase{
		Audience:            capability.PhaseAudienceOpen,
		EligibleGradeLevels: levels,
	}
}

func gradeLevelPtr(v int16) *int16 { return &v }

func TestValidateChildGradeEligibility_UnrestrictedPhaseAcceptsEverything(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(2)}}))
	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(),
		[]SubmitChild{{FirstName: "Kim"}}), "no restriction must not require a grade")
}

func TestValidateChildGradeEligibility_AcceptsEligibleGrade(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)}}))
}

func TestValidateChildGradeEligibility_RejectsOtherGrade(t *testing.T) {
	t.Parallel()

	err := validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(4)}})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
	require.ErrorIs(t, err, ErrInvalidSubmission, "must keep the 400 category")
}

// A missing grade is a rejection, not a pass: with grade collection off every
// child's grade is nil, and treating that as eligible would silently disable
// the restriction instead of surfacing the misconfiguration.
func TestValidateChildGradeEligibility_RejectsMissingGrade(t *testing.T) {
	t.Parallel()

	err := validateChildGradeEligibility(gradeEligibilityPhase(3),
		[]SubmitChild{{FirstName: "Kim"}})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
}

// Every child in a multi-child submission is checked, not just the first.
func TestValidateChildGradeEligibility_RejectsIneligibleSibling(t *testing.T) {
	t.Parallel()

	err := validateChildGradeEligibility(gradeEligibilityPhase(3, 4), []SubmitChild{
		{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)},
		{FirstName: "Alex", TargetGradeLevel: gradeLevelPtr(1)},
	})
	require.ErrorIs(t, err, ErrChildGradeNotEligible)
}

// The gate runs on the shared child-eligibility path, so submit AND the
// editable-request path enforce it — the same wiring the class gate uses.
func TestValidatePhaseChildEligibility_AppliesGradeRestriction(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	svc := &requestService{}
	err := svc.validatePhaseEligibility(context.Background(), gradeEligibilityPhase(3), SubmitRequest{
		AllowClosedPhase: true,
		Children:         []SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(1)}},
	})
	require.NoError(t, err)
}

// A class-only restriction ("nur 3a") narrows the form's grade options to the
// grades those classes belong to. Without it the grade select still offers
// every grade, the class pick list for another grade comes up empty, and the
// submit fails with class_not_eligible after the whole form was filled in
// (#1663).
func TestNarrowOfferedGradesToEligibleClassesForForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		available []string
		eligible  []string
		grades    []int
		want      []int
	}{
		{
			name:      "class-only restriction derives its grades",
			available: []string{"1a", "3a", "3b", "4a"},
			eligible:  []string{"3a", "3b"},
			want:      []int{3},
		},
		{
			name:      "classes across grades derive a sorted set",
			available: []string{"2a", "3a", "4a"},
			eligible:  []string{"4a", "2a"},
			want:      []int{2, 4},
		},
		{
			name:      "a grade-agnostic class keeps every grade open",
			available: []string{"Bienen", "3a"},
			eligible:  []string{"Bienen", "3a"},
			want:      nil,
		},
		{
			name:      "an explicit grade restriction is left untouched",
			available: []string{"3a"},
			eligible:  []string{"3a"},
			grades:    []int{3},
			want:      []int{3},
		},
		{
			name:      "no class restriction derives nothing",
			available: []string{"2a", "3a"},
			eligible:  []string{},
			want:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phase := &capability.Phase{
				AvailableSchoolClasses: tc.available,
				EligibleSchoolClasses:  tc.eligible,
				EligibleGradeLevels:    tc.grades,
			}
			// Reached through the class narrowing, exactly as the form-load
			// paths call it.
			narrowOfferedClassesToEligibleForForm(phase)
			require.Equal(t, tc.want, phase.EligibleGradeLevels)
		})
	}
}

// The derived restriction is presentation only: Submit reloads the phase, so a
// child in a derived grade is still decided by the class gate.
func TestNarrowOfferedGradesToEligibleClassesForForm_DerivedGradeStillPassesClassGate(t *testing.T) {
	t.Parallel()

	phase := &capability.Phase{
		Audience:               capability.PhaseAudienceOpen,
		AvailableSchoolClasses: []string{"3a", "3b", "4a"},
		EligibleSchoolClasses:  []string{"3a"},
	}
	narrowOfferedClassesToEligibleForForm(phase)
	require.Equal(t, []int{3}, phase.EligibleGradeLevels)
	require.Equal(t, []string{"3a"}, phase.AvailableSchoolClasses,
		"only the eligible class stays offered, so the derived grade has something to pick")
	require.NoError(t, validateChildGradeEligibility(phase,
		[]SubmitChild{{FirstName: "Kim", TargetGradeLevel: gradeLevelPtr(3)}}))
}
