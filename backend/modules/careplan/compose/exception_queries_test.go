package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExceptionQueriesShareTenantAndTransactionWithoutStatusSlotCommands(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Query", "Transaction", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Query", "Writer")
	module := buildModule(t, db)
	var observed []Observation
	queries, err := NewExceptionQueries(db, func(o Observation) { observed = append(observed, o) })
	require.NoError(t, err)
	otherCtx := tenantContext(t, db, testpkg.UniqueTestTenantID(t))
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	txCtx := tenant.WithTransactionForTest(ctx, tx)
	date := careplan.Date("2031-02-03")
	pickup, err := module.CreatePickupException(txCtx, careplan.PickupException{StudentID: student.ID, ExceptionDate: date, CreatedBy: staff.ID})
	require.NoError(t, err)
	status, err := module.UpsertStudentStatusDay(txCtx, careplan.StudentStatusDay{StudentID: student.ID, Date: date, Status: "sick", ReportedAt: time.Now(), Source: "parent"})
	require.NoError(t, err)
	foundPickup, err := queries.FindPickupException(txCtx, pickup.ID, false)
	require.NoError(t, err)
	assert.Equal(t, pickup.ID, foundPickup.ID)
	foundStatus, err := queries.FindStudentStatusDay(txCtx, status.ID, true)
	require.NoError(t, err)
	assert.Equal(t, status.ID, foundStatus.ID)
	pickups, err := queries.ListPickupExceptions(txCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	statuses, err := queries.ListStudentStatusDays(txCtx, careplan.StudentStatusDayFilter{StudentIDs: []int64{student.ID}, ActiveOnly: true})
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	foreignCtx := tenant.WithTransactionForTest(otherCtx, tx)
	_, err = queries.FindPickupException(foreignCtx, pickup.ID, false)
	require.ErrorIs(t, err, careplan.ErrStudentScheduleNotFound)
	_, err = queries.FindStudentStatusDay(foreignCtx, status.ID, true)
	require.ErrorIs(t, err, careplan.ErrStudentStatusDayNotFound)
	require.NoError(t, tx.Rollback())
	_, err = queries.FindPickupException(ctx, pickup.ID, false)
	require.ErrorIs(t, err, careplan.ErrStudentScheduleNotFound)
	_, err = queries.FindStudentStatusDay(ctx, status.ID, true)
	require.ErrorIs(t, err, careplan.ErrStudentStatusDayNotFound)
	require.Len(t, observed, 8)
	for _, observation := range observed {
		assert.EqualValues(t, 1, observation.Stats.Queries)
	}
}

func TestDayLocksPreserveStudentFailureContracts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Lock", "Failure", "1a")
	missing := errors.New("owner student missing")
	databaseFailure := errors.New("owner database failed")
	for _, tc := range []struct {
		name    string
		failure error
		want    error
	}{
		{name: "missing student", failure: fmt.Errorf("lookup: %w", missing), want: careplanning.ErrStudentNotFound},
		{name: "database failure", failure: databaseFailure, want: databaseFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			locks, err := NewDayLocks(db, func(context.Context, int64) error { return tc.failure }, missing)
			require.NoError(t, err)
			err = locks.LockStudentAndExceptionDay(testpkg.Ctx(t), student.ID, "2031-02-03")
			require.ErrorIs(t, err, tc.want)
			assert.Equal(t, "lock student for care exception day: "+tc.want.Error(), err.Error())
		})
	}
	locks, err := NewDayLocks(db, func(context.Context, int64) error { t.Fatal("invalid ID reached student lock"); return nil }, missing)
	require.NoError(t, err)
	assert.EqualError(t, locks.LockStudentAndExceptionDay(testpkg.Ctx(t), 0, "2031-02-03"), "student id is required")
}
