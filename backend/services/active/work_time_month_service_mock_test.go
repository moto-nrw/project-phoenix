package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- func-field mocks -------------------------------------------------------

type wtmMockSessionReader struct {
	sessions []*activeModels.WorkSession
	calls    int
	lastFrom timezone.Date
}

func (m *wtmMockSessionReader) GetHistoryByStaffID(_ context.Context, _ int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	m.calls++
	m.lastFrom = from
	var result []*activeModels.WorkSession
	rangeStart := from.BerlinMidnight()
	rangeEnd := to.AddDays(1).BerlinMidnight()
	for _, s := range m.sessions {
		end := time.Now()
		if s.CheckOutTime != nil {
			end = *s.CheckOutTime
		}
		if s.CheckInTime.Before(rangeEnd) && end.After(rangeStart) {
			result = append(result, s)
		}
	}
	return result, nil
}

type wtmMockBreakReader struct {
	activeBreaks map[int64]*activeModels.WorkSessionBreak
}

func (m *wtmMockBreakReader) GetActiveBySessionID(_ context.Context, sessionID int64) (*activeModels.WorkSessionBreak, error) {
	return m.activeBreaks[sessionID], nil
}

type wtmMockAbsenceReader struct {
	absences []*activeModels.StaffAbsence
}

func (m *wtmMockAbsenceReader) GetByStaffAndDateRange(_ context.Context, _ int64, from, to timezone.Date) ([]*activeModels.StaffAbsence, error) {
	var result []*activeModels.StaffAbsence
	for _, a := range m.absences {
		if !a.DateEnd.Before(from) && !a.DateStart.After(to) {
			result = append(result, a)
		}
	}
	return result, nil
}

type wtmMockStaffReader struct {
	staff *userModels.Staff
}

func (m *wtmMockStaffReader) FindByID(_ context.Context, _ any) (*userModels.Staff, error) {
	return m.staff, nil
}

type wtmMockScheduleReader struct {
	entries []*configModels.StaffWorkSchedule
	// hasHistory models snapshots that exist outside the queried range; the
	// reader above ignores the range and always returns `entries`.
	hasHistory bool
}

func (m *wtmMockScheduleReader) FindByStaffIDsValidInRange(_ context.Context, _ []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
	return m.entries, nil
}

func (m *wtmMockScheduleReader) HasScheduleHistory(_ context.Context, _ int64) (bool, error) {
	return m.hasHistory || len(m.entries) > 0, nil
}

type wtmMockModelReader struct {
	model  *configModels.WorkTimeModel
	called bool
}

func (m *wtmMockModelReader) FindByID(_ context.Context, _ int64) (*configModels.WorkTimeModel, error) {
	m.called = true
	return m.model, nil
}

type wtmMockShiftReader struct {
	shifts []*scheduleModels.StaffShift
}

func (m *wtmMockShiftReader) FindByStaffAndDateRange(_ context.Context, _ int64, from, to timezone.Date) ([]*scheduleModels.StaffShift, error) {
	var result []*scheduleModels.StaffShift
	for _, s := range m.shifts {
		if !s.Date.Before(from) && !s.Date.After(to) {
			result = append(result, s)
		}
	}
	return result, nil
}

type wtmMockSettings struct {
	accountStart string
	err          error
}

func (m *wtmMockSettings) ResolveString(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.accountStart, nil
}

// ---- fixture builder --------------------------------------------------------

const wtmStaffID = int64(42)

type wtmFixture struct {
	svc       *workTimeMonthService
	sessions  *wtmMockSessionReader
	breaks    *wtmMockBreakReader
	absences  *wtmMockAbsenceReader
	shifts    *wtmMockShiftReader
	schedules *wtmMockScheduleReader
	models    *wtmMockModelReader
	settings  *wtmMockSettings
}

// newWTMFixture builds the service with a Monday-only 480-minute schedule
// valid since 2020, no data, account start 2026-06-01, today = 2026-07-15.
func newWTMFixture() *wtmFixture {
	f := &wtmFixture{
		sessions: &wtmMockSessionReader{},
		breaks:   &wtmMockBreakReader{activeBreaks: map[int64]*activeModels.WorkSessionBreak{}},
		absences: &wtmMockAbsenceReader{},
		shifts:   &wtmMockShiftReader{},
		schedules: &wtmMockScheduleReader{entries: []*configModels.StaffWorkSchedule{
			{StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 480, RotationLength: 1, ValidFrom: timezone.NewDate(2020, time.January, 1)},
		}},
		models:   &wtmMockModelReader{},
		settings: &wtmMockSettings{accountStart: "2026-06-01"},
	}
	svc := NewWorkTimeMonthService(f.sessions, f.breaks, f.absences,
		&wtmMockStaffReader{staff: &userModels.Staff{Model: base.Model{ID: wtmStaffID}}},
		f.schedules, f.models, f.shifts, f.settings, nil).(*workTimeMonthService)
	svc.todayFunc = func() timezone.Date { return timezone.NewDate(2026, time.July, 15) }
	f.svc = svc
	return f
}

