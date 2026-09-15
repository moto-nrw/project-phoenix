package statistics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

func enrolledEveryDay(timezone.Date, timezone.Date) bool { return true }

type reportStudents struct{}

func (reportStudents) FindOverlappingWithGroups(context.Context, timezone.Date, timezone.Date, timezone.Date) ([]*ReportStudent, error) {
	return nil, nil
}

type reportRetention struct{ err error }

func (r reportRetention) RoomRetentionDays(context.Context) (int, error)   { return 30, r.err }
func (r reportRetention) CourseRetentionDays(context.Context) (int, error) { return 365, r.err }

type reportAudit struct {
	recordErr error
	lookupErr error
	events    []AccessEvent
}

func (a *reportAudit) RecordStatisticsAccess(_ context.Context, event AccessEvent) error {
	a.events = append(a.events, event)
	return a.recordErr
}

func (a *reportAudit) SeenStatisticsAccessSince(context.Context, int64, map[string]string, time.Time) (bool, error) {
	return false, a.lookupErr
}

func TestReportRejectsRetentionAndAuditFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("dependency unavailable")
	filters := Filters{From: timezone.NewDate(2026, 8, 1), To: timezone.NewDate(2026, 8, 2), Sections: []Section{SectionCourses}}
	audit := &reportAudit{}
	service := NewService(Config{Students: reportStudents{}, Retention: reportRetention{err: failure}, AccessLog: audit, Now: fixedNow})
	report, err := service.Report(context.Background(), filters, Actor{AccountID: 1})
	require.ErrorIs(t, err, failure)
	require.Nil(t, report)
	require.Empty(t, audit.events)

	audit.recordErr = failure
	service = NewService(Config{Students: reportStudents{}, Retention: reportRetention{}, AccessLog: audit, Now: fixedNow})
	report, err = service.ReportForExport(context.Background(), filters, Actor{AccountID: 1}, "xlsx")
	require.ErrorIs(t, err, ErrAuditFailed)
	require.Nil(t, report)
	require.Len(t, audit.events, 1)
	require.Equal(t, "xlsx", audit.events[0].Metadata["format"])
	require.Equal(t, "export", audit.events[0].Metadata["action"])
	require.Equal(t, "unknown", audit.events[0].ActorRole)
	require.Equal(t, filters.From.BerlinMidnight(), audit.events[0].RangeStart)
	require.Equal(t, filters.To.EndOfDay(), audit.events[0].RangeEnd)

	audit.events = nil
	audit.recordErr = nil
	audit.lookupErr = failure
	report, err = service.Report(context.Background(), filters, Actor{AccountID: 1})
	require.ErrorIs(t, err, ErrAuditFailed)
	require.Nil(t, report)
	require.Empty(t, audit.events)
}

type dateSet map[timezone.Date]bool

func (d dateSet) HolidayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return map[timezone.Date]bool(d), nil
}

func (d dateSet) ClosingDayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return map[timezone.Date]bool(d), nil
}

type periodList []HolidayPeriod

func (p periodList) StatisticsHolidayPeriods(context.Context, timezone.Date, timezone.Date) ([]HolidayPeriod, error) {
	return p, nil
}

type retentionSettings []RetentionSetting

func (s retentionSettings) ListAcceptedRetentionSettings(context.Context) ([]RetentionSetting, error) {
	return s, nil
}

type accessLogSpy struct {
	metadata []map[string]string
}

func (s *accessLogSpy) RecordStatisticsAccess(context.Context, AccessEvent) error { return nil }

func (s *accessLogSpy) SeenStatisticsAccessSince(_ context.Context, _ int64, metadata map[string]string, _ time.Time) (bool, error) {
	s.metadata = append(s.metadata, metadata)
	return false, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 25, 12, 0, 0, 0, timezone.Berlin)
}

