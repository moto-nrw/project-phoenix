package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	"github.com/moto-nrw/project-phoenix/services"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshotFixture is the shared arrangement for the Monatsabschluss tests:
// one staff member with a Mondays-480 contract, one full August 2025 session,
// plus an admin who performs the close.
//
// The dates are deliberately in the past. todayFunc is unexported, so this
// external test package runs against the real calendar: a future month would
// hit the Soll clamp at today and the close guard would (correctly) refuse.
type snapshotFixture struct {
	tenantID  int64
	staff     int64
	admin     int64
	session   *timerecords.WorkSession
	schedule  *testpkg.StaffWorkScheduleFixture
	repos     *repositories.Factory
	db        *testpkg.DB
	ctx       context.Context
	monthSvc  timetracking.WorkTimeMonthService
	closeSvc  timetracking.StaffMonthCloseService
	adjustSvc timetracking.StaffBalanceAdjustmentService
}

// snapshotSessionSettings satisfies the work-session service's settings
// surface; the snapshot tests exercise no setting-driven behaviour.
type snapshotSessionSettings struct{}

func (snapshotSessionSettings) EnforcePlannedStart(context.Context) (bool, error) { return false, nil }
func (snapshotSessionSettings) RequireDeviationReason(context.Context) (bool, error) {
	return false, nil
}
func (snapshotSessionSettings) DeviationToleranceMinutes(context.Context) (int, error) { return 0, nil }
func (snapshotSessionSettings) TimeTrackingRetentionDays(context.Context) (int, error) { return 0, nil }

// newAdminSessionService builds the admin correction path used to mutate a
// closed month, wired exactly as services/factory.go does.
func (f *snapshotFixture) newAdminSessionService() timetracking.WorkSessionService {
	return timetracking.NewWorkSessionService(
		f.repos.WorkSession, f.repos.WorkSessionBreak, services.NewWorkSessionAudit(f.repos.WorkSessionEdit), f.repos.StaffAbsence,
		f.repos.GroupSupervisor, f.repos.ActiveGroup, services.WorkSessionStaff(f.repos.Staff, repositories.MustNewStaffEmployment(f.db)), services.NewWorkSessionSchedules(f.repos.StaffWorkSchedule), services.NewWorkSessionTimeModels(f.repos.WorkTimeModel),
		snapshotSessionSettings{}, nil, f.db, services.RenderTimeTrackingPDF,
		services.RenderTimeTrackingWorkbook,
	)
}

const (
	snapshotYear       = 2025
	snapshotMonth      = 8 // August 2025
	snapshotNextMonth  = 9
	snapshotSessionDay = 4 // Monday, 2025-08-04
)