func wtmSession(date timezone.Date, startHour, minutes, breakMinutes int) *activeModels.WorkSession {
	checkIn := time.Date(date.Year, date.Month, date.Day, startHour, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(time.Duration(minutes+breakMinutes) * time.Minute)
	return &activeModels.WorkSession{
		StaffID:      wtmStaffID,
		Date:         date,
		Status:       activeModels.WorkSessionStatusPresent,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		BreakMinutes: breakMinutes,
	}
}

func wtmShift(date timezone.Date, minutes int, cancelled bool) *scheduleModels.StaffShift {
	start := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
	return &scheduleModels.StaffShift{
		StaffID:   wtmStaffID,
		Date:      date,
		StartTime: start,
		EndTime:   start.Add(time.Duration(minutes) * time.Minute),
		Cancelled: cancelled,
	}
}

// ---- tests ------------------------------------------------------------------

// The Übertrag chain walks every month from the account start to the requested
// one, so the request sizes the work. A card for 2100 would make the server
// aggregate — and query — seven decades of days for a reader that cannot
// exist; the bound turns that into a 400 before the first query (#1842).
func TestWTMMonthSummary_RejectsFarFutureMonth(t *testing.T) {
	f := newWTMFixture()

	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2100, 12)

	require.ErrorIs(t, err, ErrMonthOutOfRange)
	assert.Zero(t, f.sessions.calls, "the bound must reject before touching the repositories")
}

// The bound is a year of headroom, not a ban on the future: month-by-month
// forward navigation stays usable against planned shifts and absences.
func TestWTMMonthSummary_AcceptsMonthWithinFutureBound(t *testing.T) {
	f := newWTMFixture()
	ctx := context.Background()

	// today = 2026-07-15, so +12 months is 2027-07: the last accepted month.
	summary, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2027, 7)
	require.NoError(t, err)
	assert.Equal(t, 2027, summary.Year)

	_, err = f.svc.GetMonthSummary(ctx, wtmStaffID, 2027, 8)
	require.ErrorIs(t, err, ErrMonthOutOfRange, "one month past the bound is rejected")
}

// The account start setting is not range-checked (any valid date is accepted),
// so a stale/misconfigured value years back would otherwise make every
// current-month card aggregate and query every intervening day — on a poll.
// A start older than maxCarryChainMonths is rejected rather than silently
// clamped to the floor: clamping would drop the balance of every month before
// the floor and present a wrong opening/closing Stundenkonto as authoritative
// (#1842). No real Stundenkonto reaches back this far.
func TestWTMMonthSummary_RejectsAncientAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2000-01-01" // 26 years before the requested month
	ctx := context.Background()

	// today = 2026-07-15; requesting the current month. The floor is
	// 2026-07 minus 120 months = 2016-07-01; a start of 2000-01-01 is older.
	_, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2026, 7)
	require.ErrorIs(t, err, ErrAccountStartTooOld)

	// The chain must not even query when it rejects: no decades-wide walk.
	assert.True(t, f.sessions.lastFrom.IsZero(),
		"rejected request must not query sessions, but walked from %s",
		f.sessions.lastFrom.String())
}

// A realistic account start (well within the bound) is never clamped: the chain
// still begins exactly on the configured date, so the bound cannot silently
// drop carry for a legitimate Stundenkonto.
func TestWTMMonthSummary_RealisticAccountStartNotClamped(t *testing.T) {
	f := newWTMFixture() // account start 2026-06-01, today 2026-07-15
	ctx := context.Background()

	_, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2026, 7)
	require.NoError(t, err)

	assert.Equal(t, timezone.NewDate(2026, time.June, 1), f.sessions.lastFrom,
		"an account start within the bound must not be clamped")
}

