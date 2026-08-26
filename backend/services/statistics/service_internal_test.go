package statistics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type dateSet map[timezone.Date]bool

func (d dateSet) HolidayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return map[timezone.Date]bool(d), nil
}

func (d dateSet) ClosingDayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return map[timezone.Date]bool(d), nil
}

type periodList []*scheduleModels.CalendarPeriod

func (p periodList) FindActiveOverlappingByType(context.Context, string, timezone.Date, timezone.Date, int64) ([]*scheduleModels.CalendarPeriod, error) {
	return p, nil
}

type retentionSettings []userModels.StudentRetentionSetting

func (s retentionSettings) ListAcceptedRetentionSettings(context.Context) ([]userModels.StudentRetentionSetting, error) {
	return s, nil
}

type accessLogSpy struct {
	metadata []map[string]string
}

func (s *accessLogSpy) Create(context.Context, *auditModels.DataAccessLog) error { return nil }

func (s *accessLogSpy) ExistsSince(_ context.Context, _ int64, _ string, metadata map[string]string, _ time.Time) (bool, error) {
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
	students := []*userModels.StudentWithGroupInfo{
		{Student: &userModels.Student{PersonID: 10, SchoolClass: "1a", GroupID: &groupID, Person: &userModels.Person{FirstName: "Zoe", LastName: "Beta"}}, GroupName: "Sonne"},
		{Student: &userModels.Student{PersonID: 11, SchoolClass: "1a", Person: &userModels.Person{FirstName: "Adam", LastName: "Alpha"}}},
	}
	students[0].ID = 100
	students[1].ID = 101

	rows := buildStudentRows(students, care,
		[]activeModels.AttendanceDayRow{
			{StudentID: 100, Date: d1},
			{StudentID: 100, Date: timezone.NewDate(2026, 6, 13)}, // not a care day: ignored
		},
		[]activeModels.StatusDayRow{
			{StudentID: 100, Date: d2, Status: activeModels.StudentStatusDayExcused},
			{StudentID: 100, Date: d2, Status: activeModels.StudentStatusDaySick}, // sick wins
			{StudentID: 100, Date: d3, Status: activeModels.StudentStatusDayClassTrip},
			{StudentID: 100, Date: d1, Status: activeModels.StudentStatusDaySick}, // present beats any status
		},
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

	assert.NoError(t, svc.validate(Filters{From: today.AddDays(-365), To: today}))
	assert.ErrorIs(t, svc.validate(Filters{From: today.AddDays(-366), To: today}), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{From: today, To: today.AddDays(1)}), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{From: today, To: today.AddDays(-1)}), ErrInvalidRange)
	assert.ErrorIs(t, svc.validate(Filters{}), ErrInvalidRange)
}

func TestFilterStudentsByGroup(t *testing.T) {
	t.Parallel()
	a, b := int64(21), int64(22)
	students := []*userModels.StudentWithGroupInfo{
		{Student: &userModels.Student{GroupID: &a}},
		{Student: &userModels.Student{GroupID: &b}},
		{Student: &userModels.Student{}},
	}
	assert.Len(t, filterStudentsByGroup(students, nil), 3)
	assert.Len(t, filterStudentsByGroup(students, []int64{a}), 1)
	assert.Len(t, filterStudentsByGroup(students, []int64{0}), 1)
	assert.Empty(t, filterStudentsByGroup(students, []int64{99}))
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
	student := &userModels.Student{EnrolledFrom: &enrolledFrom}
	student.ID = 100

	rows := buildStudentRows([]*userModels.StudentWithGroupInfo{{Student: student}}, care,
		[]activeModels.AttendanceDayRow{{StudentID: student.ID, Date: enrolledFrom}}, nil)

	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].CareDays)
	assert.Equal(t, 1, rows[0].PresentDays)
	assert.Equal(t, 1, rows[0].UnexplainedDays)
	require.NotNil(t, rows[0].AttendanceRate)
	assert.InDelta(t, 50.0, *rows[0].AttendanceRate, 0.001)
}

func TestRoomRetentionDays_UsesLongestIndividualRetention(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{PrivacyConsents: retentionSettings{
		{StudentID: 1, DataRetentionDays: 7},
		{StudentID: 2, DataRetentionDays: 21},
	}}}

	days, err := svc.roomRetentionDays(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 21, days)
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
