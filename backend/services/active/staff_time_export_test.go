package active_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for the cross-staff export (#1417 2b). The load-bearing guarantee is
// the pin against the Monatskarte: every number in the file must equal what
// GetMonthSummary reports for the same person and month.

func (f *overviewFixture) newExportService() active.StaffTimeExportService {
	return active.NewStaffTimeExportService(
		f.svc,
		f.newWorkSessionService(),
		f.repos.Staff,
		f.repos.DataAccessLog,
		nil,
		nil,
	)
}

func (f *overviewFixture) newWorkSessionService() active.WorkSessionService {
	// nil settings: the F9 deviation checks are opt-in and irrelevant for
	// reading export rows.
	return active.NewWorkSessionService(
		f.repos.WorkSession, f.repos.WorkSessionBreak, f.repos.WorkSessionEdit,
		f.repos.StaffAbsence, f.repos.GroupSupervisor, f.repos.Staff,
		f.repos.StaffWorkSchedule, f.repos.WorkTimeModel,
		nil,
		nil,
	)
}

func (f *overviewFixture) cleanupAccessLogs(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = f.db.NewDelete().
			ModelTableExpr("audit.data_access_log AS t").
			Where("t.tenant_id = ?", f.tenantID).
			Exec(context.Background())
	})
}

// newActorAccount creates a real auth account — actor_account_id carries an FK.
func (f *overviewFixture) newActorAccount(t *testing.T) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, f.db, "time-export-actor")
	return account.ID
}

// TestMonthExportRows_MatchMonthSummary pins the export rows against the
// Monatskarte of every staff member — the file must not be able to disagree
// with the card an admin drills into.
func TestMonthExportRows_MatchMonthSummary(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 3)

	// Differentiate: part-time contract, sick day, payout + comp-time bookings.
	f.addSchedule(t, f.staff[1].ID, 240)
	yesterday := f.today.AddDays(-1)
	if monthOfDate(yesterday) == monthOfDate(f.today) {
		f.addAbsence(t, f.staff[1].ID, activeModels.AbsenceTypeSick, activeModels.AbsenceStatusReported, yesterday, yesterday)
	}
	for _, adj := range []struct {
		typ   string
		delta int
	}{
		{activeModels.BalanceAdjustmentTypePayout, -30},
		{activeModels.BalanceAdjustmentTypeCompTime, -60},
	} {
		adjustment := &activeModels.StaffBalanceAdjustment{
			StaffID:       f.staff[2].ID,
			Type:          adj.typ,
			MinutesDelta:  adj.delta,
			EffectiveDate: f.today,
			Note:          "Buchung",
			DecidedBy:     f.staff[0].ID,
			DecidedAt:     time.Now(),
		}
		adjustment.SetTenantID(f.tenantID)
		require.NoError(t, f.repos.StaffBalanceAdjust.Create(f.ctx, adjustment))
	}

	rows, err := f.svc.GetMonthExportRows(f.ctx, f.today.Year, int(f.today.Month))
	require.NoError(t, err)
	require.Len(t, rows, 3)

	byStaff := make(map[int64]active.MonthExportRow, len(rows))
	for _, row := range rows {
		byStaff[row.StaffID] = row
	}
	for _, staff := range f.staff {
		summary, err := f.monthSvc.GetMonthSummary(f.ctx, staff.ID, f.today.Year, int(f.today.Month))
		require.NoError(t, err)
		row, ok := byStaff[staff.ID]
		require.True(t, ok, "missing export row for staff %d", staff.ID)

		assert.Equal(t, summary.CarryInMinutes, row.CarryInMinutes)
		assert.Equal(t, summary.TargetMinutesToDate, row.TargetMinutes)
		assert.Equal(t, summary.ActualMinutes, row.ActualMinutes)
		assert.Equal(t, summary.CreditedSickMinutes, row.CreditedSickMinutes)
		assert.Equal(t, summary.CreditedVacationMinutes, row.CreditedVacationMinutes)
		assert.Equal(t, summary.CreditedTrainingMinutes, row.CreditedTrainingMinutes)
		assert.Equal(t, summary.CreditedOtherMinutes, row.CreditedOtherMinutes)
		assert.Equal(t, summary.SickDays, row.SickDays)
		assert.Equal(t, summary.VacationDays, row.VacationDays)
		assert.Equal(t, summary.TrainingDays, row.TrainingDays)
		assert.Equal(t, summary.BalanceMinutes, row.BalanceMinutes)
		assert.Equal(t, summary.ClosingBalanceMinutes, row.ClosingBalanceMinutes)
		assert.False(t, row.IsClosed)

		var payout, compTime, reset int
		for _, adjustment := range summary.Adjustments {
			switch adjustment.Type {
			case activeModels.BalanceAdjustmentTypePayout:
				payout += adjustment.MinutesDelta
			case activeModels.BalanceAdjustmentTypeCompTime:
				compTime += adjustment.MinutesDelta
			case activeModels.BalanceAdjustmentTypeReset:
				reset += adjustment.MinutesDelta
			}
		}
		assert.Equal(t, payout, row.PayoutMinutes)
		assert.Equal(t, compTime, row.CompTimeMinutes)
		assert.Equal(t, reset, row.ResetMinutes)
	}
}