// June 2026 Mondays: 1, 8, 15, 22, 29 → 5 × 480 = 2400 target minutes.
func TestWTMMonthSummary_PastMonthComputation(t *testing.T) {
	f := newWTMFixture()
	ctx := context.Background()

	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.June, 1), 8, 480, 30),
		wtmSession(timezone.NewDate(2026, time.June, 8), 8, 240, 0),
	}
	f.absences.absences = []*activeModels.StaffAbsence{
		// Full sick Monday: credits 480, counts 1 sick day.
		{Model: base.Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeSick, Status: activeModels.AbsenceStatusReported,
			DateStart: timezone.NewDate(2026, time.June, 15), DateEnd: timezone.NewDate(2026, time.June, 15)},
		// Overlapping second absence on the same day: must NOT double-credit.
		{Model: base.Model{ID: 2}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeSick, Status: activeModels.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, time.June, 15), DateEnd: timezone.NewDate(2026, time.June, 16)},
		// Half-day vacation Monday: credits 240, counts 0.5 vacation days.
		{Model: base.Model{ID: 3}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation, Status: activeModels.AbsenceStatusApproved,
			HalfDay: true, DateStart: timezone.NewDate(2026, time.June, 22), DateEnd: timezone.NewDate(2026, time.June, 22)},
		// Vacation on a Tuesday (target 0): no credit, no day count.
		{Model: base.Model{ID: 4}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation, Status: activeModels.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, time.June, 2), DateEnd: timezone.NewDate(2026, time.June, 2)},
		// Requested (not approved) vacation: never counts.
		{Model: base.Model{ID: 5}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation, Status: activeModels.AbsenceStatusRequested,
			DateStart: timezone.NewDate(2026, time.June, 29), DateEnd: timezone.NewDate(2026, time.June, 29)},
	}
	f.shifts.shifts = []*scheduleModels.StaffShift{
		wtmShift(timezone.NewDate(2026, time.June, 1), 180, false),
		wtmShift(timezone.NewDate(2026, time.June, 8), 240, true), // cancelled → not counted
	}

	summary, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2026, 6)
	require.NoError(t, err)

	assert.Equal(t, 2400, summary.TargetMinutes)
	assert.Equal(t, 2400, summary.TargetMinutesToDate, "past month is never pro-rated")
	assert.Equal(t, 720, summary.ActualMinutes)
	assert.Equal(t, 480, summary.CreditedSickMinutes)
	assert.Equal(t, 240, summary.CreditedVacationMinutes)
	assert.Equal(t, 0, summary.CreditedOtherMinutes)
	assert.InDelta(t, 1.0, summary.SickDays, 0.001)
	assert.InDelta(t, 0.5, summary.VacationDays, 0.001)
	require.NotNil(t, summary.PlannedShiftMinutes)
	assert.Equal(t, 180, *summary.PlannedShiftMinutes)
	// 720 + 480 + 240 − 2400
	assert.Equal(t, -960, summary.BalanceMinutes)
	assert.Equal(t, 0, summary.CarryInMinutes, "account start = June → nothing to carry")
	assert.Equal(t, -960, summary.ClosingBalanceMinutes)
}

func TestWTMMonthSummary_NoShiftsMeansNilPlanned(t *testing.T) {
	f := newWTMFixture()
	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.Nil(t, summary.PlannedShiftMinutes, "no Dienstplan rows → nil, not 0")
}

func TestWTMMonthSummary_LiveCarry(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-05-01"
	// May 2026 Mondays: 4, 11, 18, 25 → 4 × 480 = 1920 target; one 480 session.
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.May, 4), 8, 480, 0),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.Equal(t, 480-1920, summary.CarryInMinutes)
	assert.Equal(t, summary.CarryInMinutes+summary.BalanceMinutes, summary.ClosingBalanceMinutes)

	// Live semantics: a correction in May immediately changes June's carry.
	f.sessions.sessions = append(f.sessions.sessions,
		wtmSession(timezone.NewDate(2026, time.May, 11), 8, 480, 0))
	summary, err = f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.Equal(t, 960-1920, summary.CarryInMinutes, "carry must recompute live")
}

func TestWTMMonthSummary_CurrentMonthProRated(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"
	// July 2026 Mondays: 6, 13, 20, 27. Today = Jul 15 → to-date counts 6 + 13.
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 6), 8, 480, 0),
	}
	f.absences.absences = []*activeModels.StaffAbsence{
		// Future vacation (Jul 20) inside the current month: no credit yet.
		{Model: base.Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeVacation, Status: activeModels.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, time.July, 20), DateEnd: timezone.NewDate(2026, time.July, 20)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 4*480, summary.TargetMinutes)
	assert.Equal(t, 2*480, summary.TargetMinutesToDate)
	assert.Equal(t, 0, summary.CreditedVacationMinutes, "future absence days don't credit yet")
	assert.Equal(t, 480-960, summary.BalanceMinutes)
}