// A day that is a public holiday AND a closing day is subtracted once.
func TestCareDays_UnionOfExclusions(t *testing.T) {
	t.Parallel()
	mon := timezone.NewDate(2026, 6, 8)
	svc := &service{cfg: Config{
		Holidays:    dateSet{mon: true},
		ClosingDays: dateSet{mon: true, timezone.NewDate(2026, 6, 9): true},
		Periods: periodList{{
			StartDate: timezone.NewDate(2026, 6, 13), // Saturday
			EndDate:   timezone.NewDate(2026, 6, 15), // Monday
		}},
		Now: fixedNow,
	}}

	care, excluded, err := svc.careDays(context.Background(), mon, timezone.NewDate(2026, 6, 19))
	require.NoError(t, err)

	// Two weeks = 10 weekdays; minus Mon 08 (holiday+closing), Tue 09
	// (closing), Mon 15 (holiday period) = 7.
	assert.Len(t, care, 7)
	assert.Equal(t, 3, excluded.Total)
	assert.Equal(t, 1, excluded.PublicHolidays)
	assert.Equal(t, 2, excluded.ClosingDays)
	assert.Equal(t, 1, excluded.HolidayPeriods, "weekend days of a holiday period are not counted")
	assert.False(t, care[timezone.NewDate(2026, 6, 13)], "weekend never a care day")
}

// A holiday period reaching far beyond the window is clamped to it.
func TestCareDays_ClampsHolidayPeriodToWindow(t *testing.T) {
	t.Parallel()
	from, to := timezone.NewDate(2026, 6, 8), timezone.NewDate(2026, 6, 12)
	svc := &service{cfg: Config{
		Periods: periodList{{
			StartDate: timezone.NewDate(1900, 1, 1),
			EndDate:   timezone.NewDate(2100, 1, 1),
		}},
		Now: fixedNow,
	}}

	care, excluded, err := svc.careDays(context.Background(), from, to)
	require.NoError(t, err)

	assert.Empty(t, care, "the whole window falls into the holiday period")
	assert.Equal(t, 5, excluded.HolidayPeriods, "only weekdays inside the window are counted")
	assert.Equal(t, 5, excluded.Total)
}

func TestBuildStudentRows_CategoriesAndPrecedence(t *testing.T) {
	t.Parallel()
	d1, d2, d3, d4 := timezone.NewDate(2026, 6, 8), timezone.NewDate(2026, 6, 9), timezone.NewDate(2026, 6, 10), timezone.NewDate(2026, 6, 11)
	care := map[timezone.Date]bool{d1: true, d2: true, d3: true, d4: true}
	groupID := int64(77)
	students := []*ReportStudent{
		{SchoolClass: "1a", GroupID: &groupID, FirstName: "Zoe", LastName: "Beta", GroupName: "Sonne", EnrolledOn: enrolledEveryDay},
		{SchoolClass: "1a", FirstName: "Adam", LastName: "Alpha", EnrolledOn: enrolledEveryDay},
	}
	students[0].ID = 100
	students[1].ID = 101

	rows := buildStudentRows(students, care,
		[]AttendanceDay{
			{StudentID: 100, Date: d1},
			{StudentID: 100, Date: timezone.NewDate(2026, 6, 13)}, // not a care day: ignored
		},
		[]StatusDay{
			{StudentID: 100, Date: d2, Status: activeModels.StudentStatusDayExcused},
			{StudentID: 100, Date: d2, Status: activeModels.StudentStatusDaySick}, // sick wins
			{StudentID: 100, Date: d3, Status: activeModels.StudentStatusDayClassTrip},
			{StudentID: 100, Date: d1, Status: activeModels.StudentStatusDaySick}, // present beats any status
		},
		timezone.DateFromTime(fixedNow()),
	)
	require.Len(t, rows, 2)
	assert.Equal(t, "Alpha", rows[0].LastName, "sorted by last name")

	zoe := rows[1]
	assert.Equal(t, 1, zoe.PresentDays)
	assert.Equal(t, 1, zoe.SickDays)
	assert.Equal(t, 1, zoe.ExcusedDays)
	assert.Equal(t, 1, zoe.UnexplainedDays)
	require.NotNil(t, zoe.AttendanceRate)
	assert.InDelta(t, 25.0, *zoe.AttendanceRate, 0.001)

	adam := rows[0]
	assert.Equal(t, 4, adam.UnexplainedDays)
	assert.Nil(t, adam.GroupID)

	groups := buildGroupRows(rows)
	require.Len(t, groups, 2)
	assert.Equal(t, "Sonne", groups[0].Name)
	assert.Equal(t, NoGroupName, groups[1].Name, "children without a group form the last pseudo group")
	require.NotNil(t, groups[1].AttendanceRate)
	assert.InDelta(t, 0.0, *groups[1].AttendanceRate, 0.001)

	totals := buildTotals(rows)
	assert.Equal(t, 2, totals.StudentCount)
	require.NotNil(t, totals.AttendanceRate)
	assert.InDelta(t, 12.5, *totals.AttendanceRate, 0.001)
}

