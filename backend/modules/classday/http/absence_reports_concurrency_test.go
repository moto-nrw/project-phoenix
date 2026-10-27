package classdayhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMorningCorrectionInvalidatesSchoolAfterCommit(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.markAbsent(t)
	listener := &realtime.Client{Channel: make(chan realtime.Event, 4), AccountID: int64(f.claims.ID)}
	f.hub.RegisterSchool(listener, f.tenantID)
	defer f.hub.Unregister(listener)
	elsewhere := &realtime.Client{Channel: make(chan realtime.Event, 4), AccountID: int64(f.claims.ID)}
	otherTenant, _ := testpkg.CreateTestTenant(t, f.db)
	f.hub.RegisterSchool(elsewhere, otherTenant)
	defer f.hub.Unregister(elsewhere)
	staff := testpkg.CreateTestStaff(t, f.db, "Morning", "Correction")
	write := classday.MorningAttendanceWrite{SchoolClass: f.class, Date: classday.Date(morningMonday), RecordedBy: staff.ID,
		Marks: []classday.MorningAttendanceMark{{StudentID: f.present, Status: classday.MorningAttendancePresent}}}
	rollback := errors.New("roll back correction")
	err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), f.db, f.tenantID, func(ctx context.Context) error {
		require.NoError(t, f.module.ClassDay.RecordMorningAttendance(ctx, write))
		require.Empty(t, listener.Channel, "no event before commit")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Empty(t, listener.Channel, "rollback must emit nothing")
	notices := len(f.module.Unreported.Reports)
	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.Len(t, f.module.Unreported.Reports, notices, "a correction sends no absence notification")
	require.Len(t, listener.Channel, 1, "the school must refresh even without a notification")
	event := <-listener.Channel
	require.Equal(t, realtime.EventClassDayChanged, event.Type)
	require.Nil(t, event.Data.StudentID)
	require.Empty(t, elsewhere.Channel, "invalidation stays at its school")
	_, rows := f.absenceRows(t, classDayReadPermission)
	require.False(t, rows[f.present].Unreported)
}

func TestSchoolReportWaitsForMorningCorrection(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := setupMorningAttendanceFixture(t, morningMonday)
	f.markAbsent(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()
	reports := f.module.ClassDay.(classday.AbsenceReports)
	staff := testpkg.CreateTestStaff(t, f.db, "Concurrent", "Correction")
	result := make(chan error, 1)
	err := testpkg.WithTenantTx(t, ctx, f.db, f.tenantID, func(txCtx context.Context, tx bun.Tx) error {
		// Hold the teacher's correction uncommitted: the report initially reads
		// the old absent mark, then must wait and recheck it under the row lock.
		require.NoError(t, f.module.ClassDay.RecordMorningAttendance(txCtx, classday.MorningAttendanceWrite{
			SchoolClass: f.class, Date: classday.Date(morningMonday), RecordedBy: staff.ID,
			Marks: []classday.MorningAttendanceMark{{StudentID: f.present, Status: classday.MorningAttendancePresent}},
		}))
		var holderPID int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &holderPID))
		go func() {
			result <- testpkg.WithinTenantContext(t, ctx, f.db, f.tenantID, func(reportCtx context.Context) error {
				return reports.ReportAbsence(reportCtx, classday.AbsenceReportWrite{
					SchoolClass: f.class, Date: classday.Date(morningMonday), StudentID: f.present, Status: classday.AbsenceReportSick,
					Actor: classday.AbsenceReportActor{AccountID: int64(f.claims.ID), Name: "School office"},
				})
			})
		}()
		require.Eventually(t, func() bool {
			var blocked bool
			require.NoError(t, f.db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND ? = ANY(pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &blocked))
			return blocked
		}, 5*time.Second, 10*time.Millisecond)
		return nil
	})
	require.NoError(t, err)
	select {
	case err := <-result:
		require.ErrorIs(t, err, classday.ErrAbsenceReportNotMissing)
	case <-ctx.Done():
		t.Fatal("report did not finish", ctx.Err())
	}
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM active.student_status_days WHERE student_id = ? AND date = ? AND cleared_at IS NULL`, f.present, calendar.Date(morningMonday)).Scan(ctx, &count))
	require.Zero(t, count, "a corrected morning must not gain a status day")
	people := rootAPI.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, f.db))
	child, err := people.Student.FindByID(ctx, f.present)
	require.NoError(t, err)
	require.True(t, child.Sick == nil || !*child.Sick)
	require.True(t, child.Excused == nil || !*child.Excused)
}