func TestWTMMonthSummary_ModelFallback(t *testing.T) {
	f := newWTMFixture()
	f.schedules.entries = nil
	modelID := int64(9)
	staffReader := &wtmMockStaffReader{staff: &userModels.Staff{Model: base.Model{ID: wtmStaffID}, WorkTimeModelID: &modelID}}
	f.svc.staffRepo = staffReader
	f.models.model = &configModels.WorkTimeModel{
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2020, time.January, 1),
		Entries: []*configModels.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: configModels.DayMonday, TargetMinutes: 300},
		},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.True(t, f.models.called, "work-time model fallback must be consulted")
	assert.Equal(t, 5*300, summary.TargetMinutes)
}

// A mid-month account start binds the anchor month to the exact day: days
// before it belong to no balance, in the card itself and in every later
// carry. June 2026 Mondays from the 15th on: 15, 22, 29 → 3 × 480.
func TestWTMMonthSummary_MidMonthAccountStartClampsAnchorMonth(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-06-15"
	f.sessions.sessions = []*activeModels.WorkSession{
		// Before the account start — must be invisible.
		wtmSession(timezone.NewDate(2026, time.June, 1), 8, 480, 0),
		wtmSession(timezone.NewDate(2026, time.June, 15), 8, 480, 0),
	}
	f.absences.absences = []*activeModels.StaffAbsence{
		// Sick Monday before the account start — must not credit.
		{Model: base.Model{ID: 1}, StaffID: wtmStaffID, AbsenceType: activeModels.AbsenceTypeSick, Status: activeModels.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, time.June, 8), DateEnd: timezone.NewDate(2026, time.June, 8)},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.Equal(t, 3*480, summary.TargetMinutes, "Soll starts on the account start date, not on the 1st")
	assert.Equal(t, 3*480, summary.TargetMinutesToDate)
	assert.Equal(t, 480, summary.ActualMinutes, "sessions before the account start don't count")
	assert.Equal(t, 0, summary.CreditedSickMinutes, "absences before the account start don't credit")
	assert.Equal(t, 480-1440, summary.BalanceMinutes)
	assert.Equal(t, 0, summary.CarryInMinutes)
}

func TestWTMMonthSummary_MidMonthAccountStartCarry(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-06-15"
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.June, 1), 8, 480, 0),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, -1440, summary.CarryInMinutes,
		"July's Übertrag is June 15–30 only; June 1–14 predates the account")
}

// A month entirely before the account start is summarized standalone.
func TestWTMMonthSummary_MonthBeforeAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-06-15"

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 5)
	require.NoError(t, err)
	assert.Equal(t, 4*480, summary.TargetMinutes, "May 2026 has 4 Mondays")
	assert.Equal(t, 0, summary.CarryInMinutes)
}

// A failing settings read must surface, not silently fall back to January:
// the fallback would return a materially wrong Stundenkonto with HTTP 200.
func TestWTMMonthSummary_SettingsErrorPropagates(t *testing.T) {
	f := newWTMFixture()
	f.settings.err = errors.New("settings unavailable")

	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account start date")
}

func TestWTMMonthSummary_InvalidMonth(t *testing.T) {
	f := newWTMFixture()
	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 13)
	assert.Error(t, err)
	_, err = f.svc.GetMonthSummary(context.Background(), wtmStaffID, 1999, 1)
	assert.Error(t, err)
}

// A future-dated session (the admin correction service accepts them) must not
// enter the balance: targets and credits are clamped at today, so counting its
// Ist would invent a plus in the current month and carry it forward forever.
func TestWTMMonthSummary_FutureSessionExcludedFromActual(t *testing.T) {
	f := newWTMFixture()

	// today = 2026-07-15; July Mondays up to today: 6, 13 → 960 target_to_date.
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 13), 8, 480, 0),
		// Dated after today — must be ignored entirely.
		wtmSession(timezone.NewDate(2026, time.July, 20), 8, 480, 0),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 480, summary.ActualMinutes, "only the session on or before today counts")
	assert.Equal(t, 2*480, summary.TargetMinutesToDate)
	assert.Equal(t, 480-2*480, summary.BalanceMinutes)
}

// The future-dated session must not leak into a later month's Übertrag either
// — the carry sums the same per-month balances.
func TestWTMMonthSummary_FutureSessionExcludedFromCarry(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 20), 8, 480, 0),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 8)
	require.NoError(t, err)
	// July: target_to_date 960 (Mondays 6, 13), actual 0 → carry −960.
	assert.Equal(t, -960, summary.CarryInMinutes, "future session must not inflate the carry")
}

