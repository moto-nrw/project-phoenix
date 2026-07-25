package active_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// countingQueryHook counts SQL statements so a test can prove the overview's
// query count does not grow with the number of staff members.
type countingQueryHook struct{ n atomic.Int64 }

func (h *countingQueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	h.n.Add(1)
	return ctx
}

func (h *countingQueryHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// overviewFixture arranges a tenant with N staff members, each with a contract
// on every weekday, a worked session today, and (optionally) a planned shift.
type overviewFixture struct {
	tenantID int64
	db       *bun.DB
	repos    *repositories.Factory
	ctx      context.Context
	staff    []*userModels.Staff
	svc      active.StaffOverviewService
	monthSvc active.WorkTimeMonthService
	today    timezone.Date
}

// newOverviewFixture builds `count` staff members. The account start is the
// first of the current month, so every fixture row sits inside the window.
func newOverviewFixture(t *testing.T, count int) *overviewFixture {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(tenantID)
	today := timezone.TodayDate()

	f := &overviewFixture{
		tenantID: tenantID, db: db, repos: repos, ctx: ctx, today: today,
	}

	t.Cleanup(func() {
		for _, table := range []string{
			"active.staff_month_balance_snapshots",
			"active.staff_balance_adjustments",
			"active.staff_vacation_quotas",
			"active.staff_absences",
			"active.work_sessions",
			"schedule.staff_shifts",
			"config.staff_work_schedules",
		} {
			_, _ = db.NewDelete().ModelTableExpr(table+" AS t").Where("t.tenant_id = ?", tenantID).Exec(context.Background())
		}
		ids := make([]int64, 0, len(f.staff))
		for _, staff := range f.staff {
			ids = append(ids, staff.ID)
		}
		testpkg.CleanupStaffFixtures(t, db, ids...)
		testpkg.CleanupTenantTestData(t, db, tenantID)
	})

	for i := range count {
		staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Ueber", "Sicht")
		f.staff = append(f.staff, staff)
		f.addSchedule(t, staff.ID, 480)
		f.addSession(t, staff.ID, today, 4*time.Hour)
		if i == 0 {
			f.addShift(t, staff.ID, today)
		}
	}

	monthStart := timezone.NewDate(today.Year, today.Month, 1)
	settings := wtmIntSettings{accountStart: monthStart.String()}
	f.monthSvc = active.NewWorkTimeMonthService(
		repos.WorkSession, repos.WorkSessionBreak, repos.StaffAbsence, repos.Staff,
		repos.StaffWorkSchedule, repos.WorkTimeModel, repos.StaffShift,
		settings, nil,
	)
	f.monthSvc.SetAdjustmentReader(repos.StaffBalanceAdjust)
	f.monthSvc.SetSnapshotReader(repos.StaffMonthSnapshot)

	f.svc = f.newOverviewService(settings)
	return f
}

func (f *overviewFixture) newOverviewService(settings wtmIntSettings) active.StaffOverviewService {
	return active.NewStaffOverviewService(
		f.repos.Staff, f.repos.WorkSession, f.repos.WorkSessionBreak, f.repos.StaffAbsence,
		f.repos.StaffBalanceAdjust, f.repos.StaffVacationQuota, f.repos.StaffMonthSnapshot,
		f.repos.StaffWorkSchedule, f.repos.WorkTimeModel, f.repos.StaffShift,
		settings, nil,
	)
}

// addSchedule gives the staff member the same contractual Soll on all seven
// weekdays, so the expected values do not depend on which day the test runs.
func (f *overviewFixture) addSchedule(t *testing.T, staffID int64, targetMinutes int) {
	t.Helper()
	for day := range 7 {
		row := &configModels.StaffWorkSchedule{
			TenantID:      f.tenantID,
			StaffID:       staffID,
			DayOfWeek:     day,
			TargetMinutes: targetMinutes,
			WeekIndex:     0, RotationLength: 1,
			ValidFrom: timezone.NewDate(2020, time.January, 1),
		}
		_, err := f.db.NewInsert().Model(row).ModelTableExpr("config.staff_work_schedules").Exec(f.ctx)
		require.NoError(t, err)
	}
}

func (f *overviewFixture) addSession(t *testing.T, staffID int64, date timezone.Date, duration time.Duration) *activeModels.WorkSession {
	t.Helper()
	checkIn := time.Date(date.Year, date.Month, date.Day, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(duration)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        date,
		Status:      activeModels.WorkSessionStatusPresent,
		Source:      activeModels.WorkSessionSourceApp,
		CheckInTime: checkIn, CheckOutTime: &checkOut,
		CreatedBy: staffID,
	}
	session.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.WorkSession.Create(f.ctx, session))
	return session
}