// TestMonthExportRows_ClosedMonthCarriesFrozenValue is the settlement-file
// guarantee: a closed month exports the FROZEN carry (the value the next month
// really starts from), with the live difference visible as drift — never
// silently smoothed.
func TestMonthExportRows_ClosedMonthCarriesFrozenValue(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 1)
	staffID := f.staff[0].ID
	closedMonth := timezone.NewDate(f.today.Year, f.today.Month, 1).AddDays(-1)
	settings := wtmIntSettings{
		accountStart: timezone.NewDate(closedMonth.Year, closedMonth.Month, 1).String(),
	}
	monthSvc := active.NewWorkTimeMonthService(
		f.repos.WorkSession, f.repos.WorkSessionBreak, f.repos.StaffAbsence, f.repos.Staff,
		f.repos.StaffWorkSchedule, f.repos.WorkTimeModel, f.repos.StaffShift,
		settings, nil,
	)
	monthSvc.SetAdjustmentReader(f.repos.StaffBalanceAdjust)
	monthSvc.SetSnapshotReader(f.repos.StaffMonthSnapshot)
	closeSvc := active.NewStaffMonthCloseService(f.repos.StaffMonthSnapshot, monthSvc, f.repos.Staff, settings, nil)
	svc := f.newOverviewService(settings)

	closeResult, err := closeSvc.CloseMonth(f.ctx, staffID, closedMonth.Year, int(closedMonth.Month), "Abschluss")
	require.NoError(t, err)
	require.Len(t, closeResult.Snapshots, 1)
	frozenBalance := closeResult.Snapshots[0].ClosingBalanceMinutes

	// Retroactive session — the live number moves, the frozen one must not.
	f.addSession(t, staffID, closedMonth, 4*time.Hour)

	rows, err := svc.GetMonthExportRows(f.ctx, closedMonth.Year, int(closedMonth.Month))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	row := rows[0]

	detail, err := monthSvc.GetMonthSummary(f.ctx, staffID, closedMonth.Year, int(closedMonth.Month))
	require.NoError(t, err)
	require.True(t, detail.IsClosed)

	assert.True(t, row.IsClosed)
	assert.Equal(t, frozenBalance, row.ClosingBalanceMinutes,
		"the file must carry the frozen month-end value the next month starts from")
	assert.Equal(t, detail.DriftMinutes, row.DriftMinutes)
	assert.NotZero(t, row.DriftMinutes, "the retroactive session must be visible as drift")
	assert.Equal(t, detail.BalanceMinutes, row.BalanceMinutes, "live rows stay live, like the Monatskarte")
}

// TestMonthExportRows_YearSpansMonths: month=0 exports January through the
// running month, one row per staff member and month.
func TestMonthExportRows_YearSpansMonths(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 2)

	rows, err := f.svc.GetMonthExportRows(f.ctx, f.today.Year, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2*int(f.today.Month))
	// Rows are grouped per staff member, months ascending.
	assert.Equal(t, 1, rows[0].Month)
	assert.Equal(t, int(f.today.Month), rows[int(f.today.Month)-1].Month)
	assert.Equal(t, rows[0].StaffID, rows[int(f.today.Month)-1].StaffID)
}

func TestMonthExportRows_RejectsFutureMonth(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 1)

	next := f.today.AddDays(40)
	_, err := f.svc.GetMonthExportRows(f.ctx, next.Year, int(next.Month))
	require.Error(t, err)
	assert.ErrorIs(t, err, active.ErrTimeExportInvalid)

	_, err = f.svc.GetMonthExportRows(f.ctx, f.today.Year+1, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, active.ErrTimeExportInvalid)
}

