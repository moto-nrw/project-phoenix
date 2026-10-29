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

type careTimeTargets map[int64]map[calendar.Date]studentpresence.CareTimeTarget

func (t careTimeTargets) CareTimeTargets(_ context.Context, ids []int64, _, _ calendar.Date) (map[int64]map[calendar.Date]studentpresence.CareTimeTarget, error) {
	result := make(map[int64]map[calendar.Date]studentpresence.CareTimeTarget, len(ids))
	for _, id := range ids {
		result[id] = t[id]
	}
	return result, nil
}

type attendanceSpans []ports.AttendanceSpan

func (s attendanceSpans) ListAttendanceSpans(_ context.Context, _, _ calendar.Date, ids []int64) ([]ports.AttendanceSpan, error) {
	if len(ids) == 0 {
		return s, nil
	}
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var out []ports.AttendanceSpan
	for _, span := range s {
		if wanted[span.StudentID] {
			out = append(out, span)
		}
	}
	return out, nil
}

func (attendanceSpans) ListAttendanceDays(context.Context, string, string) ([]studentpresence.AttendanceDay, error) {
	return nil, nil
}

// carePlanFacts is the care plan as the report reads it: status days and
// booked care times.
type carePlanFacts struct {
	careTimeTargets
	statusDays []ports.StatusDay
}

func (f carePlanFacts) StatusDays(context.Context, calendar.Date, calendar.Date) ([]ports.StatusDay, error) {
	return f.statusDays, nil
}

func clock(hour, minute int) *calendar.WallClock {
	c, err := calendar.NewWallClock(hour, minute, 0, 0)
	if err != nil {
		panic(err)
	}
	return &c
}

func booked(start, end *calendar.WallClock) studentpresence.CareTimeTarget {
	return studentpresence.CareTimeTarget{Status: studentpresence.CareTimeScheduled, Arrival: start, Pickup: end}
}

func berlin(date calendar.Date, hour, minute int) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, calendar.Berlin)
}

func span(studentID int64, date calendar.Date, fromHour, fromMinute, toHour, toMinute int) ports.AttendanceSpan {
	out := berlin(date, toHour, toMinute)
	return ports.AttendanceSpan{StudentID: studentID, Date: date, FirstCheckIn: berlin(date, fromHour, fromMinute), LastCheckOut: &out}
}

func minutes(n int) *int { return &n }

// One week of a child booked Monday to Friday 08:00 to 15:00, covering every
// kind of day the comparison distinguishes.
func careTimeWeek() (Config, calendar.Date, calendar.Date) {
	const child = int64(7)
	mon := calendar.NewDate(2026, 8, 17)
	tue, wed, thu, fri := mon.AddDays(1), mon.AddDays(2), mon.AddDays(3), mon.AddDays(4)
	sat, sun, nextMon := mon.AddDays(5), mon.AddDays(6), mon.AddDays(7)
	week := booked(clock(8, 0), clock(15, 0))
	open := ports.AttendanceSpan{StudentID: child, Date: nextMon, FirstCheckIn: berlin(nextMon, 8, 0)}
	cfg := Config{
		Students: careTimeStudents{{ID: child, FirstName: "Mia", LastName: "Berg", SchoolClass: "2a", GroupName: "Füchse", EnrolledOn: enrolledEveryDay}},
		StatusDays: carePlanFacts{careTimeTargets: careTimeTargets{child: {
			mon: week, tue: week, wed: week, thu: week, fri: week,
			sat:     {Status: studentpresence.CareTimeNotScheduled},
			sun:     {Status: studentpresence.CareTimeNotScheduled},
			nextMon: week,
		}}, statusDays: []ports.StatusDay{{StudentID: child, Date: wed, Status: "sick"}}},
		Attendance: attendanceSpans{
			span(child, mon, 7, 50, 15, 10),
			span(child, tue, 7, 30, 15, 0),
			span(child, sat, 10, 0, 12, 0),
			open,
		},
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
	week := booked(clock(8, 0), clock(15, 0))
	students := cfg.Students.(careTimeStudents)
	students = append(students,
		&ports.StatisticsStudent{ID: 8, FirstName: "Ben", LastName: "Arnold", EnrolledOn: enrolledEveryDay},
		&ports.StatisticsStudent{ID: 9, FirstName: "Lea", LastName: "Adam", EnrolledOn: enrolledEveryDay},
	)
	cfg.Students = students
	targets := cfg.StatusDays.(carePlanFacts).careTimeTargets
	targets[8] = map[calendar.Date]studentpresence.CareTimeTarget{from: week, from.AddDays(1): week}
	targets[9] = map[calendar.Date]studentpresence.CareTimeTarget{from: week}
	cfg.Attendance = append(cfg.Attendance.(attendanceSpans),
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