// reopenSession clears check-out on the staff member's session for that day —
// the staff member is currently at work. It updates the fixture's existing row
// because work_sessions is unique per staff and date.
func (f *overviewFixture) reopenSession(t *testing.T, staffID int64, date timezone.Date) {
	t.Helper()
	res, err := f.db.NewUpdate().
		Table("active.work_sessions").
		Set("check_out_time = NULL").
		Where("staff_id = ? AND date = ? AND tenant_id = ?", staffID, date, f.tenantID).
		Exec(f.ctx)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
}

// addShift plans a shift that covers the whole day, so it also covers "now".
func (f *overviewFixture) addShift(t *testing.T, staffID int64, date timezone.Date) {
	t.Helper()
	shift := &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      date,
		StartTime: time.Date(2000, time.January, 1, 0, 1, 0, 0, time.UTC),
		EndTime:   time.Date(2000, time.January, 1, 23, 59, 0, 0, time.UTC),
		CreatedBy: staffID,
	}
	shift.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffShift.Create(f.ctx, shift))
}

func (f *overviewFixture) addAbsence(t *testing.T, staffID int64, absenceType, status string, start, end timezone.Date) {
	t.Helper()
	absence := &activeModels.StaffAbsence{
		StaffID:     staffID,
		AbsenceType: absenceType,
		Status:      status,
		DateStart:   start,
		DateEnd:     end,
		CreatedBy:   staffID,
	}
	absence.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffAbsence.Create(f.ctx, absence))
}

func (f *overviewFixture) setEmploymentType(t *testing.T, staffID int64, employmentType string) {
	t.Helper()
	_, err := f.db.NewUpdate().
		Table("users.staff").
		Set("employment_type = ?", employmentType).
		Where("id = ?", staffID).
		Exec(f.ctx)
	require.NoError(t, err)
}

// TestTimeTrackingOverview_MatchesMonthSummary is the anti-drift guarantee: the
// cross-staff row must equal what the per-staff Monatskarte reports, because
// both run the same math. Without this the aggregate view could quietly
// contradict the detail view a school admin drills into.
func TestTimeTrackingOverview_MatchesMonthSummary(t *testing.T) {
	f := newOverviewFixture(t, 3)

	// Make the three staff members genuinely different: a part-time contract,
	// a sick day, a payout booking, and one with no schedule at all.
	f.addSchedule(t, f.staff[1].ID, 240)
	yesterday := f.today.AddDays(-1)
	if monthOfDate(yesterday) == monthOfDate(f.today) {
		f.addAbsence(t, f.staff[1].ID, activeModels.AbsenceTypeSick, activeModels.AbsenceStatusReported, yesterday, yesterday)
	}
	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       f.staff[2].ID,
		Type:          activeModels.BalanceAdjustmentTypePayout,
		MinutesDelta:  -30,
		EffectiveDate: f.today,
		Note:          "Auszahlung",
		DecidedBy:     f.staff[0].ID,
		DecidedAt:     time.Now(),
	}
	adjustment.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffBalanceAdjust.Create(f.ctx, adjustment))

	overview, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	require.Len(t, overview.Rows, 3)

	byStaff := make(map[int64]active.TimeTrackingOverviewRow, len(overview.Rows))
	for _, row := range overview.Rows {
		byStaff[row.StaffID] = row
	}
	for _, staff := range f.staff {
		row, ok := byStaff[staff.ID]
		require.True(t, ok, "staff %d missing from the overview", staff.ID)

		expected, err := f.monthSvc.GetMonthSummary(f.ctx, staff.ID, f.today.Year, int(f.today.Month))
		require.NoError(t, err)
		assert.Equal(t, expected.TargetMinutesToDate, row.SollMinutes, "soll for staff %d", staff.ID)
		assert.Equal(t, expected.ActualMinutes, row.IstMinutes, "ist for staff %d", staff.ID)
		assert.Equal(t, expected.ClosingBalanceMinutes, row.BalanceMinutes, "saldo for staff %d", staff.ID)
	}
}