func newSnapshotFixture(t *testing.T) *snapshotFixture {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Abschluss", "Mitarbeiter")
	admin := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Abschluss", "Leitung")
	owners := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, owners)
	ctx := testpkg.TenantContext(tenantID)

	t.Cleanup(func() {
		for _, table := range []string{
			"active.staff_month_balance_snapshots",
			"active.staff_balance_adjustments",
			"audit.work_session_edits",
			"active.staff_absences",
			"active.work_sessions",
			"config.staff_work_schedules",
		} {
			_, _ = db.NewDelete().ModelTableExpr(table+" AS t").Where("t.tenant_id = ?", tenantID).Exec(context.Background())
		}
	})

	// Contract: Mondays 480 minutes.
	schedule := testpkg.CreateTestStaffWorkScheduleForTenant(t, db, tenantID, staff.ID, timetracking.DayMonday, 480, scheduleValidFrom)

	checkIn := time.Date(snapshotYear, time.August, snapshotSessionDay, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(8 * time.Hour)
	session := &timerecords.WorkSession{
		StaffID:     staff.ID,
		Date:        timezone.NewDate(snapshotYear, time.August, snapshotSessionDay),
		Status:      workforce.WorkSessionStatusPresent,
		Source:      workforce.WorkSessionSourceApp,
		CheckInTime: checkIn, CheckOutTime: &checkOut,
		CreatedBy: staff.ID,
	}
	session.SetTenantID(tenantID)
	require.NoError(t, repos.WorkSession.Create(ctx, session))

	settings := wtmIntSettings{accountStart: "2025-01-01"}
	monthSvc := timetracking.NewWorkTimeMonthService(
		repos.WorkSession, repos.WorkSessionBreak, repos.StaffAbsence, services.StaffScheduleAssignments(repositories.MustNewStaffEmployment(db)),
		services.NewWorkScheduleTargets(repos.StaffWorkSchedule), services.NewWorkTimeTargetModels(repos.WorkTimeModel), services.NewTimeTrackingShifts(owners.Workforce),
		settings, nil,
		timetracking.WithMonthSnapshots(services.MonthSnapshotCapability(repos.StaffMonthSnapshot)),
		timetracking.WithMonthAdjustments(repos.StaffBalanceAdjust),
	)

	closeSvc := timetracking.NewStaffMonthCloseService(
		services.MonthSnapshotCapability(repos.StaffMonthSnapshot), monthSvc, services.MonthCloseStaff(repos.Staff), settings, nil,
	)
	adjustSvc := timetracking.NewStaffBalanceAdjustmentService(
		repos.StaffBalanceAdjust, monthSvc, settings, nil,
		timetracking.WithAdjustmentSnapshots(services.MonthSnapshotCapability(repos.StaffMonthSnapshot)),
	)

	return &snapshotFixture{
		tenantID: tenantID, staff: staff.ID, admin: admin.ID,
		session: session, schedule: schedule, repos: repos, db: db, ctx: ctx,
		monthSvc: monthSvc, closeSvc: closeSvc, adjustSvc: adjustSvc,
	}
}

// TestMonthClose_FreezesBalanceAgainstRetroactiveSessionEdit is the acceptance
// scenario for #1417: a closed month, then a session inside it corrected
// afterwards. The frozen closing balance and every later carry must stay put,
// and the divergence must be reported as drift rather than swallowed.
func TestMonthClose_FreezesBalanceAgainstRetroactiveSessionEdit(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	augustBefore, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotMonth)
	require.NoError(t, err)
	require.Equal(t, 480, augustBefore.ActualMinutes)

	septemberCarryBefore, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)
	balanceBefore, err := f.monthSvc.GetClosingBalanceAsOf(f.ctx, f.staff, timezone.TodayDate())
	require.NoError(t, err)

	result, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Monatsabschluss August")
	require.NoError(t, err)
	assert.Equal(t, 2, result.ClosedStaff, "close is school-wide: staff member and admin both get a row")
	assert.Equal(t, 0, result.SkippedStaff)

	var frozen *timetracking.MonthSnapshot
	for _, snapshot := range result.Snapshots {
		if snapshot.StaffID == f.staff {
			frozen = snapshot
		}
	}
	require.NotNil(t, frozen)
	assert.Equal(t, augustBefore.ClosingBalanceMinutes, frozen.ClosingBalanceMinutes)

	// Retroactive correction through the admin path: check out two hours
	// earlier, so August loses 120 minutes.
	workSessionSvc := f.newAdminSessionService()
	earlierCheckOut := f.session.CheckInTime.Add(6 * time.Hour)
	notes := "Korrektur: zwei Stunden früher gegangen"
	_, err = workSessionSvc.UpdateSessionAsAdmin(f.ctx, f.admin, f.staff, f.session.ID, timetracking.SessionUpdateRequest{
		CheckOutTime: &earlierCheckOut,
		Notes:        &notes,
	})
	require.NoError(t, err)

	// The frozen month still reports its OWN live numbers...
	augustAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotMonth)
	require.NoError(t, err)
	assert.Equal(t, 360, augustAfter.ActualMinutes, "the correction is visible in the month itself")
	assert.True(t, augustAfter.IsClosed)
	require.NotNil(t, augustAfter.FrozenClosingBalanceMinutes)
	assert.Equal(t, augustBefore.ClosingBalanceMinutes, *augustAfter.FrozenClosingBalanceMinutes)
	assert.Equal(t, -120, augustAfter.DriftMinutes, "drift reports the divergence instead of hiding it")

	// ...but nothing after it moves.
	septemberAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)
	assert.Equal(t, septemberCarryBefore.CarryInMinutes, septemberAfter.CarryInMinutes,
		"September's Übertrag comes from the frozen value, not from re-summing August")
	assert.True(t, septemberAfter.CarryInFrozen)
	require.NotNil(t, septemberAfter.CarryInFrozenFromMonth)
	assert.Equal(t, "2025-08", *septemberAfter.CarryInFrozenFromMonth)

	balanceAfter, err := f.monthSvc.GetClosingBalanceAsOf(f.ctx, f.staff, timezone.TodayDate())
	require.NoError(t, err)
	assert.Equal(t, balanceBefore, balanceAfter, "the current Stundenkonto is unchanged")
}

