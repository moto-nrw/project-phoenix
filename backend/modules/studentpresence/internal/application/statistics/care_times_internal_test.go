package statistics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type careTimeStudents []*ports.StatisticsStudent

func (s careTimeStudents) FindOverlappingWithGroups(context.Context, calendar.Date, calendar.Date, calendar.Date) ([]*ports.StatisticsStudent, error) {
	return s, nil
}

type statusDayList []ports.StatusDay

func (s statusDayList) StatusDays(context.Context, calendar.Date, calendar.Date) ([]ports.StatusDay, error) {
	return s, nil
}

// shortRetention keeps attendance for days days and status days for a year.
type shortRetention struct{ days int }

func (shortRetention) RoomRetentionDays(context.Context) (int, error)   { return 30, nil }
func (shortRetention) CourseRetentionDays(context.Context) (int, error) { return 365, nil }
func (r shortRetention) AttendanceRetentionDays(context.Context) (int, error) {
	return r.days, nil
}
func (shortRetention) StatusDayRetentionDays(context.Context) (int, error) { return 365, nil }

func wallClockAt(hour, minute int) *calendar.WallClock {
	c, err := calendar.NewWallClock(hour, minute, 0, 0)
	if err != nil {
		panic(err)
	}
	return &c
}

func booked(start, end *calendar.WallClock) ports.PlannedCareDay {
	return ports.PlannedCareDay{Expected: true, Arrival: start, Pickup: end}
}

func berlin(date calendar.Date, hour, minute int) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, calendar.Berlin)
}

func span(studentID int64, date calendar.Date, fromHour, fromMinute, toHour, toMinute int) *ports.Attendance {
	out := berlin(date, toHour, toMinute)
	return &ports.Attendance{StudentID: studentID, Date: date, CheckInTime: berlin(date, fromHour, fromMinute), CheckOutTime: &out}
}

func minutes(n int) *int { return &n }

// One week of a child booked Monday to Friday 08:00 to 15:00, covering every
// kind of day the comparison distinguishes.
func careTimeWeek() (Config, calendar.Date, calendar.Date) {
	const child = int64(7)
	mon := calendar.NewDate(2026, 8, 17)
	tue, wed, thu, fri := mon.AddDays(1), mon.AddDays(2), mon.AddDays(3), mon.AddDays(4)
	sat, sun, nextMon := mon.AddDays(5), mon.AddDays(6), mon.AddDays(7)
	week := booked(wallClockAt(8, 0), wallClockAt(15, 0))
	open := &ports.Attendance{StudentID: child, Date: nextMon, CheckInTime: berlin(nextMon, 8, 0)}
	cfg := Config{
		Students: careTimeStudents{{ID: child, FirstName: "Mia", LastName: "Berg", SchoolClass: "2a", GroupName: "Füchse", EnrolledOn: enrolledEveryDay}},
		PlannedCareDays: plannedStub{child: {
			mon: week, tue: week, wed: week, thu: week, fri: week,
			sat: {}, sun: {}, nextMon: week,
		}},
		StatusDays: statusDayList{{StudentID: child, Date: wed, Status: "sick"}},
		AttendanceIntervals: &attendanceRowsStub{rows: []*ports.Attendance{
			// Monday has two rows: the span runs from the first check-in to
			// the last check-out.
			span(child, mon, 7, 50, 11, 0),
			span(child, mon, 12, 0, 15, 10),
			span(child, tue, 7, 30, 15, 0),
			span(child, sat, 10, 0, 12, 0),
			open,
		}},
		Retention:   reportRetention{},
		ClosingDays: dateSet{fri: true},
		Now:         func() time.Time { return berlin(nextMon, 12, 0) },
	}
	return cfg, mon, nextMon
}

