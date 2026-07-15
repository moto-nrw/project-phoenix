package active

import (
	"context"
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
}

func (m *wtmMockSessionReader) GetHistoryByStaffID(_ context.Context, _ int64, from, to timezone.Date) ([]*activeModels.WorkSession, error) {
	var result []*activeModels.WorkSession
	for _, s := range m.sessions {
		if !s.Date.Before(from) && !s.Date.After(to) {
			result = append(result, s)
		}
	}
	return result, nil
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
}

func (m *wtmMockScheduleReader) FindByStaffIDsValidInRange(_ context.Context, _ []int64, _, _ timezone.Date) ([]*configModels.StaffWorkSchedule, error) {
	return m.entries, nil
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
}

func (m *wtmMockSettings) ResolveString(_ context.Context, _ string) (string, error) {
	return m.accountStart, nil
}

// ---- fixture builder --------------------------------------------------------

const wtmStaffID = int64(42)

type wtmFixture struct {
	svc       *workTimeMonthService
	sessions  *wtmMockSessionReader
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
		absences: &wtmMockAbsenceReader{},
		shifts:   &wtmMockShiftReader{},
		schedules: &wtmMockScheduleReader{entries: []*configModels.StaffWorkSchedule{
			{StaffID: wtmStaffID, DayOfWeek: configModels.DayMonday, TargetMinutes: 480, RotationLength: 1, ValidFrom: timezone.NewDate(2020, time.January, 1)},
		}},
		models:   &wtmMockModelReader{},
		settings: &wtmMockSettings{accountStart: "2026-06-01"},
	}
	svc := NewWorkTimeMonthService(f.sessions, f.absences,
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

func TestWTMMonthSummary_InvalidMonth(t *testing.T) {
	f := newWTMFixture()
	_, err := f.svc.GetMonthSummary(context.Background(), wtmStaffID, 2026, 13)
	assert.Error(t, err)
	_, err = f.svc.GetMonthSummary(context.Background(), wtmStaffID, 1999, 1)
	assert.Error(t, err)
}
