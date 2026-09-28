package studentintegration_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func studentStatus(t *testing.T, db *bun.DB, id int64) string {
	t.Helper()
	var status string
	err := db.NewSelect().TableExpr("users.student_school_memberships").Column("status").Where("student_profile_id = ?", id).Where("deleted_at IS NULL").Scan(context.Background(), &status)
	require.NoError(t, err)
	return status
}

func TestStudentDirectoryReadsClassesAndCohorts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	membership := buildStudentMembership(t, db)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, db, "Anna", "Directory", "1a")
	second := testpkg.CreateTestStudent(t, db, "Ben", "Directory", "2b")
	graduate := testpkg.CreateTestStudent(t, db, "Carl", "Directory", "4c")

	graduated, err := membership.Graduate(ctx, []int64{graduate.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, graduated)

	byID, err := module.ListStudentsByID(ctx, []int64{first.ID, graduate.ID, first.ID, 0})
	require.NoError(t, err)
	require.Len(t, byID, 2, "alumni stay visible by id so callers can see the lifecycle status")
	assert.Equal(t, first.ID, byID[0].ID)
	assert.Equal(t, first.PersonID, byID[0].PersonID)
	assert.Equal(t, testpkg.Tenant(t), byID[0].TenantID)
	assert.True(t, byID[1].IsAlumnus())
	names, err := module.ListStudentNamesByID(ctx, []int64{first.ID, graduate.ID})
	require.NoError(t, err)
	require.Len(t, names, 2)
	assert.Equal(t, peopledirectory.StudentName{StudentID: first.ID, FirstName: "Anna", LastName: "Directory"}, names[0])

	classes, err := module.ListSchoolClasses(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"1a", "2b"}, classes, "the graduate's class is not listed")

	cohort, err := module.ListStudentsByClasses(ctx, []string{"2b", "4c", ""})
	require.NoError(t, err)
	require.Len(t, cohort, 1)
	assert.Equal(t, second.ID, cohort[0].ID)

	enrolled, err := module.ListEnrolledStudents(ctx)
	require.NoError(t, err)
	ids := make([]int64, 0, len(enrolled))
	for _, student := range enrolled {
		ids = append(ids, student.ID)
	}
	assert.Contains(t, ids, first.ID)
	assert.Contains(t, ids, second.ID)
	assert.NotContains(t, ids, graduate.ID)
}

func TestStudentDirectoryPromotesGraduatesAndReactivates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	membership := buildStudentMembership(t, db)
	ctx := testpkg.Ctx(t)
	promoted := testpkg.CreateTestStudent(t, db, "Promoted", "Student", "1a")
	moved := testpkg.CreateTestStudent(t, db, "Moved", "Student", "1b")

	affected, err := membership.ChangeClass(ctx, []int64{promoted.ID, moved.ID}, "1a", "2a")
	require.NoError(t, err)
	assert.EqualValues(t, 1, affected, "only students still in the from-class move")

	reverted, err := membership.ChangeClass(ctx, []int64{promoted.ID}, "2a", "1a")
	require.NoError(t, err)
	assert.EqualValues(t, 1, reverted)
	reverted, err = membership.ChangeClass(ctx, []int64{moved.ID}, "2a", "1a")
	require.NoError(t, err)
	assert.Zero(t, reverted, "a student that never reached the to-class is left alone")

	graduated, err := membership.Graduate(ctx, []int64{promoted.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, graduated)
	assert.Equal(t, peopledirectory.StudentStatusAlumnus, studentStatus(t, db, promoted.ID))
	graduated, err = membership.Graduate(ctx, []int64{promoted.ID})
	require.NoError(t, err)
	assert.Zero(t, graduated, "graduating twice is idempotent")

	reactivated, err := membership.Reactivate(ctx, []int64{promoted.ID, moved.ID}, "active", schoolmembership.EnforceChildQuota)
	require.NoError(t, err)
	assert.Equal(t, []int64{promoted.ID}, reactivated, "only alumni are restored")
	assert.Equal(t, "active", studentStatus(t, db, promoted.ID))

	_, err = membership.Reactivate(ctx, []int64{promoted.ID}, peopledirectory.StudentStatusAlumnus, schoolmembership.EnforceChildQuota)
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
}