func TestRate_RoundsToOneDecimalAndNilsOnZero(t *testing.T) {
	t.Parallel()
	assert.Nil(t, rate(3, 0))
	require.NotNil(t, rate(2, 3))
	assert.InDelta(t, 66.7, *rate(2, 3), 0.001)
	assert.InDelta(t, 100.0, *rate(5, 5), 0.001)
}

func TestValidate_RangeRules(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Now: fixedNow}}
	today := timezone.NewDate(2026, 8, 25)

	assert.NoError(t, svc.validate(Filters{From: today.AddDays(-365), To: today}, today))
	assert.ErrorIs(t, svc.validate(Filters{From: today.AddDays(-366), To: today}, today), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{From: today, To: today.AddDays(1)}, today), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{From: today, To: today.AddDays(-1)}, today), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{}, today), ErrInvalidRange)
}

func TestFilterStudentsByGroup(t *testing.T) {
	t.Parallel()
	a, b := int64(21), int64(22)
	students := []*ReportStudent{
		{GroupID: &a, EnrolledOn: enrolledEveryDay},
		{GroupID: &b, EnrolledOn: enrolledEveryDay},
		{EnrolledOn: enrolledEveryDay},
	}
	assert.Len(t, filterStudentsByGroup(students, nil), 3)
	assert.Len(t, filterStudentsByGroup(students, []int64{a}), 1)
	assert.Len(t, filterStudentsByGroup(students, []int64{0}), 1)
	assert.Empty(t, filterStudentsByGroup(students, []int64{99}))

	// A nil row and a row without a hydrated student are skipped, not
	// dereferenced — buildStudentRows guards the same two cases.
	withGaps := []*ReportStudent{nil, {}, {GroupID: &a, EnrolledOn: enrolledEveryDay}}
	assert.Len(t, filterStudentsByGroup(withGaps, []int64{a}), 1)
	assert.Empty(t, filterStudentsByGroup(withGaps, []int64{0}))
}

func TestBuildStudentRows_OnlyCountsDaysInsideEnrollment(t *testing.T) {
	t.Parallel()
	first := timezone.NewDate(2026, 6, 8)
	care := map[timezone.Date]bool{
		first:            true,
		first.AddDays(1): true,
		first.AddDays(2): true,
		first.AddDays(3): true,
	}
	enrolledFrom := first.AddDays(2)
	student := &ReportStudent{EnrolledOn: func(day, _ timezone.Date) bool { return !day.Before(enrolledFrom) }}
	student.ID = 100

	rows := buildStudentRows([]*ReportStudent{student}, care,
		[]AttendanceDay{{StudentID: student.ID, Date: enrolledFrom}}, nil,
		timezone.DateFromTime(fixedNow()))

	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].CareDays)
	assert.Equal(t, 1, rows[0].PresentDays)
	assert.Equal(t, 1, rows[0].UnexplainedDays)
	require.NotNil(t, rows[0].AttendanceRate)
	assert.InDelta(t, 50.0, *rows[0].AttendanceRate, 0.001)
}

func TestRoomRetentionDays_UsesLongestIndividualRetention(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Retention: reportRetention{}, PrivacyConsents: retentionSettings{
		{StudentID: 1, DataRetentionDays: 7},
		{StudentID: 2, DataRetentionDays: 21},
	}}}
	covered := []StudentRow{{StudentID: 1}, {StudentID: 2}}

	days, err := svc.roomRetentionDays(context.Background(), covered)
	require.NoError(t, err)
	assert.Equal(t, 21, days)
}