func TestWTMMonthSummary_AssignsOvernightSessionToBothDays(t *testing.T) {
	f := newWTMFixture()
	start := timezone.NewDate(2026, time.June, 30).BerlinMidnight().Add(22 * time.Hour)
	end := start.Add(4 * time.Hour)
	f.sessions.sessions = []*activeModels.WorkSession{{
		StaffID:      wtmStaffID,
		Date:         timezone.NewDate(2026, time.June, 30),
		Status:       activeModels.WorkSessionStatusPresent,
		CheckInTime:  start,
		CheckOutTime: &end,
	}}

	june, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.Equal(t, 120, june.ActualMinutes)

	july, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 120, july.ActualMinutes)
}

// Editing a schedule overwrites users.staff.rotation_anchor_date. A historical
// version carrying its own anchor must keep the parity it was written with,
// instead of being re-parityed by the current anchor (#1842).
func TestWTMMonthSummary_HistoricalRowAnchorWinsOverStaffAnchor(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-06-01"

	// The staff-level anchor now sits one week off the version's own anchor,
	// which flips A/B parity for every June week if it is applied.
	staffAnchor := timezone.NewDate(2026, time.June, 8)
	versionAnchor := timezone.NewDate(2026, time.June, 1)
	svc := NewWorkTimeMonthService(f.sessions, f.breaks, f.absences,
		&wtmMockStaffReader{staff: &userModels.Staff{Model: base.Model{ID: wtmStaffID}, RotationAnchorDate: &staffAnchor}},
		f.schedules, f.models, f.shifts, f.settings, nil).(*workTimeMonthService)
	svc.todayFunc = func() timezone.Date { return timezone.NewDate(2026, time.July, 15) }

	// 2-week rotation: week A Mondays = 480, week B Mondays = 0.
	f.schedules.entries = []*configModels.StaffWorkSchedule{
		{
			StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 480,
			WeekIndex: 0, RotationLength: 2,
			ValidFrom:          timezone.NewDate(2020, time.January, 1),
			RotationAnchorDate: &versionAnchor,
		},
	}

	summary, err := svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	// Anchored on Jun 1: week A Mondays are Jun 1, 15, 29 → 3 × 480.
	// Under the staff anchor (Jun 8) it would be Jun 8, 22 → 2 × 480.
	assert.Equal(t, 3*480, summary.TargetMinutes, "the version's own anchor must decide the parity")
}

// GetDailyTargets is what the daily table reads, and it must resolve each day
// against the schedule version valid ON that day — the card and the rows
// beneath it are otherwise free to disagree.
func TestWTMDailyTargets_UsesDateValidScheduleVersion(t *testing.T) {
	f := newWTMFixture()

	// Old version: Mondays 480, closed on 2026-07-01. New version: Mondays 240.
	closedAt := timezone.NewDate(2026, time.July, 1)
	f.schedules.entries = []*configModels.StaffWorkSchedule{
		{
			StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 480, RotationLength: 1,
			ValidFrom: timezone.NewDate(2020, time.January, 1), ValidUntil: &closedAt,
		},
		{
			StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 240, RotationLength: 1,
			ValidFrom: closedAt,
		},
	}

	targets, err := f.svc.GetDailyTargets(context.Background(), wtmStaffID,
		timezone.NewDate(2026, time.June, 29), timezone.NewDate(2026, time.July, 6))
	require.NoError(t, err)
	require.Len(t, targets, 8)

	byDate := make(map[timezone.Date]int, len(targets))
	for _, target := range targets {
		byDate[target.Date] = target.TargetMinutes
	}
	assert.Equal(t, 480, byDate[timezone.NewDate(2026, time.June, 29)], "Monday under the old version")
	assert.Equal(t, 240, byDate[timezone.NewDate(2026, time.July, 6)], "Monday under the new version")
	assert.Equal(t, 0, byDate[timezone.NewDate(2026, time.June, 30)], "Tuesday is a planned day off")
}