// TestTimeTrackingOverview_VacationMatchesQuotaEndpoint pins Resturlaub against
// the per-staff endpoint it mirrors.
func TestTimeTrackingOverview_VacationMatchesQuotaEndpoint(t *testing.T) {
	f := newOverviewFixture(t, 2)

	// An approved vacation day earlier this year plus a still-requested one:
	// the requested days must be reserved, not counted as taken.
	yearStart := timezone.NewDate(f.today.Year, time.January, 5)
	f.addAbsence(t, f.staff[0].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved, yearStart, yearStart.AddDays(2))
	future := timezone.NewDate(f.today.Year, time.December, 1)
	f.addAbsence(t, f.staff[0].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusRequested, future, future)

	absenceSvc := active.NewStaffAbsenceService(
		f.repos.StaffAbsence, f.repos.WorkSession, f.repos.StaffVacationQuota,
		f.repos.StaffAbsenceAudit, wtmIntSettings{}, f.monthSvc,
	)

	overview, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)

	for _, row := range overview.Rows {
		expected, err := absenceSvc.GetVacationQuotaSummary(f.ctx, row.StaffID, f.today.Year)
		require.NoError(t, err)
		assert.InDelta(t, expected.RemainingDays, row.RemainingVacationDays, 0.001,
			"Resturlaub for staff %d must match the per-staff endpoint", row.StaffID)
	}
}

// TestTimeTrackingOverview_QueryCountIsConstant is the only test that protects
// the performance property. Without it the prefetch adapters could silently
// regress to per-staff reads and nothing would fail.
func TestTimeTrackingOverview_QueryCountIsConstant(t *testing.T) {
	small := newOverviewFixture(t, 1)
	hook := &countingQueryHook{}
	small.db.AddQueryHook(hook)
	_, err := small.svc.GetTimeTrackingOverview(small.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	oneStaff := hook.n.Load()

	large := newOverviewFixture(t, 8)
	hookLarge := &countingQueryHook{}
	large.db.AddQueryHook(hookLarge)
	_, err = large.svc.GetTimeTrackingOverview(large.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	eightStaff := hookLarge.n.Load()

	assert.Equal(t, oneStaff, eightStaff,
		"query count must not grow with the number of staff members (1 staff: %d, 8 staff: %d)",
		oneStaff, eightStaff)
}

// TestDashboardSummary_KPIs checks each counter, including the priority collapse
// that keeps sick and vacation from double-counting the same person.
func TestDashboardSummary_KPIs(t *testing.T) {
	f := newOverviewFixture(t, 4)

	f.reopenSession(t, f.staff[0].ID, f.today)
	f.addAbsence(t, f.staff[1].ID, activeModels.AbsenceTypeSick, activeModels.AbsenceStatusReported, f.today, f.today)
	f.addAbsence(t, f.staff[2].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved, f.today, f.today)
	// Overlapping sick + vacation on the same person: counts as sick only.
	f.addAbsence(t, f.staff[3].ID, activeModels.AbsenceTypeSick, activeModels.AbsenceStatusReported, f.today, f.today)
	f.addAbsence(t, f.staff[3].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved, f.today, f.today)
	// A pending request must be counted but must not affect sick/vacation.
	f.addAbsence(t, f.staff[2].ID, activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusRequested,
		f.today.AddDays(30), f.today.AddDays(31))

	summary, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodMonth)
	require.NoError(t, err)

	assert.Equal(t, 4, summary.ActiveStaffCount)
	assert.Equal(t, 2, summary.SickToday, "staff[1] and staff[3]")
	assert.Equal(t, 1, summary.VacationToday, "staff[3] counts as sick, not vacation")
	assert.Equal(t, 1, summary.CurrentlyClockedIn, "only staff[0] has an open session")
	assert.Equal(t, 1, summary.ExpectedClockedIn, "only staff[0] has a shift covering now")
	assert.Equal(t, 1, summary.PendingRequestsCount)
}

// TestDashboardSummary_WeekVsMonth — the saldo is an account balance and must
// not change with the selected period; Soll and Ist must.
func TestDashboardSummary_WeekVsMonth(t *testing.T) {
	f := newOverviewFixture(t, 2)

	month, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodMonth)
	require.NoError(t, err)
	week, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodWeek)
	require.NoError(t, err)

	assert.Equal(t, month.SaldoSchoolTotalMinutes, week.SaldoSchoolTotalMinutes,
		"the Stundenkonto is not a period quantity")
	assert.LessOrEqual(t, week.Period.SollMinutes, month.Period.SollMinutes,
		"a week inside the month cannot carry more Soll than the month")

	// The default is month.
	def, err := f.svc.GetDashboardSummary(f.ctx, "")
	require.NoError(t, err)
	assert.Equal(t, month.Period, def.Period)
}

