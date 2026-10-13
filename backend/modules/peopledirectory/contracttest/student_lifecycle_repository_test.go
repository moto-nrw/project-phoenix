package contracttest_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentRepository_FindPendingDueForActivation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)

	yesterday := calendar.TodayDate().AddDays(-1)
	tomorrow := calendar.TodayDate().AddDays(1)
	asOf := calendar.TodayDate()

	t.Run("returns pending students with enrolled_from <= asOf", func(t *testing.T) {
		dueStudent := testpkg.CreateTestStudent(t, db, "PendingDue", "Lifecycle", "1a")
		futureStudent := testpkg.CreateTestStudent(t, db, "PendingFuture", "Lifecycle", "1a")
		activeStudent := testpkg.CreateTestStudent(t, db, "Active", "Lifecycle", "1a")

		testpkg.SetStudentLifecycle(t, db, dueStudent.ID, userModels.StudentStatusPending, &yesterday, nil)
		testpkg.SetStudentLifecycle(t, db, futureStudent.ID, userModels.StudentStatusPending, &tomorrow, nil)
		testpkg.SetStudentLifecycle(t, db, activeStudent.ID, userModels.StudentStatusActive, &yesterday, nil)

		results, err := repo.FindPendingDueForActivation(ctx, asOf)
		require.NoError(t, err)

		ids := make(map[int64]bool, len(results))
		for _, s := range results {
			ids[s.ID] = true
		}
		assert.True(t, ids[dueStudent.ID], "due pending student should be returned")
		assert.False(t, ids[futureStudent.ID], "pending student with future enrolled_from must NOT be returned")
		assert.False(t, ids[activeStudent.ID], "active student must NOT be returned")
	})

	t.Run("ignores pending students with NULL enrolled_from", func(t *testing.T) {
		nullStudent := testpkg.CreateTestStudent(t, db, "PendingNull", "Lifecycle", "1a")
		testpkg.SetStudentLifecycle(t, db, nullStudent.ID, userModels.StudentStatusPending, nil, nil)

		results, err := repo.FindPendingDueForActivation(ctx, asOf)
		require.NoError(t, err)
		for _, s := range results {
			assert.NotEqual(t, nullStudent.ID, s.ID, "NULL enrolled_from must not be returned")
		}
	})
}

func TestStudentRepository_FindActiveDueForDeactivation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)

	yesterday := calendar.TodayDate().AddDays(-1)
	tomorrow := calendar.TodayDate().AddDays(1)
	asOf := calendar.TodayDate()

	t.Run("returns active students with enrolled_until <= asOf", func(t *testing.T) {
		dueStudent := testpkg.CreateTestStudent(t, db, "ActiveDue", "Lifecycle", "1a")
		futureStudent := testpkg.CreateTestStudent(t, db, "ActiveFuture", "Lifecycle", "1a")
		pendingStudent := testpkg.CreateTestStudent(t, db, "PendingNotInactive", "Lifecycle", "1a")

		testpkg.SetStudentLifecycle(t, db, dueStudent.ID, userModels.StudentStatusActive, nil, &yesterday)
		testpkg.SetStudentLifecycle(t, db, futureStudent.ID, userModels.StudentStatusActive, nil, &tomorrow)
		testpkg.SetStudentLifecycle(t, db, pendingStudent.ID, userModels.StudentStatusPending, nil, &yesterday)

		results, err := repo.FindActiveDueForDeactivation(ctx, asOf)
		require.NoError(t, err)

		ids := make(map[int64]bool, len(results))
		for _, s := range results {
			ids[s.ID] = true
		}
		assert.True(t, ids[dueStudent.ID], "active student with past enrolled_until should be returned")
		assert.False(t, ids[futureStudent.ID], "active student with future enrolled_until must NOT be returned")
		assert.False(t, ids[pendingStudent.ID], "pending student must NOT be returned even if enrolled_until is past")
	})
}

func TestStudentRepository_UpdateStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)

	t.Run("transitions pending student to active", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Activate", "Lifecycle", "1a")
		testpkg.SetStudentLifecycle(t, db, student.ID, userModels.StudentStatusPending, nil, nil)

		err := repo.UpdateStatus(ctx, student.ID, userModels.StudentStatusActive)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, userModels.StudentStatusActive, found.Status)
	})

	t.Run("returns error for non-existent student", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, int64(999999), userModels.StudentStatusActive)
		require.Error(t, err, "update on missing student must error")
	})
}

func TestStudentRepository_TransitionStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).Student
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Conditional", "Lifecycle", "1a")
	testpkg.SetStudentLifecycle(t, db, student.ID, userModels.StudentStatusPending, nil, nil)

	applied, err := repo.TransitionStatus(
		ctx,
		student.ID,
		userModels.StudentStatusPending,
		userModels.StudentStatusActive,
	)
	require.NoError(t, err)
	assert.True(t, applied)

	applied, err = repo.TransitionStatus(
		ctx,
		student.ID,
		userModels.StudentStatusPending,
		userModels.StudentStatusInactive,
	)
	require.NoError(t, err)
	assert.False(t, applied, "a stale expected status must not overwrite the current status")

	found, err := repo.FindByID(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.StudentStatusActive, found.Status)
}
