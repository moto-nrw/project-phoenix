package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	guardianMonday    careplan.Date = "2032-05-10"
	guardianTuesday   careplan.Date = "2032-05-11"
	guardianWednesday careplan.Date = "2032-05-12"
)

type guardianFixture struct {
	ctx       context.Context
	module    *careplan.Module
	studentID int64
	staffID   int64
	accountID int64
}

func newGuardianFixture(t *testing.T) guardianFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return guardianFixture{
		ctx:       testpkg.Ctx(t),
		module:    buildModule(t, db),
		studentID: testpkg.CreateTestStudent(t, db, "Guardian", "Change", "1a").ID,
		staffID:   testpkg.CreateTestStaff(t, db, "Guardian", "Staff").ID,
		accountID: testpkg.CreateTestAccount(t, db, "guardian-change").ID,
	}
}

func wallClock(hour, minute int) *time.Time {
	clock := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)
	return &clock
}

func (f guardianFixture) pickupRows(t *testing.T, date careplan.Date) []careplan.PickupException {
	t.Helper()
	rows, err := f.module.ListPickupExceptions(f.ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{f.studentID}, Date: date})
	require.NoError(t, err)
	return rows
}

func (f guardianFixture) activeStatusDays(t *testing.T) []careplan.StudentStatusDay {
	t.Helper()
	rows, err := f.module.ListStudentStatusDays(f.ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{f.studentID}, From: guardianMonday, To: guardianWednesday, ActiveOnly: true,
	})
	require.NoError(t, err)
	return rows
}

func (f guardianFixture) seedStatus(t *testing.T, date careplan.Date, status string) {
	t.Helper()
	_, err := f.module.UpsertStudentStatusDay(f.ctx, careplan.StudentStatusDay{
		StudentID: f.studentID, Date: date, Status: status, ReportedAt: time.Now(), Source: careplan.StudentStatusSourceManual,
	})
	require.NoError(t, err)
}