func TestStudentDirectoryLocksRowsInsideTheCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observed []peopleCompose.Observation
	module := buildStudentOwnersModule(t, db, func(observation peopleCompose.Observation) { observed = append(observed, observation) })
	student := testpkg.CreateTestStudent(t, db, "Locked", "Student", "3a")

	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		require.NoError(t, module.LockStudent(txCtx, student.ID))
		require.NoError(t, module.LockStudent(txCtx, student.ID), "the row lock is re-entrant within one transaction")
		return module.LockStudent(txCtx, student.ID+1_000_000)
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNotFound)
	require.ErrorIs(t, module.LockStudent(testpkg.Ctx(t), 0), peopledirectory.ErrInvalidStudent)

	codes := make([]string, 0, len(observed))
	for _, observation := range observed {
		if observation.Operation == "lock_student" {
			codes = append(codes, peopledirectory.ErrorCode(observation.Err))
		}
	}
	assert.Equal(t, []string{"none", "none", "not_found"}, codes)
}

func TestStudentDirectoryTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	membership := buildStudentMembership(t, db)
	student := testpkg.CreateTestStudent(t, db, "Isolated", "Student", "2c")
	otherCtx, _ := otherTenantContext(t, db)

	listed, err := module.ListStudentsByID(otherCtx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, listed)
	names, err := module.ListStudentNamesByID(otherCtx, []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, names)

	classes, err := module.ListSchoolClasses(otherCtx)
	require.NoError(t, err)
	assert.NotContains(t, classes, "2c")

	promoted, err := membership.ChangeClass(otherCtx, []int64{student.ID}, "2c", "3c")
	require.NoError(t, err)
	assert.Zero(t, promoted)
	graduated, err := membership.Graduate(otherCtx, []int64{student.ID})
	require.NoError(t, err)
	assert.Zero(t, graduated)
	assert.Equal(t, "active", studentStatus(t, db, student.ID))

	err = tenant.WithinCurrentTenant(otherCtx, func(txCtx context.Context) error {
		require.ErrorIs(t, module.LockStudent(txCtx, student.ID), peopledirectory.ErrStudentNotFound)
		// Visiting students are the one deliberate way past the boundary.
		across, err := module.ListStudentsAcrossTenantsByID(txCtx, []int64{student.ID})
		require.NoError(t, err)
		require.Len(t, across, 1)
		assert.Equal(t, testpkg.Tenant(t), across[0].TenantID)
		return nil
	})
	require.NoError(t, err)
}

func TestStudentNamesExcludeSoftDeletedPeople(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	student := testpkg.CreateTestStudent(t, db, "Deleted", "Person", "2c")

	_, err := db.NewUpdate().TableExpr(`users.persons`).
		Set(`deleted_at = NOW()`).
		Where(`id = ?`, student.PersonID).
		Exec(context.Background())
	require.NoError(t, err)

	names, err := module.ListStudentNamesByID(testpkg.Ctx(t), []int64{student.ID})
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestStudentDirectoryWritesRollBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	membership := buildStudentMembership(t, db)
	student := testpkg.CreateTestStudent(t, db, "Rolled", "Back", "1a")
	wantErr := errors.New("abort outer transaction")

	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		graduated, err := membership.Graduate(txCtx, []int64{student.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, graduated)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assert.Equal(t, "active", studentStatus(t, db, student.ID))

	// A retry after the rollback applies the write exactly once.
	graduated, err := membership.Graduate(testpkg.Ctx(t), []int64{student.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, graduated)
}

func TestStudentDirectoryRequiresATransactionForWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)

	err := module.LockStudent(context.Background(), 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, sql.ErrNoRows)
	assert.NotErrorIs(t, err, peopledirectory.ErrStudentNotFound, "a missing runtime is not reported as a missing student")
}

// The Abgänge view reads a transition's ledger through this lookup: a child
// whose row was purged since the apply must be absent from the result, so the
// caller can tell "purged" apart from "still an alumnus" (#2711, ported from
// the deleted grade-transition repository test).
func TestStudentLookupOmitsPurgedChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildStudentOwnersModule(t, db)
	membership := buildStudentMembership(t, db)
	ctx := testpkg.Ctx(t)
	active := testpkg.CreateTestStudent(t, db, "Aktives", "Kind", "1a")
	graduate := testpkg.CreateTestStudent(t, db, "Abgang", "Kind", "4a")

	graduated, err := membership.Graduate(ctx, []int64{graduate.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, graduated, "exactly the one child in the cohort graduates")

	// A student id that no longer has a row at all.
	missingID := graduate.ID + 1_000_000

	found, err := module.ListStudentsByID(ctx, []int64{active.ID, graduate.ID, missingID})
	require.NoError(t, err)
	states := make(map[int64]bool, len(found))
	for _, student := range found {
		states[student.ID] = student.IsAlumnus()
	}
	require.Len(t, found, 2)
	assert.True(t, states[graduate.ID], "this is the one read whose whole purpose is to see graduates")
	assert.False(t, states[active.ID])
	assert.NotContains(t, states, missingID)
}
