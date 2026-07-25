package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Grade-1 concrete-class collection (#1663). Grade 1 is opt-in (#1833), and the
// trigger has to agree with what the submit-time class gate demands — a phase
// whose eligible class list is satisfiable by a grade-1 child must collect that
// child's class, or every grade-1 submission is cleared and then rejected with
// class_not_eligible.

func TestCollectsGrade1Class(t *testing.T) {
	tests := []struct {
		name      string
		available []string
		eligible  []string
		want      bool
	}{
		{
			name:      "no restriction, no grade-1 class offered stays grade-level only",
			available: []string{"2a", "3a"},
			want:      false,
		},
		{
			name:      "no restriction, prefixless class alone does not trigger collection",
			available: []string{"Bienen", "2a"},
			want:      false,
		},
		{
			name:      "no restriction, an offered grade-1 class triggers collection",
			available: []string{"1a", "2a"},
			want:      true,
		},
		{
			name:      "restriction to a prefixless class triggers collection",
			available: []string{"Bienen"},
			eligible:  []string{"Bienen"},
			want:      true,
		},
		{
			name:      "restriction to a grade-1 class triggers collection",
			available: []string{"1a", "2a"},
			eligible:  []string{"1a"},
			want:      true,
		},
		{
			name:      "restriction to grade >= 2 classes leaves grade 1 classless",
			available: []string{"1a", "2a"},
			eligible:  []string{"2a"},
			want:      false,
		},
		{
			name:      "blank eligible entries are not a restriction",
			available: []string{"1a"},
			eligible:  []string{"  "},
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phase := &enrollmentModels.Phase{
				AvailableSchoolClasses: tc.available,
				EligibleSchoolClasses:  tc.eligible,
			}
			assert.Equal(t, tc.want, CollectsGrade1Class(phase))
		})
	}
}

func TestCollectsGrade1Class_NilPhase(t *testing.T) {
	assert.False(t, CollectsGrade1Class(nil))
}

// The decision must not change when a self-service form load narrows the
// offered list to the eligible subset — otherwise the form and the submit path
// disagree about whether grade 1 declares a class at all.
func TestCollectsGrade1Class_StableUnderFormNarrowing(t *testing.T) {
	phase := &enrollmentModels.Phase{
		AvailableSchoolClasses: []string{"Bienen", "2a", "3a"},
		EligibleSchoolClasses:  []string{"Bienen"},
	}
	before := CollectsGrade1Class(phase)
	narrowOfferedClassesToEligibleForForm(phase)
	assert.Equal(t, []string{"Bienen"}, phase.AvailableSchoolClasses)
	assert.Equal(t, before, CollectsGrade1Class(phase))
	assert.True(t, before)
}

// A phase restricted to a prefixless class ("Bienen") is satisfiable by a
// grade-1 child: the class is collected, kept, and passes the eligibility gate.
// Before #1663's fix the class was cleared for grade 1 and the very next gate
// rejected the submission, so such a phase accepted no grade-1 child at all.
func TestValidateAndNormalizeSchoolClasses_Grade1KeepsPrefixlessEligibleClass(t *testing.T) {
	phase := &enrollmentModels.Phase{
		Audience:               enrollmentModels.PhaseAudienceOpen,
		AvailableSchoolClasses: []string{"Bienen"},
		EligibleSchoolClasses:  []string{"Bienen"},
		EligibleGradeLevels:    []int{1},
		RequireSchoolClass:     true,
	}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: true}}}
	children := []SubmitChild{{
		FirstName:         "Kim",
		LastName:          "Test",
		TargetGradeLevel:  grade(1),
		TargetSchoolClass: classPtr(" Bienen "),
	}}

	require.NoError(t, svc.validateAndNormalizeSchoolClasses(context.Background(), phase, children))
	require.NotNil(t, children[0].TargetSchoolClass, "grade 1 must keep the eligible prefixless class")
	assert.Equal(t, "Bienen", *children[0].TargetSchoolClass)
	require.NoError(t, svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{Children: children}))
}

// The pick is mandatory rather than silently cleared: an omitted class on such
// a phase is a "please choose" error, not an eligibility rejection.
func TestValidateAndNormalizeSchoolClasses_Grade1RequiresPrefixlessEligibleClass(t *testing.T) {
	phase := &enrollmentModels.Phase{
		Audience:               enrollmentModels.PhaseAudienceOpen,
		AvailableSchoolClasses: []string{"Bienen"},
		EligibleSchoolClasses:  []string{"Bienen"},
		RequireSchoolClass:     true,
	}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: true}}}
	children := []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetGradeLevel: grade(1)}}

	err := svc.validateAndNormalizeSchoolClasses(context.Background(), phase, children)
	require.ErrorIs(t, err, ErrInvalidSubmission)
}

// A restriction naming only grade >= 2 classes keeps the #1833 default: grade 1
// declares no class and is rejected by the eligibility gate as ineligible,
// which is the correct outcome for a phase aimed at older children.
func TestValidateAndNormalizeSchoolClasses_Grade1StaysClasslessForGrade2Restriction(t *testing.T) {
	phase := &enrollmentModels.Phase{
		Audience:               enrollmentModels.PhaseAudienceOpen,
		AvailableSchoolClasses: []string{"1a", "2a"},
		EligibleSchoolClasses:  []string{"2a"},
		RequireSchoolClass:     true,
	}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: true}}}
	children := []SubmitChild{{FirstName: "Kim", LastName: "Test", TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("2a")}}

	require.NoError(t, svc.validateAndNormalizeSchoolClasses(context.Background(), phase, children))
	assert.Nil(t, children[0].TargetSchoolClass)
	require.ErrorIs(t,
		svc.validatePhaseEligibility(context.Background(), phase, SubmitRequest{Children: children}),
		ErrChildClassNotEligible)
}