func (f guardianFixture) report(status string, dates ...careplan.Date) careplan.GuardianAbsenceReport {
	note := "Fieber"
	return careplan.GuardianAbsenceReport{
		StudentID: f.studentID, GuardianAccountID: f.accountID, Dates: dates, Status: status,
		Note: &note, ReportedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestGuardianAbsenceReportReplacesOtherStatusesAndReturnsOnlySubmittedDays(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	absences, err := NewGuardianAbsences(f.module)
	require.NoError(t, err)
	f.seedStatus(t, guardianMonday, careplan.StudentStatusDayExcused)
	f.seedStatus(t, guardianTuesday, careplan.StudentStatusDayExcused)

	report := f.report(careplan.StudentStatusDaySick, guardianWednesday, guardianMonday)
	rows, err := absences.ReportGuardianAbsence(f.ctx, report)
	require.NoError(t, err)

	require.Len(t, rows, 2, "Tuesday lies inside the range but was not submitted")
	for _, row := range rows {
		assert.Contains(t, []careplan.Date{guardianMonday, guardianWednesday}, row.Date)
		assert.Equal(t, careplan.StudentStatusDaySick, row.Status)
		assert.Equal(t, careplan.StudentStatusSourceParent, row.Source)
		require.NotNil(t, row.GuardianAccountID)
		assert.Equal(t, f.accountID, *row.GuardianAccountID)
		require.NotNil(t, row.Note)
		assert.Equal(t, "Fieber", *row.Note)
		assert.True(t, report.ReportedAt.Equal(row.ReportedAt))
	}
	var active []string
	for _, row := range f.activeStatusDays(t) {
		active = append(active, row.Date.String()+" "+row.Status)
	}
	assert.ElementsMatch(t, []string{
		guardianMonday.String() + " " + careplan.StudentStatusDaySick,
		guardianTuesday.String() + " " + careplan.StudentStatusDayExcused,
		guardianWednesday.String() + " " + careplan.StudentStatusDaySick,
	}, active, "the Monday excusal is cleared, the unsubmitted Tuesday stays")
}

func TestGuardianAbsenceReportRefusesManualPartialAbsence(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	absences, err := NewGuardianAbsences(f.module)
	require.NoError(t, err)
	f.seedStatus(t, guardianMonday, careplan.StudentStatusDayExcused)
	staffID := f.staffID
	_, err = f.module.CreatePickupException(f.ctx, careplan.PickupException{
		StudentID: f.studentID, ExceptionDate: guardianWednesday, ExcusedFrom: wallClock(14, 0),
		ExcusedCreatedBy: &staffID, Source: careplan.ExceptionSourceStaff, CreatedBy: f.staffID,
	})
	require.NoError(t, err)

	_, err = absences.ReportGuardianAbsence(f.ctx, f.report(careplan.StudentStatusDaySick, guardianMonday, guardianWednesday))
	require.ErrorIs(t, err, careplan.ErrManualPartialAbsenceConflict)

	rows := f.activeStatusDays(t)
	require.Len(t, rows, 1, "the refusal happens before any status is cleared or written")
	assert.Equal(t, careplan.StudentStatusDayExcused, rows[0].Status)
}

func TestGuardianAbsenceReportAllowsAutomaticExcusalAndUnrequestedPartialAbsence(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	absences, err := NewGuardianAbsences(f.module)
	require.NoError(t, err)
	staffID := f.staffID
	_, err = f.module.CreatePickupException(f.ctx, careplan.PickupException{
		StudentID: f.studentID, ExceptionDate: guardianMonday, PickupTime: wallClock(13, 0), ExcusedFrom: wallClock(13, 0),
		ExcusedAuto: true, Source: careplan.ExceptionSourceStaff, CreatedBy: f.staffID,
	})
	require.NoError(t, err)
	_, err = f.module.CreatePickupException(f.ctx, careplan.PickupException{
		StudentID: f.studentID, ExceptionDate: guardianTuesday, ExcusedFrom: wallClock(14, 0),
		ExcusedCreatedBy: &staffID, Source: careplan.ExceptionSourceStaff, CreatedBy: f.staffID,
	})
	require.NoError(t, err)

	rows, err := absences.ReportGuardianAbsence(f.ctx, f.report(careplan.StudentStatusDayExcused, guardianMonday, guardianWednesday))
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestGuardianAbsenceReportJoinsCallerTransaction(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	absences, err := NewGuardianAbsences(f.module)
	require.NoError(t, err)
	rollback := errors.New("roll back the report")

	err = tenant.WithinCurrentTenant(f.ctx, func(txCtx context.Context) error {
		rows, reportErr := absences.ReportGuardianAbsence(txCtx, f.report(careplan.StudentStatusDaySick, guardianMonday))
		require.NoError(t, reportErr)
		require.Len(t, rows, 1)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	assert.Empty(t, f.activeStatusDays(t))
}

func TestGuardianAbsenceReportRequiresDates(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	absences, err := NewGuardianAbsences(f.module)
	require.NoError(t, err)
	_, err = absences.ReportGuardianAbsence(f.ctx, f.report(careplan.StudentStatusDaySick))
	require.Error(t, err)
	_, err = NewGuardianAbsences(nil)
	require.Error(t, err)
}

// recordingExcusal observes the pickup excusal coupling. ReleaseBeforeDelete
// records whether the row still existed, which proves the release ran first.
type recordingExcusal struct {
	careplan.PickupAutoExcusal
	module *careplan.Module
	calls  []string
	synced []int64
}

func (r *recordingExcusal) Sync(_ context.Context, id int64) (bool, error) {
	r.calls = append(r.calls, "sync")
	r.synced = append(r.synced, id)
	return true, nil
}

func (r *recordingExcusal) ReleaseBeforeDelete(ctx context.Context, row *careplan.PickupException) error {
	if _, err := r.module.FindPickupException(ctx, row.ID, false); err != nil {
		r.calls = append(r.calls, "release-after-delete")
		return nil
	}
	r.calls = append(r.calls, "release")
	return nil
}

func (f guardianFixture) change(date careplan.Date, pickup *time.Time, reason string) careplan.GuardianPickupChange {
	return careplan.GuardianPickupChange{
		TenantID: tenant.FromContext(f.ctx), StudentID: f.studentID, GuardianAccountID: f.accountID,
		Date: date, PickupTime: pickup, Reason: &reason,
	}
}

func TestGuardianPickupExceptionCreatesUpdatesAndClearsTheGuardianLeg(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	excusal := &recordingExcusal{module: f.module}
	pickups, err := NewGuardianPickupExceptions(f.module, excusal)
	require.NoError(t, err)

	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(14, 0), "Arzt")))
	rows := f.pickupRows(t, guardianMonday)
	require.Len(t, rows, 1)
	created := rows[0]
	assert.Equal(t, careplan.ExceptionSourceGuardian, created.Source)
	assert.Zero(t, created.CreatedBy)
	require.NotNil(t, created.CreatedByGuardian)
	assert.Equal(t, f.accountID, *created.CreatedByGuardian)
	assert.Equal(t, "14:00", created.PickupTime.Format("15:04"))
	assert.Equal(t, "Arzt", *created.Reason)
	assert.Equal(t, []int64{created.ID}, excusal.synced)

	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(13, 30), "Oma")))
	rows = f.pickupRows(t, guardianMonday)
	require.Len(t, rows, 1)
	assert.Equal(t, created.ID, rows[0].ID, "the guardian row is updated in place")
	assert.Equal(t, "13:30", rows[0].PickupTime.Format("15:04"))
	assert.Equal(t, "Oma", *rows[0].Reason)
	assert.Equal(t, []int64{created.ID, created.ID}, excusal.synced)

	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, nil, "")))
	assert.Empty(t, f.pickupRows(t, guardianMonday))
	assert.Equal(t, []string{"sync", "sync", "release"}, excusal.calls, "a removed row is released first and not synced")

	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianTuesday, nil, "")))
	assert.Equal(t, []string{"sync", "sync", "release"}, excusal.calls, "clearing a missing row does nothing")
}

