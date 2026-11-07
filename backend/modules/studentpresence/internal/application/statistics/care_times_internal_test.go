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

var careTimeActor = studentpresence.StatisticsActor{AccountID: 3, Role: "admin"}

// shortRetention keeps attendance for days days and status days for a year.
type shortRetention struct{ days int }

func (shortRetention) RoomRetentionDays(context.Context) (int, error)   { return 30, nil }
func (shortRetention) CourseRetentionDays(context.Context) (int, error) { return 365, nil }
func (r shortRetention) OccupancyRetentionDays(context.Context) (int, int, error) {
	return 365, r.days, nil
}

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
		AccessLog:   &reportAudit{},
		ClosingDays: dateSet{fri: true},
		Now:         func() time.Time { return berlin(nextMon, 12, 0) },
	}
	return cfg, mon, nextMon
}

func TestStudentCareTimesComparesBookedAndActualTimePerDay(t *testing.T) {
	t.Parallel()
	cfg, from, to := careTimeWeek()
	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to, careTimeActor)
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
	_, err := service.StudentCareTimes(context.Background(), 99, from, to, careTimeActor)
	require.ErrorIs(t, err, studentpresence.ErrCareTimeStudentNotFound)
	_, err = service.StudentCareTimes(context.Background(), 7, to, from, careTimeActor)
	require.ErrorIs(t, err, studentpresence.ErrInvalidStatisticsRange)
	_, err = service.StudentCareTimes(context.Background(), 7, from, to.AddDays(1), careTimeActor)
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

	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to, careTimeActor)
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

	result, err := NewService(cfg).StudentCareTimes(context.Background(), 7, from, to, careTimeActor)
	require.NoError(t, err)

	kinds := map[calendar.Date]studentpresence.CareTimeDayKind{}
	for _, day := range result.Days {
		kinds[day.Date] = day.Kind
	}
	assert.NotContains(t, kinds, to, "today without a check-in is not decided yet")
	assert.Equal(t, studentpresence.CareTimeDayNoPlan, kinds[from], "attended, but the booking names no times")
	assert.Equal(t, 1, result.Summary.AbsentDays, "only Thursday is a shortfall")
}

func classSpan(studentID int64, date calendar.Date, fromHour, fromMinute, toHour, toMinute int) *ports.Attendance {
	row := span(studentID, date, fromHour, fromMinute, toHour, toMinute)
	reason := studentpresence.CheckoutReasonClass
	row.CheckOutReason = &reason
	return row
}

// Soll is the booking and Ist runs from the first check-in to the last check-out
// (#4774). Time in class takes nothing off either side and is only shown (#5656).
func TestClassifyCareTimeDayShowsTimeInClassWithoutDeducting(t *testing.T) {
	t.Parallel()

	const child = int64(7)
	date := calendar.NewDate(2026, 8, 17)
	key := dayKey{child, date}
	classify := func(plan ports.PlannedCareDay, rows ...*ports.Attendance) studentpresence.CareTimeDay {
		attended := attendanceSpans(rows)[key]
		day, listed := classifyCareTimeDay(date, plan, &attended, "", true)
		require.True(t, listed)
		return day
	}
	wholeDay := booked(wallClockAt(7, 30), wallClockAt(16, 0))
	at := func(hour, minute int) calendar.WallClock { return *wallClockAt(hour, minute) }

	t.Run("no return reads as the checkout, with the class time open", func(t *testing.T) {
		t.Parallel()
		day := classify(wholeDay, classSpan(child, date, 7, 30, 8, 30))
		assert.Equal(t, studentpresence.CareTimeDayWithinPlan, day.Kind)
		assert.Equal(t, minutes(60), day.ActualMinutes)
		assert.Equal(t, minutes(510), day.PlannedMinutes)
		assert.Equal(t, minutes(-450), day.DeviationMinutes)
		require.Len(t, day.ClassTimes, 1)
		assert.Equal(t, at(8, 30), day.ClassTimes[0].Start)
		assert.Nil(t, day.ClassTimes[0].End)
	})

	t.Run("return keeps the class hours in both sides and shows the gap", func(t *testing.T) {
		t.Parallel()
		day := classify(wholeDay, span(child, date, 13, 0, 16, 0), classSpan(child, date, 7, 30, 8, 30))
		assert.Equal(t, studentpresence.CareTimeDayWithinPlan, day.Kind)
		assert.Equal(t, minutes(510), day.ActualMinutes)
		assert.Equal(t, minutes(510), day.PlannedMinutes)
		assert.Equal(t, minutes(0), day.DeviationMinutes)
		require.Len(t, day.ClassTimes, 1)
		assert.Equal(t, at(8, 30), day.ClassTimes[0].Start)
		require.NotNil(t, day.ClassTimes[0].End)
		assert.Equal(t, at(13, 0), *day.ClassTimes[0].End)
	})

	t.Run("overrun is measured against the booking", func(t *testing.T) {
		t.Parallel()
		day := classify(booked(wallClockAt(13, 0), wallClockAt(16, 0)),
			classSpan(child, date, 7, 30, 8, 30), span(child, date, 13, 0, 16, 0))
		assert.Equal(t, studentpresence.CareTimeDayOverrun, day.Kind)
		assert.Equal(t, 330, day.EarlyMinutes, "the first check-in is 07:30, the booking starts at 13:00")
		assert.Equal(t, minutes(330), day.DeviationMinutes)
	})

	t.Run("unbooked day counts first check-in to last check-out", func(t *testing.T) {
		t.Parallel()
		day := classify(ports.PlannedCareDay{},
			span(child, date, 12, 0, 14, 0), classSpan(child, date, 8, 0, 9, 0), classSpan(child, date, 7, 0, 7, 30))
		assert.Equal(t, studentpresence.CareTimeDayUnbooked, day.Kind)
		assert.Equal(t, minutes(420), day.ActualMinutes)
		require.Len(t, day.ClassTimes, 2)
		assert.Equal(t, at(7, 30), day.ClassTimes[0].Start, "earliest first, whatever the row order")
		require.NotNil(t, day.ClassTimes[0].End)
		assert.Equal(t, at(8, 0), *day.ClassTimes[0].End)
	})

	t.Run("going home shows no class time", func(t *testing.T) {
		t.Parallel()
		day := classify(wholeDay, span(child, date, 7, 30, 8, 30))
		assert.Empty(t, day.ClassTimes)
		assert.Equal(t, minutes(-450), day.DeviationMinutes)
	})
}