func TestStudentCareTimesComparesBookedAndActualTimePerDay(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to)
	require.NoError(t, err)

	require.Equal(t, studentpresence.CareTimeToleranceMinutes, result.ToleranceMinutes)
	kinds := map[calendar.Date]studentpresence.CareTimeDayKind{}
	for _, day := range result.Days {
		kinds[day.Date] = day.Kind
	}
	assert.Equal(t, map[calendar.Date]studentpresence.CareTimeDayKind{
		from:            studentpresence.CareTimeDayWithinPlan,
		from.AddDays(1): studentpresence.CareTimeDayOverrun,
		from.AddDays(2): studentpresence.CareTimeDaySick,
		from.AddDays(3): studentpresence.CareTimeDayAbsent,
		from.AddDays(4): studentpresence.CareTimeDayClosed,
		from.AddDays(5): studentpresence.CareTimeDayUnbooked,
		to:              studentpresence.CareTimeDayOpen,
	}, kinds, "the unbooked Sunday without attendance is not listed")

	monday := result.Days[0]
	assert.Equal(t, minutes(420), monday.PlannedMinutes)
	assert.Equal(t, minutes(440), monday.ActualMinutes)
	assert.Equal(t, minutes(20), monday.DeviationMinutes)
	assert.Equal(t, 10, monday.EarlyMinutes)
	assert.Equal(t, 10, monday.LateMinutes)
	assert.Equal(t, "07:50", monday.ActualStart.Time().Format("15:04"), "Ist is the Berlin wall clock of the check-in")

	tuesday := result.Days[1]
	assert.Equal(t, 30, tuesday.EarlyMinutes)
	assert.Equal(t, minutes(30), tuesday.DeviationMinutes)

	wednesday := result.Days[2]
	assert.Nil(t, wednesday.DeviationMinutes, "a sick day is no shortfall")
	thursday := result.Days[3]
	assert.Equal(t, minutes(-420), thursday.DeviationMinutes, "an unexplained absence is the whole booked time short")
	assert.Nil(t, result.Days[4].DeviationMinutes, "a closing day is no shortfall")

	saturday := result.Days[5]
	assert.Nil(t, saturday.PlannedStart)
	assert.Equal(t, minutes(120), saturday.DeviationMinutes)

	open := result.Days[6]
	assert.Nil(t, open.ActualEnd)
	assert.Nil(t, open.DeviationMinutes)

	assert.Equal(t, studentpresence.CareTimeSummary{
		OverrunDays: 1, UnbookedDays: 1, AbsentDays: 1, ExtraMinutes: 150,
		PlannedMinutes: 1260, ActualMinutes: 1010, DeviationMinutes: -250,
	}, result.Summary)
}

func TestCareTimeOverrunsListsChildrenWithOverrunsMostFirst(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	// Ben overruns on both Monday and Tuesday; Lea keeps to her booking.
	week := booked(wallClockAt(8, 0), wallClockAt(15, 0))
	students := cfg.Students.(careTimeStudents)
	students = append(students,
		&ports.StatisticsStudent{ID: 8, FirstName: "Ben", LastName: "Arnold", EnrolledOn: enrolledEveryDay},
		&ports.StatisticsStudent{ID: 9, FirstName: "Lea", LastName: "Adam", EnrolledOn: enrolledEveryDay},
	)
	cfg.Students = students
	planned := cfg.PlannedCareDays.(plannedStub)
	planned[8] = map[calendar.Date]ports.PlannedCareDay{from: week, from.AddDays(1): week}
	planned[9] = map[calendar.Date]ports.PlannedCareDay{from: week}
	rows := cfg.AttendanceIntervals.(*attendanceRowsStub)
	rows.rows = append(rows.rows,
		span(8, from, 7, 0, 15, 0), span(8, from.AddDays(1), 8, 0, 16, 0), span(9, from, 8, 0, 15, 15))
	audit := &reportAudit{}
	cfg.AccessLog = audit

	result, err := NewService(cfg).CareTimeOverruns(context.Background(), studentpresence.CareTimeFilters{From: from, To: to}, studentpresence.StatisticsActor{AccountID: 3, Role: "admin"})
	require.NoError(t, err)

	require.Len(t, result.Students, 2, "a child without overruns is not listed")
	assert.Equal(t, int64(8), result.Students[0].StudentID, "Ben and Mia both have two overruns; the tie sorts by last name")
	assert.Equal(t, 2, result.Students[0].Summary.OverrunCount())
	assert.Equal(t, 120, result.Students[0].Summary.ExtraMinutes)
	assert.Equal(t, int64(7), result.Students[1].StudentID)
	assert.Equal(t, "Füchse", result.Students[1].GroupName)
	require.Len(t, audit.events, 1)
	assert.Equal(t, "care_times", audit.events[0].Metadata["sections"])
	assert.Equal(t, "view", audit.events[0].Metadata["action"])
}