func TestGuardianPickupExceptionNeverTouchesStaffRows(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	excusal := &recordingExcusal{module: f.module}
	pickups, err := NewGuardianPickupExceptions(f.module, excusal)
	require.NoError(t, err)
	staff, err := f.module.CreatePickupException(f.ctx, careplan.PickupException{
		StudentID: f.studentID, ExceptionDate: guardianMonday, PickupTime: wallClock(15, 0),
		Source: careplan.ExceptionSourceStaff, CreatedBy: f.staffID,
	})
	require.NoError(t, err)

	for _, pickup := range []*time.Time{wallClock(14, 0), nil} {
		err = pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, pickup, "Arzt"))
		require.ErrorIs(t, err, careplan.ErrPickupExceptionStaffOwned)
	}
	rows := f.pickupRows(t, guardianMonday)
	require.Len(t, rows, 1)
	assert.Equal(t, staff.ID, rows[0].ID)
	assert.Equal(t, careplan.ExceptionSourceStaff, rows[0].Source)
	assert.Equal(t, "15:00", rows[0].PickupTime.Format("15:04"))
	assert.Empty(t, excusal.calls)
}

func TestGuardianPickupExceptionWithoutExcusalCoupling(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	pickups, err := NewGuardianPickupExceptions(f.module, nil)
	require.NoError(t, err)

	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(14, 0), "Arzt")))
	rows := f.pickupRows(t, guardianMonday)
	require.Len(t, rows, 1)
	require.NoError(t, pickups.WithdrawGuardianPickupException(f.ctx, rows[0]))
	assert.Empty(t, f.pickupRows(t, guardianMonday))
	_, err = NewGuardianPickupExceptions(nil, nil)
	require.Error(t, err)
}

func TestGuardianPickupExceptionWithdrawReleasesBeforeDelete(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	excusal := &recordingExcusal{module: f.module}
	pickups, err := NewGuardianPickupExceptions(f.module, excusal)
	require.NoError(t, err)
	require.NoError(t, pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(14, 0), "Arzt")))
	rows := f.pickupRows(t, guardianMonday)
	require.Len(t, rows, 1)

	require.NoError(t, pickups.WithdrawGuardianPickupException(f.ctx, rows[0]))
	assert.Empty(t, f.pickupRows(t, guardianMonday))
	assert.Equal(t, []string{"sync", "release"}, excusal.calls)
}