// TestStaffTimeExport_CSVAndAudit exercises the full orchestrator: CSV bytes
// with BOM + header, and the GDPR access-audit row.
func TestStaffTimeExport_CSVAndAudit(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 2)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	exportSvc := f.newExportService()

	file, err := exportSvc.Export(f.ctx, active.TimeExportRequest{
		Year:  f.today.Year,
		Month: int(f.today.Month),
	}, actorID, "admin")
	require.NoError(t, err)

	assert.True(t, bytes.HasPrefix(file.Data, []byte{0xEF, 0xBB, 0xBF}), "UTF-8 BOM for Excel")
	content := string(bytes.TrimPrefix(file.Data, []byte{0xEF, 0xBB, 0xBF}))
	lines := strings.Split(strings.TrimSpace(content), "\n")
	require.Len(t, lines, 3, "header + one row per staff member")
	assert.Contains(t, lines[0], "Übertrag Monatsende")
	assert.Contains(t, lines[0], "Abweichung seit Abschluss")
	assert.Contains(t, lines[1], ";offen")
	assert.Contains(t, file.Filename, ".csv")

	var logs []*auditModels.DataAccessLog
	require.NoError(t, f.db.NewSelect().
		Model(&logs).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where("tenant_id = ?", f.tenantID).
		Scan(context.Background()))
	require.Len(t, logs, 1)
	assert.Equal(t, "time_tracking_export", logs[0].ResourceType)
	assert.Equal(t, actorID, logs[0].ActorAccountID)
	assert.EqualValues(t, "csv", logs[0].Metadata["format"])
	assert.EqualValues(t, "month", logs[0].Metadata["granularity"])
}

// failingAccessLogRepo simulates an audit outage.
type failingAccessLogRepo struct{}

func (failingAccessLogRepo) Create(context.Context, *auditModels.DataAccessLog) error {
	return errors.New("audit down")
}

func (failingAccessLogRepo) ExistsSince(context.Context, int64, string, map[string]string, time.Time) (bool, error) {
	return false, nil
}

// TestStaffTimeExport_NoFileWithoutAudit: when the access-audit row cannot be
// written, no file leaves the service (same contract as enrollment exports).
func TestStaffTimeExport_NoFileWithoutAudit(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 1)
	actorID := f.newActorAccount(t)
	exportSvc := active.NewStaffTimeExportService(
		f.svc, f.newWorkSessionService(), f.repos.Staff, failingAccessLogRepo{}, nil, nil,
	)

	file, err := exportSvc.Export(f.ctx, active.TimeExportRequest{
		Year:  f.today.Year,
		Month: int(f.today.Month),
	}, actorID, "admin")
	require.Error(t, err)
	assert.Nil(t, file)
}

// TestStaffTimeExport_DayRowsMatchSingleExport: a day in the cross-staff file
// must be identical to the same day in the per-staff download.
func TestStaffTimeExport_DayRowsMatchSingleExport(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 2)
	actorID := f.newActorAccount(t)
	f.cleanupAccessLogs(t)
	sessionSvc := f.newWorkSessionService()
	exportSvc := active.NewStaffTimeExportService(
		f.svc, sessionSvc, f.repos.Staff, f.repos.DataAccessLog, nil, nil,
	)

	file, err := exportSvc.Export(f.ctx, active.TimeExportRequest{
		Year:        f.today.Year,
		Month:       int(f.today.Month),
		Granularity: active.ExportGranularityDay,
	}, actorID, "admin")
	require.NoError(t, err)

	monthStart := timezone.NewDate(f.today.Year, f.today.Month, 1)
	singleRows, err := sessionSvc.DayExportRows(f.ctx, f.staff[0].ID, monthStart, monthOfLastDay(f.today))
	require.NoError(t, err)
	require.NotEmpty(t, singleRows)

	content := string(bytes.TrimPrefix(file.Data, []byte{0xEF, 0xBB, 0xBF}))
	// Every cell of the per-staff row appears verbatim in the cross-staff file.
	assert.Contains(t, content, strings.Join(singleRows[0].Cells, ";"))
}

func TestStaffTimeExport_RejectsUnknownParameters(t *testing.T) {
	t.Parallel()

	f := newOverviewFixture(t, 1)
	exportSvc := f.newExportService()

	for _, req := range []active.TimeExportRequest{
		{Year: f.today.Year, Month: 1, Granularity: "weekly"},
		{Year: f.today.Year, Month: 1, Format: "pdf"},
		{Year: f.today.Year, Month: 1, TimeFormat: "seconds"},
		{Year: f.today.Year, Month: 13},
		{Year: 1999, Month: 1},
	} {
		_, err := exportSvc.Export(f.ctx, req, f.staff[0].ID, "admin")
		assert.ErrorIs(t, err, active.ErrTimeExportInvalid)
	}
}

// monthOfLastDay returns the last day of the month containing d.
func monthOfLastDay(d timezone.Date) timezone.Date {
	return timezone.NewDate(d.Year, d.Month+1, 1).AddDays(-1)
}