// TestMonthClose_FreezesBalanceAgainstRetroactiveScheduleChange is the case the
// issue names literally ("Schutz gegen Schedule-Wechsel-Historie"): changing
// the contractual Soll of a past month must not move a closed balance.
func TestMonthClose_FreezesBalanceAgainstRetroactiveScheduleChange(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	septemberBefore, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)

	_, err = f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Monatsabschluss August")
	require.NoError(t, err)

	// Retroactively halve the contractual Soll of every Monday.
	testpkg.SetStaffWorkScheduleTargetMinutes(t, f.db, f.ctx, f.schedule.ID, 240)

	augustAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotMonth)
	require.NoError(t, err)
	assert.NotZero(t, augustAfter.DriftMinutes, "the Soll change shows up as drift")

	septemberAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)
	assert.Equal(t, septemberBefore.CarryInMinutes, septemberAfter.CarryInMinutes,
		"a retroactive Dienstplan change must not move a closed month's Übertrag")
}

// TestMonthClose_ReopenRestoresLiveChain proves the reopen path: the escape
// hatch for a legitimate correction after the freeze.
func TestMonthClose_ReopenRestoresLiveChain(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	septemberBefore, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)

	_, err = f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Monatsabschluss August")
	require.NoError(t, err)

	workSessionSvc := f.newAdminSessionService()
	earlierCheckOut := f.session.CheckInTime.Add(6 * time.Hour)
	notes := "Korrektur: zwei Stunden früher gegangen"
	_, err = workSessionSvc.UpdateSessionAsAdmin(f.ctx, f.admin, f.staff, f.session.ID, timetracking.SessionUpdateRequest{
		CheckOutTime: &earlierCheckOut,
		Notes:        &notes,
	})
	require.NoError(t, err)

	require.NoError(t, f.closeSvc.ReopenMonth(f.ctx, f.staff, f.admin, snapshotYear, snapshotMonth, "Korrektur nachgetragen"))

	septemberAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotNextMonth)
	require.NoError(t, err)
	assert.Equal(t, septemberBefore.CarryInMinutes-120, septemberAfter.CarryInMinutes,
		"after reopening, the correction flows through again")
	assert.False(t, septemberAfter.CarryInFrozen)

	augustAfter, err := f.monthSvc.GetMonthSummary(f.ctx, f.staff, snapshotYear, snapshotMonth)
	require.NoError(t, err)
	assert.False(t, augustAfter.IsClosed)
	assert.Zero(t, augustAfter.DriftMinutes)
}

// TestMonthClose_RejectsUnfinishedMonth pins the most important guard: freezing
// a month that is not over would charge its full Soll against an Ist that stops
// today, and that error would carry forward forever.
func TestMonthClose_RejectsUnfinishedMonth(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)
	today := timezone.TodayDate()

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, today.Year(), int(today.Month()), "zu früh")
	require.ErrorIs(t, err, timetracking.ErrMonthNotClosable)
}

// TestMonthClose_RejectsMissingReason keeps freezing attributable.
func TestMonthClose_RejectsMissingReason(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "   ")
	require.ErrorIs(t, err, timetracking.ErrMonthCloseInvalid)
}