func TestDashboardSummary_WeekExcludesAdjustmentsOutsideRange(t *testing.T) {
	f := newOverviewFixture(t, 1)

	before, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodWeek)
	require.NoError(t, err)

	weekStart := configModels.MondayOf(f.today)
	effectiveDate := timezone.NewDate(f.today.Year, f.today.Month, 1)
	if !effectiveDate.Before(weekStart) {
		effectiveDate = f.today.AddDays(1)
	}
	require.Equal(t, f.today.Month, effectiveDate.Month,
		"fixture must place the adjustment inside the prefetched month")
	require.True(t, effectiveDate.Before(weekStart) || effectiveDate.After(f.today),
		"fixture adjustment must lie outside the requested week")

	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       f.staff[0].ID,
		Type:          activeModels.BalanceAdjustmentTypePayout,
		MinutesDelta:  -60,
		EffectiveDate: effectiveDate,
		Note:          "Außerhalb der Woche",
		DecidedBy:     f.staff[0].ID,
		DecidedAt:     time.Now(),
	}
	adjustment.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffBalanceAdjust.Create(f.ctx, adjustment))

	after, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodWeek)
	require.NoError(t, err)

	assert.Equal(t, before.Period, after.Period,
		"an adjustment outside [weekStart, today] must not alter the week aggregate")
}

func TestDashboardSummary_RejectsUnknownPeriod(t *testing.T) {
	f := newOverviewFixture(t, 1)

	_, err := f.svc.GetDashboardSummary(f.ctx, "quarter")
	require.ErrorIs(t, err, active.ErrOverviewInvalid)
}

// TestTimeTrackingOverview_Filters — the employment filter narrows the rows
// before the computation, the saldo range narrows them after it, and neither
// changes the values of the surviving rows.
func TestTimeTrackingOverview_Filters(t *testing.T) {
	f := newOverviewFixture(t, 3)
	f.setEmploymentType(t, f.staff[0].ID, userModels.EmploymentTypeFullTime)
	f.setEmploymentType(t, f.staff[1].ID, userModels.EmploymentTypePartTime)

	all, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	require.Len(t, all.Rows, 3)
	unfiltered := make(map[int64]int, 3)
	for _, row := range all.Rows {
		unfiltered[row.StaffID] = row.BalanceMinutes
	}

	fullTime, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{
		EmploymentType: userModels.EmploymentTypeFullTime,
	})
	require.NoError(t, err)
	require.Len(t, fullTime.Rows, 1)
	assert.Equal(t, f.staff[0].ID, fullTime.Rows[0].StaffID)
	assert.Equal(t, unfiltered[f.staff[0].ID], fullTime.Rows[0].BalanceMinutes,
		"filtering must not change the computed values")

	// Every fixture staff member worked less than their Soll, so the balance is
	// negative; a floor of 0 must exclude everybody.
	zero := 0
	none, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SaldoMin: &zero})
	require.NoError(t, err)
	assert.Empty(t, none.Rows)
	assert.NotNil(t, none.Rows, "rows must serialize as [] and never null")

	max := 0
	everyone, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SaldoMax: &max})
	require.NoError(t, err)
	assert.Len(t, everyone.Rows, 3)
}

func TestTimeTrackingOverview_RejectsBadFilters(t *testing.T) {
	f := newOverviewFixture(t, 1)
	low, high := 100, 10

	_, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{EmploymentType: "bogus"})
	require.ErrorIs(t, err, active.ErrOverviewInvalid)

	_, err = f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SortBy: "bogus"})
	require.ErrorIs(t, err, active.ErrOverviewInvalid)

	_, err = f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SaldoMin: &low, SaldoMax: &high})
	require.ErrorIs(t, err, active.ErrOverviewInvalid)
}

