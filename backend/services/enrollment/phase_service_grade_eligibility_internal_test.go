package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// gradeCollectionSettingsStub answers the two collection toggles
// independently, which schoolClassSettingsStub cannot (it pins grade
// collection to true).
type gradeCollectionSettingsStub struct {
	collectGrade bool
	collectClass bool
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

// A grade-based eligibility restriction is uncheckable when the school does not
// collect the grade level: every child's grade is nil at submit and the gate
// rejects every submission with grade_not_eligible. Create/Update must reject
// that config up front — but, unlike the class restriction, a whole-grade phase
// must stay valid while concrete-class collection is off (#1663).
func TestValidateEligibleGradeLevelsCollectable(t *testing.T) {
	restricted := &enrollmentModels.Phase{EligibleGradeLevels: []int{3}}
	unrestricted := &enrollmentModels.Phase{EligibleGradeLevels: []int{}}

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
}
