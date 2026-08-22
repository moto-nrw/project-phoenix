package enrollment

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Heimweg-Beschränkung publish guard (#2381): a schema whose
// single_mode_grades rule keys on the target grade level must not be
// publishable while the grade-level collection setting is off — the rule
// would silently never restrict anyone.

func singleModeSchemaFields() []enrollmentModels.FormField {
	return []enrollmentModels.FormField{{
		Key: "heimwege", Label: "Erlaubte Heimwege",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		AppliesToCh: true, Target: enrollmentModels.TargetStudentAllowedDepartureModes,
		SingleModeGrades: []int{1},
	}}
}

func TestEnsureSingleModeGradesCollectable_RejectsWithCollectionOff(t *testing.T) {
	t.Parallel()

	settings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			require.Equal(t, configModel.KeyEnrollmentCollectGradeLevel, key)
			return false, nil
		},
	}
	s := &formSchemaService{settings: settings}
	err := s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single_mode_grades")
	assert.Contains(t, err.Error(), "Klassenstufen-Abfrage")
}

func TestEnsureSingleModeGradesCollectable_PassesWithCollectionOn(t *testing.T) {
	t.Parallel()

	settings := &configtest.Mock{
		ResolveBoolFn: func(context.Context, string) (bool, error) { return true, nil },
	}
	s := &formSchemaService{settings: settings}
	assert.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields()))
}

func TestEnsureSingleModeGradesCollectable_SkipsWithoutRule(t *testing.T) {
	t.Parallel()

	// No single_mode_grades anywhere → no settings read at all.
	settings := &configtest.Mock{
		ResolveBoolFn: func(context.Context, string) (bool, error) {
			t.Fatal("settings must not be consulted without a rule")
			return false, nil
		},
	}
	s := &formSchemaService{settings: settings}
	fields := []enrollmentModels.FormField{{
		Key: "heimwege", Label: "Erlaubte Heimwege",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		AppliesToCh: true, Target: enrollmentModels.TargetStudentAllowedDepartureModes,
	}}
	assert.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), fields))
}

func TestEnsureSingleModeGradesCollectable_NilSettingsSkips(t *testing.T) {
	t.Parallel()

	s := &formSchemaService{}
	assert.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields()))
}

func TestEnsureSingleModeGradesCollectable_TakesSharedLock(t *testing.T) {
	t.Parallel()

	// Same #1663 reasoning as the phase-side guards: validate-then-write
	// must serialize against a concurrent settings write disabling grade
	// collection.
	calls := 0
	settings := &configtest.Mock{
		ResolveBoolFn: func(context.Context, string) (bool, error) { return true, nil },
		LockClassCollectionPairFn: func(context.Context) error {
			calls++
			return nil
		},
	}
	s := &formSchemaService{settings: settings}
	require.NoError(t, s.ensureSingleModeGradesCollectable(context.Background(), singleModeSchemaFields()))
	assert.Equal(t, 1, calls)
}