// TestMonthClose_IsIdempotent — a second click must skip, not double-write.
func TestMonthClose_IsIdempotent(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	first, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss")
	require.NoError(t, err)
	second, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss")
	require.NoError(t, err)

	assert.Equal(t, 0, second.ClosedStaff)
	assert.Equal(t, first.ClosedStaff, second.SkippedStaff)
}

// TestMonthClose_RejectsCloseBehindLaterSnapshot prevents a newly frozen older
// month from contradicting the history used by an already frozen later month.
func TestMonthClose_RejectsCloseBehindLaterSnapshot(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotNextMonth, "Abschluss September")
	require.NoError(t, err)

	_, err = f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss August")
	require.ErrorIs(t, err, timetracking.ErrLaterMonthClosed)
}

func TestMonthClose_RetryBehindLaterSnapshotRemainsIdempotent(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss August")
	require.NoError(t, err)
	_, err = f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotNextMonth, "Abschluss September")
	require.NoError(t, err)

	result, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss August")
	require.NoError(t, err)
	assert.Zero(t, result.ClosedStaff)
	assert.Equal(t, 2, result.SkippedStaff)
}

// TestMonthClose_RejectsAdjustmentInClosedMonth — a booking inside a frozen
// month could not move its closing balance, so the ledger refuses it. Work
// sessions stay editable on purpose; that difference is what drift is for.
func TestMonthClose_RejectsAdjustmentInClosedMonth(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss")
	require.NoError(t, err)

	_, err = f.adjustSvc.CreateAdjustment(f.ctx, f.staff, f.admin, timetracking.CreateBalanceAdjustmentRequest{
		Type:          workforce.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: timezone.NewDate(snapshotYear, time.August, 20),
		Note:          "Auszahlung im abgeschlossenen Monat",
	})
	require.ErrorIs(t, err, timetracking.ErrAdjustmentInvalid)
}

// TestMonthClose_RejectedAdjustmentCarriesClosedMonthSentinel pins the
// dedicated sentinel the API layer maps to the stable code
// "adjustment_in_closed_month", so the frontend can explain the month close
// instead of showing a generic validation error (#1417 UI).
func TestMonthClose_RejectedAdjustmentCarriesClosedMonthSentinel(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss")
	require.NoError(t, err)

	_, err = f.adjustSvc.CreateAdjustment(f.ctx, f.staff, f.admin, timetracking.CreateBalanceAdjustmentRequest{
		Type:          workforce.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: timezone.NewDate(snapshotYear, time.August, 20),
		Note:          "Auszahlung im abgeschlossenen Monat",
	})
	require.ErrorIs(t, err, timetracking.ErrAdjustmentInClosedMonth)
}

// TestMonthClose_ReopenRequiresNewestFirst — reopening an older month while a
// later one stays frozen would leave a state no admin can reason about.
func TestMonthClose_ReopenRequiresNewestFirst(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	_, err := f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotMonth, "Abschluss August")
	require.NoError(t, err)
	_, err = f.closeSvc.CloseMonth(f.ctx, f.admin, snapshotYear, snapshotNextMonth, "Abschluss September")
	require.NoError(t, err)

	err = f.closeSvc.ReopenMonth(f.ctx, f.staff, f.admin, snapshotYear, snapshotMonth, "Korrektur")
	require.ErrorIs(t, err, timetracking.ErrLaterMonthClosed)

	// Newest first works.
	require.NoError(t, f.closeSvc.ReopenMonth(f.ctx, f.staff, f.admin, snapshotYear, snapshotNextMonth, "Korrektur"))
	require.NoError(t, f.closeSvc.ReopenMonth(f.ctx, f.staff, f.admin, snapshotYear, snapshotMonth, "Korrektur"))
}

// TestMonthClose_ReopenUnclosedMonthIsNotFound guards the 404 path.
func TestMonthClose_ReopenUnclosedMonthIsNotFound(t *testing.T) {
	t.Parallel()

	f := newSnapshotFixture(t)

	err := f.closeSvc.ReopenMonth(f.ctx, f.staff, f.admin, snapshotYear, snapshotMonth, "Korrektur")
	require.ErrorIs(t, err, timetracking.ErrMonthNotClosed)
}