// A mid-month account start makes the card skip every day of the anchor month
// before the start date. The table reads GetDailyTargets, so it has to skip the
// same days — otherwise its Soll column bills days the card never counted and
// the two contradict each other on the very first month of the account.
func TestWTMDailyTargets_ZeroesDaysBeforeMidMonthAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-08"

	targets, err := f.svc.GetDailyTargets(context.Background(), wtmStaffID,
		timezone.NewDate(2026, time.June, 29), timezone.NewDate(2026, time.July, 13))
	require.NoError(t, err)

	byDate := make(map[timezone.Date]int, len(targets))
	for _, target := range targets {
		byDate[target.Date] = target.TargetMinutes
	}
	assert.Equal(t, 0, byDate[timezone.NewDate(2026, time.July, 6)],
		"Monday in the anchor month but before the start day carries no Soll")
	assert.Equal(t, 480, byDate[timezone.NewDate(2026, time.July, 13)],
		"Monday after the start day keeps its Soll")
	assert.Equal(t, 480, byDate[timezone.NewDate(2026, time.June, 29)],
		"a month WHOLLY before the anchor is summarized standalone over its full length, so it stays fully counted")
}

// The guarantee the table depends on, stated end to end: for a mid-month start
// the per-day targets must sum to exactly the card's Summe Soll.
func TestWTMDailyTargets_SumMatchesCardForMidMonthStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-08"
	ctx := context.Background()

	targets, err := f.svc.GetDailyTargets(ctx, wtmStaffID,
		timezone.NewDate(2026, time.July, 1), timezone.NewDate(2026, time.July, 31))
	require.NoError(t, err)
	sum := 0
	for _, target := range targets {
		sum += target.TargetMinutes
	}

	summary, err := f.svc.GetMonthSummary(ctx, wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, summary.TargetMinutes, sum, "table rows and Monatskarte must cover the same span")
	assert.Equal(t, 3*480, sum, "Mondays from the start day on: Jul 13, 20, 27")
}

func TestWTMRangeAggregate_ZeroesBalanceBeforeMidWeekAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-08"

	aggregate, err := f.svc.GetRangeAggregate(
		context.Background(),
		wtmStaffID,
		timezone.NewDate(2026, time.July, 6),
		timezone.NewDate(2026, time.July, 12),
	)
	require.NoError(t, err)

	assert.Zero(t, aggregate.TargetMinutes,
		"the Monday before the Wednesday account start carries no Soll")
	assert.Zero(t, aggregate.BalanceMinutes,
		"a day excluded from Soll must also be excluded from the balance delta")
}

func TestWTMRangeAggregate_ExcludesActualBeforeMidWeekAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-08"
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 6), 8, 480, 0),
		wtmSession(timezone.NewDate(2026, time.July, 8), 8, 120, 0),
	}

	aggregate, err := f.svc.GetRangeAggregate(
		context.Background(),
		wtmStaffID,
		timezone.NewDate(2026, time.July, 6),
		timezone.NewDate(2026, time.July, 12),
	)
	require.NoError(t, err)

	assert.Equal(t, 120, aggregate.ActualMinutes,
		"only work on or after the Wednesday account start counts")
	assert.Equal(t, 120, aggregate.BalanceMinutes,
		"the displayed Ist and balance cover the same account-start range")
}

func TestWTMRangeAggregate_ExcludesPreviousMonthBeforeAccountStart(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.June, 29), 8, 60, 0),
		wtmSession(timezone.NewDate(2026, time.July, 1), 8, 120, 0),
	}

	aggregate, err := f.svc.GetRangeAggregate(
		context.Background(),
		wtmStaffID,
		timezone.NewDate(2026, time.June, 29),
		timezone.NewDate(2026, time.July, 5),
	)
	require.NoError(t, err)

	assert.Zero(t, aggregate.TargetMinutes, "the Monday before the account start carries no Soll")
	assert.Equal(t, 120, aggregate.ActualMinutes, "only work on or after the account start counts")
	assert.Equal(t, 120, aggregate.BalanceMinutes, "the weekly delta starts at the account anchor")
}

func TestWTMRangeAggregate_UnsetAccountStartUsesPeriodEndYear(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = ""
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2025, time.December, 29), 8, 120, 0),
	}

	aggregate, err := f.svc.GetRangeAggregate(
		context.Background(),
		wtmStaffID,
		timezone.NewDate(2025, time.December, 29),
		timezone.NewDate(2026, time.January, 4),
	)
	require.NoError(t, err)

	assert.Zero(t, aggregate.TargetMinutes, "the default account starts on January 1 of the period-end year")
	assert.Zero(t, aggregate.ActualMinutes, "December work before that default account start is excluded")
	assert.Zero(t, aggregate.BalanceMinutes, "the ISO week aggregate starts with the new account year")
}

