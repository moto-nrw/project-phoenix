package api

import (
	"context"
	"testing"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type weekProtocolSeedAttendanceProbe struct {
	day calendar.Date
}

func (p *weekProtocolSeedAttendanceProbe) GroupDayAttendance(_ context.Context, day calendar.Date, _ []int64) ([]timetableModule.WeekProtocolGroupDay, error) {
	p.day = day
	return []timetableModule.WeekProtocolGroupDay{}, nil
}

func (p *weekProtocolSeedAttendanceProbe) SchoolDayAttendance(_ context.Context, day calendar.Date) (timetableModule.WeekProtocolGroupDay, error) {
	p.day = day
	return timetableModule.WeekProtocolGroupDay{}, nil
}

func TestWeekProtocolSeedAttendanceUsesCurrentDayForSundayDraft(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 30)
	probe := &weekProtocolSeedAttendanceProbe{}
	attendance := weekProtocolSeedAttendance{source: probe, today: func() calendar.Date { return today }}

	_, err := attendance.GroupDayAttendance(context.Background(), today.StartOfISOWeek().AddDays(6), nil)

	require.NoError(t, err)
	assert.Equal(t, today, probe.day)
}

// The whole school's day of an open-care seed (#5401) is read like a
// group's: from the current demo day.
func TestWeekProtocolSeedSchoolAttendanceUsesCurrentDayForSundayDraft(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 30)
	probe := &weekProtocolSeedAttendanceProbe{}
	attendance := weekProtocolSeedAttendance{source: probe, today: func() calendar.Date { return today }}

	_, err := attendance.SchoolDayAttendance(context.Background(), today.StartOfISOWeek().AddDays(6))

	require.NoError(t, err)
	assert.Equal(t, today, probe.day)
}