func TestGuardianPickupExceptionJoinsCallerTransaction(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	pickups, err := NewGuardianPickupExceptions(f.module, &recordingExcusal{module: f.module})
	require.NoError(t, err)
	rollback := errors.New("roll back the pickup change")

	err = tenant.WithinCurrentTenant(f.ctx, func(txCtx context.Context) error {
		require.NoError(t, pickups.ApplyGuardianPickupException(txCtx, f.change(guardianMonday, wallClock(14, 0), "Arzt")))
		rows, listErr := f.module.ListPickupExceptions(txCtx, careplan.StudentScheduleFilter{StudentIDs: []int64{f.studentID}, Date: guardianMonday})
		require.NoError(t, listErr)
		require.Len(t, rows, 1)
		require.NoError(t, pickups.WithdrawGuardianPickupException(txCtx, rows[0]))
		require.NoError(t, pickups.ApplyGuardianPickupException(txCtx, f.change(guardianTuesday, wallClock(14, 0), "Arzt")))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	assert.Empty(t, f.pickupRows(t, guardianMonday))
	assert.Empty(t, f.pickupRows(t, guardianTuesday))
}

// hiddenPickupRows hides existing pickup exceptions from the command, so the
// apply takes the create path while the row already exists: the same state a
// concurrent guardian write produces between the read and the insert.
type hiddenPickupRows struct{ *careplan.Module }

func (hiddenPickupRows) ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	return nil, nil
}

type failingSync struct {
	careplan.PickupAutoExcusal
	err error
}

func (s failingSync) Sync(context.Context, int64) (bool, error) { return false, s.err }

func TestGuardianPickupExceptionClassifiesConcurrentWritesAsRaced(t *testing.T) {
	t.Parallel()
	f := newGuardianFixture(t)
	visible, err := NewGuardianPickupExceptions(f.module, nil)
	require.NoError(t, err)
	require.NoError(t, visible.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(14, 0), "Arzt")))

	// The raw driver error of a real duplicate insert, reused below as the
	// failure of the excusal sync.
	var duplicate driverError
	t.Run("insert against an existing row", func(t *testing.T) {
		racing, err := NewGuardianPickupExceptions(hiddenPickupRows{f.module}, nil)
		require.NoError(t, err)
		err = racing.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(13, 0), "Oma"))
		require.ErrorIs(t, err, careplan.ErrGuardianPickupExceptionRaced)
		var raw driverError
		require.True(t, errors.As(err, &raw), "the raw database error stays in the chain")
		assert.Equal(t, "23505", raw.Field('C'))
		duplicate = raw
	})
	t.Run("excusal sync", func(t *testing.T) {
		require.NotNil(t, duplicate, "the insert subtest provides a real unique violation")
		pickups, err := NewGuardianPickupExceptions(f.module, failingSync{err: duplicate})
		require.NoError(t, err)
		err = pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(13, 0), "Oma"))
		require.ErrorIs(t, err, careplan.ErrGuardianPickupExceptionRaced)
		var raw driverError
		require.True(t, errors.As(err, &raw), "the raw database error stays in the chain")
		assert.Equal(t, "23505", raw.Field('C'))
	})
	t.Run("other failures stay unclassified", func(t *testing.T) {
		syncErr := errors.New("sync unavailable")
		pickups, err := NewGuardianPickupExceptions(f.module, failingSync{err: syncErr})
		require.NoError(t, err)
		err = pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianMonday, wallClock(13, 0), "Oma"))
		require.ErrorIs(t, err, syncErr)
		assert.NotErrorIs(t, err, careplan.ErrGuardianPickupExceptionRaced)
		_, err = f.module.CreatePickupException(f.ctx, careplan.PickupException{
			StudentID: f.studentID, ExceptionDate: guardianTuesday, PickupTime: wallClock(15, 0),
			Source: careplan.ExceptionSourceStaff, CreatedBy: f.staffID,
		})
		require.NoError(t, err)
		err = pickups.ApplyGuardianPickupException(f.ctx, f.change(guardianTuesday, wallClock(13, 0), "Oma"))
		require.ErrorIs(t, err, careplan.ErrPickupExceptionStaffOwned)
		assert.NotErrorIs(t, err, careplan.ErrGuardianPickupExceptionRaced)
	})
}