// TestTimeTrackingOverview_SortIsStable — ListAllWithPerson has no ORDER BY, so
// without an explicit staff-ID tiebreak equal balances would swap places
// between identical requests.
func TestTimeTrackingOverview_SortIsStable(t *testing.T) {
	f := newOverviewFixture(t, 4)

	first, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SortBy: active.OverviewSortBalance})
	require.NoError(t, err)
	for range 3 {
		again, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{SortBy: active.OverviewSortBalance})
		require.NoError(t, err)
		require.Equal(t, first.Rows, again.Rows, "repeated requests must return the same order")
	}

	desc, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{
		SortBy: active.OverviewSortBalance, Descending: true,
	})
	require.NoError(t, err)
	require.Len(t, desc.Rows, 4)
	for i := 1; i < len(desc.Rows); i++ {
		assert.GreaterOrEqual(t, desc.Rows[i-1].BalanceMinutes, desc.Rows[i].BalanceMinutes)
	}
}

// TestTimeTrackingOverview_EmptyAndDegenerate covers the shapes that would
// otherwise 500 or return null.
func TestTimeTrackingOverview_EmptyAndDegenerate(t *testing.T) {
	f := newOverviewFixture(t, 0)

	overview, err := f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	assert.NotNil(t, overview.Rows)
	assert.Empty(t, overview.Rows)

	summary, err := f.svc.GetDashboardSummary(f.ctx, active.OverviewPeriodMonth)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.ActiveStaffCount)
	assert.Equal(t, 0, summary.SaldoSchoolTotalMinutes)

	// A staff member with neither a schedule nor a work-time model: Soll 0,
	// Ist 0, no error.
	bare := testpkg.CreateTestStaffForTenant(t, f.db, f.tenantID, "Ohne", "Plan")
	f.staff = append(f.staff, bare)
	overview, err = f.svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)
	assert.Equal(t, 0, overview.Rows[0].SollMinutes)
	assert.Equal(t, 0, overview.Rows[0].IstMinutes)
	assert.Equal(t, 0, overview.Rows[0].BalanceMinutes)
}

// TestTimeTrackingOverview_RespectsFrozenMonths ties the two halves of #1417
// together: a closed past month must shape the overview's saldo exactly as it
// shapes the detail card.
func TestTimeTrackingOverview_RespectsFrozenMonths(t *testing.T) {
	f := newOverviewFixture(t, 1)
	staffID := f.staff[0].ID

	// Account start in the previous month so there is something to freeze.
	prevMonth := timezone.NewDate(f.today.Year, f.today.Month, 1).AddDays(-1)
	settings := wtmIntSettings{accountStart: timezone.NewDate(prevMonth.Year, prevMonth.Month, 1).String()}
	monthSvc := active.NewWorkTimeMonthService(
		f.repos.WorkSession, f.repos.WorkSessionBreak, f.repos.StaffAbsence, f.repos.Staff,
		f.repos.StaffWorkSchedule, f.repos.WorkTimeModel, f.repos.StaffShift,
		settings, nil,
	)
	monthSvc.SetAdjustmentReader(f.repos.StaffBalanceAdjust)
	monthSvc.SetSnapshotReader(f.repos.StaffMonthSnapshot)
	closeSvc := active.NewStaffMonthCloseService(f.repos.StaffMonthSnapshot, monthSvc, f.repos.Staff, settings, nil)
	svc := f.newOverviewService(settings)

	_, err := closeSvc.CloseMonth(f.ctx, staffID, prevMonth.Year, int(prevMonth.Month), "Abschluss")
	require.NoError(t, err)

	overview, err := svc.GetTimeTrackingOverview(f.ctx, active.OverviewFilters{})
	require.NoError(t, err)
	require.Len(t, overview.Rows, 1)

	expected, err := monthSvc.GetMonthSummary(f.ctx, staffID, f.today.Year, int(f.today.Month))
	require.NoError(t, err)
	require.True(t, expected.CarryInFrozen, "the previous month is frozen")
	assert.Equal(t, expected.ClosingBalanceMinutes, overview.Rows[0].BalanceMinutes,
		"the overview must splice at the frozen month just like the detail card")
}

func monthOfDate(d timezone.Date) int { return d.Year*12 + int(d.Month) }