// A day without a check-out is never fully compared, also when an early
// arrival already made it an overrun: its later minutes are unknown (#4683).
func TestComparedDayCountsKeepsOpenDaysIncomplete(t *testing.T) {
	t.Parallel()
	in := wallClockAt(7, 0)
	out := wallClockAt(15, 0)
	end := wallClockAt(16, 0)
	days := []studentpresence.CareTimeDay{
		{Kind: studentpresence.CareTimeDayWithinPlan, ActualStart: in, ActualEnd: out, PlannedEnd: end},
		{Kind: studentpresence.CareTimeDayOverrun, ActualStart: in, ActualEnd: out, PlannedEnd: end},
		{Kind: studentpresence.CareTimeDayUnbooked, ActualStart: in, ActualEnd: out},
		// A module without an end time: the pickup is unknown, so the end of
		// the day cannot be compared.
		{Kind: studentpresence.CareTimeDayWithinPlan, ActualStart: in, ActualEnd: out},
		{Kind: studentpresence.CareTimeDayOverrun, ActualStart: in},
		{Kind: studentpresence.CareTimeDayUnbooked, ActualStart: in},
		{Kind: studentpresence.CareTimeDayOpen, ActualStart: in},
		{Kind: studentpresence.CareTimeDayNoPlan, ActualStart: in, ActualEnd: out},
		{Kind: studentpresence.CareTimeDayAbsent},
	}
	compared, uncompared := comparedDayCounts(days)
	assert.Equal(t, 3, compared, "only days with a check-in, a check-out and a known booked end are compared")
	assert.Equal(t, 5, uncompared, "open days, days without a plan and days without a booked end stay incomplete")
}

// A booked day whose pickup is unknown is never shown as within the
// booking: its departure cannot be compared (#4683).
func TestClassifyCareTimeDayWithoutABookedEnd(t *testing.T) {
	t.Parallel()
	date := calendar.NewDate(2026, 6, 8)
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	checkOut := date.BerlinMidnight().Add(17 * time.Hour)
	planned := studentpresence.PlannedCareDay{Expected: true, Arrival: wallClockAt(8, 0)}

	day, listed := classifyCareTimeDay(date, planned, &attendanceSpan{firstCheckIn: checkIn, lastCheckOut: &checkOut}, "", true)
	require.True(t, listed)
	assert.Equal(t, studentpresence.CareTimeDayEndUnknown, day.Kind)
	assert.False(t, day.Kind.IsOverrun())

	early := date.BerlinMidnight().Add(7 * time.Hour)
	day, _ = classifyCareTimeDay(date, planned, &attendanceSpan{firstCheckIn: early, lastCheckOut: &checkOut}, "", true)
	assert.Equal(t, studentpresence.CareTimeDayOverrun, day.Kind, "an early arrival is an overrun even without a booked end")
}

func (shortRetention) PrefetchCareTimeSettings(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
