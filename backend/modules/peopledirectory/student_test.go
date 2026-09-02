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
	assert.Zero(t, engine.calls)
	require.NoError(t, module.LockStudent(ctx, 9))
	assert.Equal(t, []int64{9}, engine.student.ids)

	promoted, err := module.PromoteStudents(ctx, nil, "1a", "2a")
	require.NoError(t, err)
	assert.Zero(t, promoted)
	_, err = module.PromoteStudents(ctx, []int64{4}, "", "2a")
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	promoted, err = module.PromoteStudents(ctx, []int64{4, 4, 6}, "1a", "2a")
	require.NoError(t, err)
	assert.EqualValues(t, 2, promoted)
	assert.Equal(t, studentCall{ids: []int64{4, 6}, from: "1a", to: "2a"}, engine.student)

	_, err = module.RevertStudentClass(ctx, 0, "1a", "2a")
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	_, err = module.RevertStudentClass(ctx, 4, "1a", "")
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)

	graduated, err := module.GraduateStudentsByClasses(ctx, []string{""})
	require.NoError(t, err)
	assert.Zero(t, graduated)
	graduated, err = module.GraduateStudents(ctx, []int64{0})
	require.NoError(t, err)
	assert.Zero(t, graduated)

	_, err = module.ReactivateStudents(ctx, []int64{4}, " ")
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	_, err = module.ReactivateStudents(ctx, []int64{4}, peopledirectory.StudentStatusAlumnus)
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	restored, err := module.ReactivateStudents(ctx, []int64{4}, " active ")
	require.NoError(t, err)
	assert.Equal(t, []int64{4}, restored)
	assert.Equal(t, "active", engine.student.status)
}

func TestStudentErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "not_found", peopledirectory.ErrorCode(peopledirectory.ErrStudentNotFound))
	assert.Equal(t, "invalid", peopledirectory.ErrorCode(&peopledirectory.InvalidStudentError{Reason: "student ID is required"}))
	assert.False(t, peopledirectory.Student{Status: "active"}.IsAlumnus())
	assert.True(t, peopledirectory.Student{Status: peopledirectory.StudentStatusAlumnus}.IsAlumnus())
}
