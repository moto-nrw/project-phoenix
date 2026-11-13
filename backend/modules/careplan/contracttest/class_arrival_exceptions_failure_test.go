package contracttest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// A storage failure while writing a later child ends the class report with
// that failure, and the report's transaction keeps no child (#5824): the
// same rows as one command per child, whose failure rolled the report back.
func TestClassArrivalBatchStorageFailureKeepsNoChild(t *testing.T) {
	t.Parallel()
	f := newClassArrivalBatchFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, f.db, "Fehler", "Erstes", "1a")
	failing := testpkg.CreateTestStudent(t, f.db, "Fehler", "Zweites", "1a")
	last := testpkg.CreateTestStudent(t, f.db, "Fehler", "Drittes", "1a")
	_, err := f.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION schedule.fail_class_arrival_5824() RETURNS trigger AS $$
		BEGIN
			IF NEW.student_id = %d THEN RAISE EXCEPTION 'injected arrival write failure'; END IF;
			RETURN NEW;
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_class_arrival_5824 BEFORE INSERT ON schedule.student_arrival_exceptions
			FOR EACH ROW EXECUTE FUNCTION schedule.fail_class_arrival_5824();`, failing.ID))
	require.NoError(t, err)

	err = testpkg.TenantCommand0(t, ctx, f.db, func(txCtx context.Context) error {
		return f.service.CreateOrReclaimExceptionsForClass(txCtx, "1a", []int64{first.ID, failing.ID, last.ID}, f.date, &f.clock, nil, f.staffID, func() (int64, error) { return f.staffID, nil })
	})
	require.ErrorContains(t, err, "injected arrival write failure")
	var scheduleErr *careplan.ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "create student arrival exception", scheduleErr.Op)
	for _, id := range []int64{first.ID, failing.ID, last.ID} {
		row, readErr := testpkg.TenantQuery2(t, ctx, f.db, f.service.GetStudentArrivalExceptionForDate, id, f.date)
		require.NoError(t, readErr)
		assert.Nil(t, row, "child %d", id)
	}
}

// A storage failure while reclaiming a later child's guardian arrival ends
// the class report with that failure, and the report keeps no child (#5824).
func TestClassArrivalBatchReclaimFailureKeepsNoChild(t *testing.T) {
	t.Parallel()
	f := newClassArrivalBatchFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, f.db, "Fehler", "Neu", "1a")
	reclaimed := testpkg.CreateTestStudent(t, f.db, "Fehler", "Eltern", "1a")
	_, err := f.db.ExecContext(ctx, `INSERT INTO schedule.student_arrival_exceptions (tenant_id, student_id, exception_date, expected_arrival, reason, created_by, source)
		VALUES (?, ?, ?, '09:00', 'Eltern', ?, 'guardian')`, testpkg.Tenant(t), reclaimed.ID, f.date, f.staffID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION schedule.fail_class_reclaim_5824() RETURNS trigger AS $$
		BEGIN
			IF NEW.student_id = %d THEN RAISE EXCEPTION 'injected arrival reclaim failure'; END IF;
			RETURN NEW;
		END $$ LANGUAGE plpgsql;
		CREATE TRIGGER fail_class_reclaim_5824 BEFORE UPDATE ON schedule.student_arrival_exceptions
			FOR EACH ROW EXECUTE FUNCTION schedule.fail_class_reclaim_5824();`, reclaimed.ID))
	require.NoError(t, err)

	err = testpkg.TenantCommand0(t, ctx, f.db, func(txCtx context.Context) error {
		return f.service.CreateOrReclaimExceptionsForClass(txCtx, "1a", []int64{first.ID, reclaimed.ID}, f.date, &f.clock, nil, f.staffID, func() (int64, error) { return f.staffID, nil })
	})
	require.ErrorContains(t, err, "injected arrival reclaim failure")
	var scheduleErr *careplan.ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "update student arrival exception", scheduleErr.Op)
	row, err := testpkg.TenantQuery2(t, ctx, f.db, f.service.GetStudentArrivalExceptionForDate, first.ID, f.date)
	require.NoError(t, err)
	assert.Nil(t, row, "the earlier child's write rolls back")
	row, err = testpkg.TenantQuery2(t, ctx, f.db, f.service.GetStudentArrivalExceptionForDate, reclaimed.ID, f.date)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, careplan.ExceptionSourceGuardian, row.Source, "the guardian's arrival stays")
}
