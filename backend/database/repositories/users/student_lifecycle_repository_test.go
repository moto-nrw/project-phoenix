package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setLifecycle directly mutates the lifecycle columns on a fresh student
// fixture so tests can stage the pre-conditions FindPending* /
// FindActive* expect. CreateTestStudent does not currently expose status /
// enrolled_from / enrolled_until knobs, so we patch via a direct UPDATE
// scoped to the test row's ID.
func setLifecycle(t *testing.T, db *bun.DB, studentID int64, status users.StudentStatus, enrolledFrom, enrolledUntil *timezone.Date) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	q := db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(status)).
		Where("id = ?", studentID)
	if enrolledFrom != nil {
		q = q.Set("enrolled_from = ?", *enrolledFrom)
	} else {
		q = q.Set("enrolled_from = NULL")
	}
	if enrolledUntil != nil {
		q = q.Set("enrolled_until = ?", *enrolledUntil)
	} else {
		q = q.Set("enrolled_until = NULL")
	}
	_, err := q.Exec(ctx)
	require.NoError(t, err, "failed to seed lifecycle columns")
}

func TestStudentRepository_FindPendingDueForActivation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	yesterday := timezone.TodayDate().AddDays(-1)
	tomorrow := timezone.TodayDate().AddDays(1)
	asOf := timezone.TodayDate()

	t.Run("returns pending students with enrolled_from <= asOf", func(t *testing.T) {
		dueStudent := testpkg.CreateTestStudent(t, db, "PendingDue", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, dueStudent.ID)
		futureStudent := testpkg.CreateTestStudent(t, db, "PendingFuture", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, futureStudent.ID)
		activeStudent := testpkg.CreateTestStudent(t, db, "Active", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, activeStudent.ID)

		setLifecycle(t, db, dueStudent.ID, users.StudentStatusPending, &yesterday, nil)
		setLifecycle(t, db, futureStudent.ID, users.StudentStatusPending, &tomorrow, nil)
		setLifecycle(t, db, activeStudent.ID, users.StudentStatusActive, &yesterday, nil)

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
		defer cleanupStudentRecords(t, db, nullStudent.ID)
		setLifecycle(t, db, nullStudent.ID, users.StudentStatusPending, nil, nil)

		results, err := repo.FindPendingDueForActivation(ctx, asOf)
		require.NoError(t, err)
		for _, s := range results {
			assert.NotEqual(t, nullStudent.ID, s.ID, "NULL enrolled_from must not be returned")
		}
	})
}

func TestStudentRepository_FindActiveDueForDeactivation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	yesterday := timezone.TodayDate().AddDays(-1)
	tomorrow := timezone.TodayDate().AddDays(1)
	asOf := timezone.TodayDate()

	t.Run("returns active students with enrolled_until <= asOf", func(t *testing.T) {
		dueStudent := testpkg.CreateTestStudent(t, db, "ActiveDue", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, dueStudent.ID)
		futureStudent := testpkg.CreateTestStudent(t, db, "ActiveFuture", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, futureStudent.ID)
		pendingStudent := testpkg.CreateTestStudent(t, db, "PendingNotInactive", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, pendingStudent.ID)

		setLifecycle(t, db, dueStudent.ID, users.StudentStatusActive, nil, &yesterday)
		setLifecycle(t, db, futureStudent.ID, users.StudentStatusActive, nil, &tomorrow)
		setLifecycle(t, db, pendingStudent.ID, users.StudentStatusPending, nil, &yesterday)

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
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	t.Run("transitions pending student to active", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Activate", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, student.ID)
		setLifecycle(t, db, student.ID, users.StudentStatusPending, nil, nil)

		err := repo.UpdateStatus(ctx, student.ID, users.StudentStatusActive)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, student.ID)
		require.NoError(t, err)
		assert.Equal(t, users.StudentStatusActive, found.Status)
	})

	t.Run("returns error for non-existent student", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, int64(999999), users.StudentStatusActive)
		require.Error(t, err, "update on missing student must error")
	})
}

func TestStudentRepository_TransitionStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "Conditional", "Lifecycle", "1a")
	defer cleanupStudentRecords(t, db, student.ID)
	setLifecycle(t, db, student.ID, users.StudentStatusPending, nil, nil)

	applied, err := repo.TransitionStatus(
		ctx,
		student.ID,
		users.StudentStatusPending,
		users.StudentStatusActive,
	)
	require.NoError(t, err)
	assert.True(t, applied)

	applied, err = repo.TransitionStatus(
		ctx,
		student.ID,
		users.StudentStatusPending,
		users.StudentStatusInactive,
	)
	require.NoError(t, err)
	assert.False(t, applied, "a stale expected status must not overwrite the current status")

	found, err := repo.FindByID(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, users.StudentStatusActive, found.Status)
}
