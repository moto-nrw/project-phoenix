package peopledirectory_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleStudentReadsNormalizeIDsAndClassesBeforeTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	listed, err := module.ListStudentsByID(ctx, []int64{0, -4})
	require.NoError(t, err)
	assert.Empty(t, listed)
	assert.Zero(t, engine.calls, "an empty id list never reaches the engine")

	_, err = module.ListStudentsByID(ctx, []int64{7, 7, 3, 0})
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 3}, engine.student.ids, "ids are deduplicated in order, non-positive ones dropped")

	_, err = module.ListStudentNamesByID(ctx, []int64{9, 9, -1, 4})
	require.NoError(t, err)
	assert.Equal(t, []int64{9, 4}, engine.student.ids)

	_, err = module.ListStudentsAcrossTenantsByID(ctx, []int64{5, 5})
	require.NoError(t, err)
	assert.True(t, engine.across)
	assert.Equal(t, []int64{5}, engine.student.ids)

	cohort, err := module.ListStudentsByClasses(ctx, []string{"", ""})
	require.NoError(t, err)
	assert.Empty(t, cohort)
	_, err = module.ListStudentsByClasses(ctx, []string{"2a", "", "2a", "3b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"2a", "3b"}, engine.student.classes)
}

func TestModuleStudentCommandsValidateBeforeTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	require.ErrorIs(t, module.LockStudent(ctx, 0), peopledirectory.ErrInvalidStudent)
	require.ErrorIs(t, module.RenewEnrollmentStudent(ctx, 9, peopledirectory.EnrollmentStudent{
		SchoolClass: "2a", InitialProfile: &peopledirectory.EnrollmentProfilePatch{},
	}), peopledirectory.ErrInvalidStudent)
	assert.Zero(t, engine.calls)
	require.NoError(t, module.LockStudent(ctx, 9))
	assert.Equal(t, []int64{9}, engine.student.ids)

}

func TestStudentErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "not_found", peopledirectory.ErrorCode(peopledirectory.ErrStudentNotFound))
	assert.Equal(t, "invalid", peopledirectory.ErrorCode(&peopledirectory.InvalidStudentError{Reason: "student ID is required"}))
	assert.False(t, peopledirectory.Student{Status: "active"}.IsAlumnus())
	assert.True(t, peopledirectory.Student{Status: peopledirectory.StudentStatusAlumnus}.IsAlumnus())
}
