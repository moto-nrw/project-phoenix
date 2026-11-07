package contracttest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// pickupExceptionsOn stores one ordinary pickup exception per day from the
// first day on and returns their IDs in date order.
func pickupExceptionsOn(t *testing.T, ctx context.Context, service careplan.PickupScheduleService, db *bun.DB, studentID, staffID int64, first calendar.Date, days int) []int64 {
	t.Helper()
	ids := make([]int64, 0, days)
	for day := range days {
		row := &careplan.PickupException{StudentID: studentID, ExceptionDate: careplan.Date(first.AddDays(day)), Reason: testpkg.StrPtr("Ausnahme"), CreatedBy: staffID}
		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, service.CreateStudentPickupException, row))
		ids = append(ids, row.ID)
	}
	return ids
}

// Deleting all exceptions of a child stops at the first one that carries a
// partial absence (#5901). The exceptions before it are already deleted when
// the refusal is returned, the one with the partial absence and the later
// ones stay: the result of one lock-and-delete per exception in date order.
func TestDeleteAllPickupExceptionsStopsAtThePartialAbsence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Teil", "Abwesend", "1a")
	staffID := createPickupServiceTestStaffID(t, db)
	monday := calendar.NewDate(2099, 3, 9)
	before := pickupExceptionsOn(t, ctx, service, db, student.ID, staffID, monday, 1)
	from := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC))
	partial := &careplan.PickupException{
		StudentID: student.ID, ExceptionDate: careplan.Date(monday.AddDays(1)), PickupTime: &from,
		ExcusedFrom: &from, ExcusedCreatedBy: &staffID, ExcusedOwnsPickupTime: true, CreatedBy: staffID,
	}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, service.CreateStudentPickupException, partial))
	after := pickupExceptionsOn(t, ctx, service, db, student.ID, staffID, monday.AddDays(2), 1)

	rollback := errors.New("roll the deletion back")
	var deleteErr error
	var remaining []int64
	err := testpkg.TenantCommand0(t, ctx, db, func(txCtx context.Context) error {
		deleteErr = service.DeleteAllStudentPickupExceptions(txCtx, student.ID)
		rows, readErr := service.GetStudentPickupExceptions(txCtx, student.ID)
		require.NoError(t, readErr)
		for _, row := range rows {
			remaining = append(remaining, row.ID)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.ErrorIs(t, deleteErr, careplan.ErrCareExceptionContainsPartialAbsence)
	assert.Equal(t, []int64{partial.ID, after[0]}, remaining, "the exception before the partial absence is deleted, the rest stays")
	assert.NotContains(t, remaining, before[0])
}

// The deletion locks each day as before, but reads and deletes the locked
// rows in one statement each instead of two per exception.
func TestDeleteAllPickupExceptionsStatementsPerException(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.Ctx(t)
	staffID := createPickupServiceTestStaffID(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	deleteAll := func(days int) int {
		student := testpkg.CreateTestStudent(t, db, "Viele", "Ausnahmen", "1a")
		pickupExceptionsOn(t, ctx, service, db, student.ID, staffID, calendar.NewDate(2099, 3, 9), days)
		counter.Reset()
		require.NoError(t, testpkg.TenantCommand1(t, counter.Context(ctx), db, service.DeleteAllStudentPickupExceptions, student.ID))
		total := counter.Total()
		rows, err := testpkg.TenantQuery1(t, ctx, db, service.GetStudentPickupExceptions, student.ID)
		require.NoError(t, err)
		assert.Empty(t, rows)
		return total
	}
	n3 := deleteAll(3)
	n8 := deleteAll(8)
	t.Logf("delete all pickup exceptions: N3=%d N8=%d", n3, n8)
	assert.Equal(t, 2*5, n8-n3, "each further exception costs only its student and day locks")
}

// A storage failure of the deletion ends it with that failure, and the
// transaction keeps every exception (#5901): one failing delete per row
// rolled the earlier deletes back the same way.
func TestDeleteAllPickupExceptionsStorageFailureKeepsEveryException(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Fehler", "Löschen", "1a")
	staffID := createPickupServiceTestStaffID(t, db)
	ids := pickupExceptionsOn(t, ctx, service, db, student.ID, staffID, calendar.NewDate(2099, 3, 9), 3)
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION schedule.fail_pickup_delete_5901() RETURNS trigger AS $$
		BEGIN
			IF OLD.id = %d THEN RAISE EXCEPTION 'injected pickup delete failure'; END IF;
			RETURN OLD;
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_pickup_delete_5901 BEFORE DELETE ON schedule.student_pickup_exceptions
			FOR EACH ROW EXECUTE FUNCTION schedule.fail_pickup_delete_5901();`, ids[1]))
	require.NoError(t, err)

	err = testpkg.TenantCommand1(t, ctx, db, service.DeleteAllStudentPickupExceptions, student.ID)
	require.ErrorContains(t, err, "injected pickup delete failure")
	var scheduleErr *careplan.ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "delete student pickup exception", scheduleErr.Op)
	rows, err := testpkg.TenantQuery1(t, ctx, db, service.GetStudentPickupExceptions, student.ID)
	require.NoError(t, err)
	remaining := make([]int64, 0, len(rows))
	for _, row := range rows {
		remaining = append(remaining, row.ID)
	}
	assert.Equal(t, ids, remaining, "the failed deletion keeps every exception")
}