// The cutoff describes the report that is on the screen: a group filter that
// leaves only short-retention children must not advertise the tenant-wide
// maximum, and a child's own shortest consent is what the room aggregate
// clamps to.
func TestRoomRetentionDays_ScopedToTheCoveredPopulation(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Retention: reportRetention{}, PrivacyConsents: retentionSettings{
		{StudentID: 1, DataRetentionDays: 7},
		{StudentID: 1, DataRetentionDays: 90}, // same child, two consents
		{StudentID: 2, DataRetentionDays: 21},
	}}}

	days, err := svc.roomRetentionDays(context.Background(), []StudentRow{{StudentID: 1}})
	require.NoError(t, err)
	assert.Equal(t, 7, days, "a child is only kept for their shortest accepted consent")

	days, err = svc.roomRetentionDays(context.Background(), []StudentRow{{StudentID: 2}})
	require.NoError(t, err)
	assert.Equal(t, 21, days, "a filtered report must not inherit another group's window")

	// Nobody in the population has consented, so no visit of theirs is kept.
	// The configured default is then the only statement left to make.
	days, err = svc.roomRetentionDays(context.Background(), []StudentRow{{StudentID: 99}})
	require.NoError(t, err)
	assert.Equal(t, 30, days)
}

// An immediately activated child (active, enrolled_from still ahead) is in
// care from today on, so today counts in their denominator while the days
// before it do not. The same child in status pending has no care day at all.
func TestBuildStudentRows_UsesOwnerEnrollmentEligibility(t *testing.T) {
	t.Parallel()
	today := timezone.DateFromTime(fixedNow())
	care := map[timezone.Date]bool{
		today.AddDays(-2): true,
		today.AddDays(-1): true,
		today:             true,
	}
	startsLater := today.AddDays(14)

	activated := &ReportStudent{EnrolledOn: func(day, reportToday timezone.Date) bool { return !day.Before(reportToday) }}
	activated.ID = 100
	pending := &ReportStudent{EnrolledOn: func(day, _ timezone.Date) bool { return !day.Before(startsLater) }}
	pending.ID = 101

	rows := buildStudentRows(
		[]*ReportStudent{activated, pending},
		care,
		[]AttendanceDay{
			{StudentID: activated.ID, Date: today},
			{StudentID: activated.ID, Date: today.AddDays(-1)}, // before care begins
			{StudentID: pending.ID, Date: today},
		},
		nil,
		today,
	)

	require.Len(t, rows, 2)
	byID := map[int64]StudentRow{}
	for _, row := range rows {
		byID[row.StudentID] = row
	}
	assert.Equal(t, 1, byID[activated.ID].CareDays, "only today is inside care")
	assert.Equal(t, 1, byID[activated.ID].PresentDays)
	assert.Equal(t, 0, byID[activated.ID].UnexplainedDays)
	assert.Equal(t, 0, byID[pending.ID].CareDays, "a pending child is not in care yet")
	assert.Nil(t, byID[pending.ID].AttendanceRate)
}

func TestRecordAccess_DeduplicatesOnlyMatchingNormalizedGroupScopes(t *testing.T) {
	t.Parallel()
	accessLog := &accessLogSpy{}
	svc := &service{cfg: Config{AccessLog: accessLog, Now: fixedNow}}
	filters := Filters{From: timezone.NewDate(2026, 8, 1), To: timezone.NewDate(2026, 8, 2)}

	require.NoError(t, svc.recordAccess(context.Background(), Filters{From: filters.From, To: filters.To, GroupIDs: []int64{9, 2}}, Actor{AccountID: 1}, "view", "", true))
	require.NoError(t, svc.recordAccess(context.Background(), Filters{From: filters.From, To: filters.To, GroupIDs: []int64{2, 9}}, Actor{AccountID: 1}, "view", "", true))
	require.NoError(t, svc.recordAccess(context.Background(), Filters{From: filters.From, To: filters.To, GroupIDs: []int64{3}}, Actor{AccountID: 1}, "view", "", true))

	require.Len(t, accessLog.metadata, 3)
	assert.Equal(t, "2,9", accessLog.metadata[0]["group_ids"])
	assert.Equal(t, "2,9", accessLog.metadata[1]["group_ids"])
	assert.Equal(t, "3", accessLog.metadata[2]["group_ids"])
}