func TestPrefetchedMonthService_FiltersSessionsToRequestedRange(t *testing.T) {
	accountStart := timezone.NewDate(2026, time.July, 8)
	prefetch := &monthPrefetch{
		staff: map[int64]*userModels.Staff{
			wtmStaffID: {Model: base.Model{ID: wtmStaffID}},
		},
		sessions: map[int64][]*activeModels.WorkSession{
			wtmStaffID: {
				wtmSession(accountStart.AddDays(-2), 8, 60, 0),
				wtmSession(accountStart, 8, 120, 0),
			},
		},
		settings: newMemoSettingsResolver(&wtmMockSettings{accountStart: accountStart.String()}),
	}
	svc := newPrefetchedMonthService(prefetch, nil)

	summary, err := svc.GetMonthSummaryAtMonthEnd(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 120, summary.ActualMinutes,
		"the prefetched session before the account start must not enter the month bucket")
}

// An unset account start must not zero anything: chainAnchor then falls back to
// January 1st, and no day can precede January 1st of its own year.
func TestWTMDailyTargets_UnsetAccountStartZeroesNothing(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = ""

	targets, err := f.svc.GetDailyTargets(context.Background(), wtmStaffID,
		timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 31))
	require.NoError(t, err)

	sum := 0
	for _, target := range targets {
		sum += target.TargetMinutes
	}
	assert.Equal(t, 4*480, sum, "all four January Mondays keep their Soll")
}

func TestWTMDailyTargets_RejectsInvalidRange(t *testing.T) {
	f := newWTMFixture()
	ctx := context.Background()

	_, err := f.svc.GetDailyTargets(ctx, wtmStaffID,
		timezone.NewDate(2026, time.July, 10), timezone.NewDate(2026, time.July, 1))
	assert.Error(t, err, "to before from")

	_, err = f.svc.GetDailyTargets(ctx, wtmStaffID,
		timezone.NewDate(2020, time.January, 1), timezone.NewDate(2026, time.July, 1))
	assert.Error(t, err, "range beyond the cap")
}

// Every range rejection is the caller's mistake and must be recognizable as
// one: the handlers render ErrInvalidTargetRange as 400 and everything else as
// 500, so an oversized window must not reach the internal-error branch.
func TestWTMDailyTargets_InvalidRangeIsTyped(t *testing.T) {
	f := newWTMFixture()
	ctx := context.Background()

	cases := map[string]struct{ from, to timezone.Date }{
		"missing from": {timezone.Date{}, timezone.NewDate(2026, time.July, 1)},
		"missing to":   {timezone.NewDate(2026, time.July, 1), timezone.Date{}},
		"inverted":     {timezone.NewDate(2026, time.July, 10), timezone.NewDate(2026, time.July, 1)},
		"over the cap": {timezone.NewDate(2020, time.January, 1), timezone.NewDate(2026, time.July, 1)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.GetDailyTargets(ctx, wtmStaffID, tc.from, tc.to)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTargetRange)
		})
	}
}

// The work-time model may only stand in when no schedule was EVER written.
// With snapshots on record but none valid in the queried range, the days are
// genuinely zero — applying the current model would charge Soll for days the
// schedule didn't exist yet, and would contradict a wider query, which loads
// the snapshot rows and resolves those same days to zero.
func TestWTMMonthSummary_NoModelFallbackBeforeFirstSnapshot(t *testing.T) {
	f := newWTMFixture()
	f.schedules.entries = nil
	f.schedules.hasHistory = true
	modelID := int64(9)
	f.svc.staffRepo = &wtmMockStaffReader{staff: &userModels.Staff{Model: base.Model{ID: wtmStaffID}, WorkTimeModelID: &modelID}}
	f.models.model = &configModels.WorkTimeModel{
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2020, time.January, 1),
		Entries: []*configModels.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: configModels.DayMonday, TargetMinutes: 300},
		},
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 6)
	require.NoError(t, err)
	assert.False(t, f.models.called, "a range outside the snapshots must not fall back to the model")
	assert.Equal(t, 0, summary.TargetMinutes)
}

// An open session with a running break: WorkSession.BreakMinutes caches ended
// breaks only, so the live card would count the break as worked time and show
// a growing plus until the break is closed.
func TestWTMMonthSummary_ActiveBreakDeductedFromLiveActual(t *testing.T) {
	f := newWTMFixture()
	now := time.Now()
	// A live open session belongs to the current day; pin today to the real
	// calendar date so the now-relative timestamps stay on the session's own
	// day and the day cap does not clamp the still-running span.
	today := timezone.DateFromTime(now)
	f.svc.todayFunc = func() timezone.Date { return today }
	f.settings.accountStart = today.String()

	session := &activeModels.WorkSession{
		Model:        base.Model{ID: 77},
		StaffID:      wtmStaffID,
		Date:         today,
		Status:       activeModels.WorkSessionStatusPresent,
		CheckInTime:  now.Add(-120 * time.Minute),
		BreakMinutes: 0, // break still running → not in the cache yet
	}
	f.sessions.sessions = []*activeModels.WorkSession{session}
	f.breaks.activeBreaks[session.ID] = &activeModels.WorkSessionBreak{
		Model:     base.Model{ID: 5},
		SessionID: session.ID,
		StartedAt: now.Add(-30 * time.Minute),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, today.Year, int(today.Month))
	require.NoError(t, err)
	// 120 minutes since check-in, 30 of them on break → 90 worked.
	assert.InDelta(t, 90, summary.ActualMinutes, 1,
		"the running break must not count as worked time")
}

