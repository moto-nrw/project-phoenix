package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// gradeCollectionSettingsStub answers the two collection toggles
// independently, which schoolClassSettingsStub cannot (it pins grade
// collection to true), plus the tenant grade-level cap.
type gradeCollectionSettingsStub struct {
	collectGrade bool
	collectClass bool
	// gradeMax is enrollment.grade_level_max; the zero value stands for the
	// registry default so tests that only exercise the collection toggles do
	// not have to spell it out.
	gradeMax int
}

func (s gradeCollectionSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	switch key {
	case configModel.KeyEnrollmentCollectGradeLevel:
		return s.collectGrade, nil
	case configModel.KeyEnrollmentCollectSchoolClass:
		return s.collectClass, nil
	default:
		return false, nil
	}
}

func (s gradeCollectionSettingsStub) ResolveInt(_ context.Context, key string) (int, error) {
	if key == configModel.KeyEnrollmentGradeLevelMax {
		if s.gradeMax == 0 {
			return schoolclass.DefaultGradeLevelMax, nil
		}
		return s.gradeMax, nil
	}
	return 0, nil
}

// A grade-based eligibility restriction is uncheckable when the school does not
// collect the grade level: every child's grade is nil at submit and the gate
// rejects every submission with grade_not_eligible. Create/Update must reject
// that config up front — but, unlike the class restriction, a whole-grade phase
// must stay valid while concrete-class collection is off (#1663). Like the
// class guard, this one only applies to ACTIVE phases, so the fixtures below
// are active.
func TestValidateEligibleGradeLevelsCollectable(t *testing.T) {
	restricted := &enrollmentModels.Phase{EligibleGradeLevels: []int{3}, IsActive: true}
	unrestricted := &enrollmentModels.Phase{EligibleGradeLevels: []int{}, IsActive: true}

	t.Run("grade collection off with a grade restriction is rejected", func(t *testing.T) {
		s := &phaseService{settings: gradeCollectionSettingsStub{collectGrade: false, collectClass: true}}
		err := s.validateEligibleClassesCollectable(context.Background(), restricted)
		require.ErrorIs(t, err, ErrInvalidPhase)
	})

	t.Run("grade collection on without concrete classes passes", func(t *testing.T) {
		s := &phaseService{settings: gradeCollectionSettingsStub{collectGrade: true, collectClass: false}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), restricted),
			"a whole-grade phase must not require concrete-class collection")
	})

	t.Run("empty grade list is never gated", func(t *testing.T) {
		s := &phaseService{settings: gradeCollectionSettingsStub{collectGrade: false, collectClass: false}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), unrestricted))
	})

	t.Run("nil settings skips the guard", func(t *testing.T) {
		s := &phaseService{}
		assert.NoError(t, s.validateEligibleClassesCollectable(context.Background(), restricted))
	})

	// Same carve-out as the class guard: an inactive phase takes no
	// submissions, so its grade restriction cannot reject one — and gating it
	// would leave a historical phase unwritable after the grade toggle was
	// turned off (#1663).
	t.Run("an inactive phase is not gated", func(t *testing.T) {
		s := &phaseService{settings: gradeCollectionSettingsStub{collectGrade: false, collectClass: false}}
		inactive := &enrollmentModels.Phase{EligibleGradeLevels: []int{3}}
		require.NoError(t, s.validateEligibleClassesCollectable(context.Background(), inactive))
	})
}

// A grade restriction above the tenant's enrollment.grade_level_max is
// unsatisfiable from both ends: the form offers grades 1..cap so the parent can
// never pick the eligible grade, and a hand-crafted submission carrying it is
// rejected by the cap before eligibility is consulted. Create/Update must
// reject that config instead of persisting an active phase no submission can
// reach (#1663).
func TestValidateEligibleGradeLevelsWithinTenantCap(t *testing.T) {
	collecting := func(gradeMax int) gradeCollectionSettingsStub {
		return gradeCollectionSettingsStub{collectGrade: true, collectClass: true, gradeMax: gradeMax}
	}

	t.Run("grade above the tenant cap is rejected", func(t *testing.T) {
		s := &phaseService{settings: collecting(4)}
		err := s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{5}, IsActive: true},
		)
		require.ErrorIs(t, err, ErrInvalidPhase)
		assert.Contains(t, err.Error(), "above the tenant maximum 4")
	})

	t.Run("only the above-cap entry of a mixed list trips the guard", func(t *testing.T) {
		s := &phaseService{settings: collecting(4)}
		err := s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{3, 7}, IsActive: true},
		)
		require.ErrorIs(t, err, ErrInvalidPhase)
		assert.Contains(t, err.Error(), "grade 7")
	})

	t.Run("grade at the cap passes", func(t *testing.T) {
		s := &phaseService{settings: collecting(4)}
		require.NoError(t, s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{4}, IsActive: true},
		))
	})

	t.Run("a raised cap admits the higher grade", func(t *testing.T) {
		s := &phaseService{settings: collecting(schoolclass.MaxGradeLevel)}
		require.NoError(t, s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{9}, IsActive: true},
		))
	})

	t.Run("an out-of-range cap fails closed", func(t *testing.T) {
		s := &phaseService{settings: collecting(schoolclass.MaxGradeLevel + 1)}
		err := s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{3}, IsActive: true},
		)
		require.Error(t, err, "a corrupt cap must stop the write, not pick a substitute bound")
		assert.NotErrorIs(t, err, ErrInvalidPhase, "a corrupt setting is operational, not a client error")
	})

	t.Run("an unrestricted phase never reads the cap", func(t *testing.T) {
		s := &phaseService{settings: collecting(schoolclass.MaxGradeLevel + 1)}
		require.NoError(t, s.validateEligibleClassesCollectable(
			context.Background(),
			&enrollmentModels.Phase{EligibleGradeLevels: []int{}, IsActive: true},
		))
	})
}