func TestStudentCareTimesRejectsAChildOutsideTheSchoolAndBadWindows(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	service := NewService(cfg)
	_, err := service.StudentCareTimes(context.Background(), 99, from, to)
	require.ErrorIs(t, err, studentpresence.ErrCareTimeStudentNotFound)
	_, err = service.StudentCareTimes(context.Background(), 7, to, from)
	require.ErrorIs(t, err, studentpresence.ErrInvalidStatisticsRange)
	_, err = service.StudentCareTimes(context.Background(), 7, from, to.AddDays(1))
	require.ErrorIs(t, err, studentpresence.ErrInvalidStatisticsRange, "the window may not reach into the future")
}

// A cancelled day and a day without any care plan are no shortfall and no
// overrun; older days than the attendance retention are not compared.
func TestStudentCareTimesSkipsDaysBeyondRetentionAndNamesCancelledDays(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	planned := cfg.PlannedCareDays.(plannedStub)
	planned[7][from.AddDays(3)] = ports.PlannedCareDay{Cancelled: true}
	planned[7][from.AddDays(5)] = ports.PlannedCareDay{Expected: true, NoPlan: true}
	cfg.Retention = shortRetention{days: 4}

	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to)
	require.NoError(t, err)

	assert.Equal(t, to.AddDays(-4), result.DataFrom)
	kinds := map[calendar.Date]studentpresence.CareTimeDayKind{}
	for _, day := range result.Days {
		kinds[day.Date] = day.Kind
	}
	assert.Equal(t, map[calendar.Date]studentpresence.CareTimeDayKind{
		from.AddDays(3): studentpresence.CareTimeDayCancelled,
		from.AddDays(4): studentpresence.CareTimeDayClosed,
		from.AddDays(5): studentpresence.CareTimeDayNoPlan,
		to:              studentpresence.CareTimeDayOpen,
	}, kinds, "Monday to Wednesday lie before the retention cutoff")
	assert.Zero(t, result.Summary.OverrunCount(), "attendance without any care plan is no overrun")
}

func TestCareTimeOverrunsFollowsTheClassFilter(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	students := cfg.Students.(careTimeStudents)
	cfg.Students = append(students, &ports.StatisticsStudent{ID: 8, FirstName: "Ben", LastName: "Arnold", SchoolClass: "1a", EnrolledOn: enrolledEveryDay})
	planned := cfg.PlannedCareDays.(plannedStub)
	planned[8] = map[calendar.Date]ports.PlannedCareDay{from: booked(wallClockAt(8, 0), wallClockAt(15, 0))}
	rows := cfg.AttendanceIntervals.(*attendanceRowsStub)
	rows.rows = append(rows.rows, span(8, from, 7, 0, 15, 0))
	cfg.AccessLog = &reportAudit{}

	result, err := NewService(cfg).CareTimeOverruns(context.Background(),
		studentpresence.CareTimeFilters{From: from, To: to, SchoolClasses: []string{"1a"}}, studentpresence.StatisticsActor{AccountID: 3})
	require.NoError(t, err)

	require.Len(t, result.Students, 1)
	assert.Equal(t, int64(8), result.Students[0].StudentID)
}

// Today is not over: a booked child who has not checked in yet is no
// shortfall, and a booked day without any times cannot be compared.
func TestStudentCareTimesWaitsForTodayAndNeedsBookedTimes(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	rows := cfg.AttendanceIntervals.(*attendanceRowsStub)
	rows.rows = rows.rows[:len(rows.rows)-1] // no check-in today yet
	planned := cfg.PlannedCareDays.(plannedStub)
	planned[7][from] = ports.PlannedCareDay{Expected: true}

	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to)
	require.NoError(t, err)

	kinds := map[calendar.Date]studentpresence.CareTimeDayKind{}
	for _, day := range result.Days {
		kinds[day.Date] = day.Kind
	}
	assert.NotContains(t, kinds, to, "today without a check-in is not decided yet")
	assert.Equal(t, studentpresence.CareTimeDayNoPlan, kinds[from], "attended, but the booking names no times")
	assert.Equal(t, 1, result.Summary.AbsentDays, "only Thursday is a shortfall")
}