// An ended break is already in the session cache; deducting it again would
// halve the recorded break.
func TestWTMMonthSummary_EndedBreakNotDeductedTwice(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"
	f.sessions.sessions = []*activeModels.WorkSession{
		wtmSession(timezone.NewDate(2026, time.July, 13), 8, 480, 30),
	}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, 480, summary.ActualMinutes)
}

// The admin correction API accepts a check_out_time later than the current
// instant. On TODAY's session the now cap keeps only the minutes already
// worked from counting; the future tail must not credit Ist against a target
// that only accrues up to today. The fixture pins today to the real calendar
// day so the session's own day does not clamp the span before now does.
func TestWTMMonthSummary_TodayFutureCheckOutClampedAtNow(t *testing.T) {
	f := newWTMFixture()
	now := time.Now()
	todayDate := timezone.DateFromTime(now)
	f.svc.todayFunc = func() timezone.Date { return todayDate }
	f.settings.accountStart = todayDate.String() // single month, no long chain

	checkOut := now.Add(120 * time.Minute)
	f.sessions.sessions = []*activeModels.WorkSession{{
		Model:        base.Model{ID: 88},
		StaffID:      wtmStaffID,
		Date:         todayDate,
		Status:       activeModels.WorkSessionStatusPresent,
		CheckInTime:  now.Add(-60 * time.Minute),
		CheckOutTime: &checkOut,
	}}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, todayDate.Year, int(todayDate.Month))
	require.NoError(t, err)
	assert.InDelta(t, 60, summary.ActualMinutes, 1,
		"only the minutes worked up to now count, not the future part of the checkout")
}

// A historical session an admin corrected with a check-out later than its own
// day (or a future instant) must count only up to that day's end, never
// through now. Without the day cap the whole span since check-in bills as Ist
// and, via the carry chain, inflates every later balance.
func TestWTMMonthSummary_PastFutureCheckOutClampedAtDayEnd(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"

	sessionDay := timezone.NewDate(2026, time.July, 10) // well before today (07-15)
	checkIn := time.Date(2026, time.July, 10, 8, 0, 0, 0, timezone.Berlin)
	checkOut := time.Date(2030, time.January, 1, 0, 0, 0, 0, timezone.Berlin) // absurd future
	f.sessions.sessions = []*activeModels.WorkSession{{
		Model:        base.Model{ID: 88},
		StaffID:      wtmStaffID,
		Date:         sessionDay,
		Status:       activeModels.WorkSessionStatusPresent,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
	}}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	expected := int(sessionDay.EndOfDay().Sub(checkIn).Minutes())
	assert.Equal(t, expected, summary.ActualMinutes,
		"a past session's Ist is capped at its own day's end, not counted through now")
}

// A session left open on a past day must not bill every minute since check-in
// through now — that would add days or months of presence that never happened.
// The day cap bounds it at the session day's end.
func TestWTMMonthSummary_PastUnclosedSessionClampedAtDayEnd(t *testing.T) {
	f := newWTMFixture()
	f.settings.accountStart = "2026-07-01"

	sessionDay := timezone.NewDate(2026, time.July, 10)
	checkIn := time.Date(2026, time.July, 10, 8, 0, 0, 0, timezone.Berlin)
	f.sessions.sessions = []*activeModels.WorkSession{{
		Model:       base.Model{ID: 88},
		StaffID:     wtmStaffID,
		Date:        sessionDay,
		Status:      activeModels.WorkSessionStatusPresent,
		CheckInTime: checkIn,
		// CheckOutTime nil: never checked out.
	}}

	summary, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 7)
	require.NoError(t, err)
	expected := int(sessionDay.EndOfDay().Sub(checkIn).Minutes())
	assert.Equal(t, expected, summary.ActualMinutes,
		"an unclosed past session's Ist is bounded at its own day's end")
}
