package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// fakeClassDayCaller serves a fixed class assignment.
type fakeClassDayCaller struct {
	classes []string
}

func (f fakeClassDayCaller) AssignedClasses(context.Context) ([]string, error) {
	return f.classes, nil
}

func (fakeClassDayCaller) StaffID(context.Context) (int64, error) { return 0, nil }

// fakeDayRosters serves a fixed Enrollment day roster.
type fakeDayRosters struct {
	roster *DayRoster
}

func (f fakeDayRosters) ClassRosterDay(context.Context, string, timezone.Date) (*DayRoster, error) {
	if f.roster == nil {
		return &DayRoster{}, nil
	}
	return f.roster, nil
}

// fakeClassDayStatusDays serves fixed status-day rows.
type fakeClassDayStatusDays struct {
	entries []StatusDayEntry
}

func (f fakeClassDayStatusDays) ActiveStatusDays(context.Context, []int64, timezone.Date) ([]StatusDayEntry, error) {
	return f.entries, nil
}

// fakeCareDays serves fixed care-day verdicts.
type fakeCareDays struct {
	statuses map[int64]ports.CareDay
}

func (f fakeCareDays) ResolveForDate(context.Context, []int64, timezone.Date) (map[int64]ports.CareDay, error) {
	return f.statuses, nil
}

// fakeDayTimes serves fixed effective times for the bulk read the class day
// view uses.
type fakeDayTimes struct {
	pickups  map[int64]EffectivePickup
	arrivals map[int64]EffectiveArrival
}

func (f fakeDayTimes) EffectivePickups(context.Context, []int64, timezone.Date) (map[int64]EffectivePickup, error) {
	return f.pickups, nil
}

func (f fakeDayTimes) EffectiveArrivals(context.Context, []int64, timezone.Date) (map[int64]EffectiveArrival, error) {
	return f.arrivals, nil
}

// failingCompanions fails every companion lookup.
type failingCompanions struct{}

func (failingCompanions) ListLinksForStudents(context.Context, []int64) (map[int64][]peopledirectory.CompanionLink, error) {
	return nil, errors.New("companion lookup failed")
}

// fakeClassDayAccessLog captures access rows per record method and answers
// ClassDayViewSeenSince from the captured class day views, mirroring the real
// dedupe semantics (actor + school_class + date metadata).
type fakeClassDayAccessLog struct {
	views  []AccessRecord
	sheets []AccessRecord
}

func (f *fakeClassDayAccessLog) ClassDayViewSeenSince(_ context.Context, actorAccountID int64, schoolClass, date string, _ time.Time) (bool, error) {
	for _, entry := range f.views {
		if entry.ActorAccountID != actorAccountID {
			continue
		}
		if fmt.Sprint(entry.Metadata["school_class"]) == schoolClass && fmt.Sprint(entry.Metadata["date"]) == date {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeClassDayAccessLog) RecordClassDayView(_ context.Context, entry AccessRecord) error {
	f.views = append(f.views, entry)
	return nil
}

func (f *fakeClassDayAccessLog) RecordSupervisionSheet(_ context.Context, entry AccessRecord) error {
	f.sheets = append(f.sheets, entry)
	return nil
}

// fakeClassArrivalExceptions serves fixed class arrival exception rows and
// records writes.
type fakeClassArrivalExceptions struct {
	rows      []ClassArrivalException
	listErr   error
	writeErr  error
	upserts   []ClassArrivalExceptionWrite
	deletes   []string
	listRange [2]timezone.Date
}

func (f *fakeClassArrivalExceptions) ListClassArrivalExceptions(_ context.Context, _ string, from, to timezone.Date) ([]ClassArrivalException, error) {
	f.listRange = [2]timezone.Date{from, to}
	return f.rows, f.listErr
}

func (f *fakeClassArrivalExceptions) UpsertClassArrivalException(_ context.Context, in ClassArrivalExceptionWrite) (ClassArrivalException, error) {
	if f.writeErr != nil {
		return ClassArrivalException{}, f.writeErr
	}
	f.upserts = append(f.upserts, in)
	return ClassArrivalException{
		SchoolClass: in.SchoolClass, Date: in.Date, ArrivalTime: in.ArrivalTime,
		Reason: in.Reason, CreatedAt: time.Date(2026, 8, 3, 7, 0, 0, 0, time.UTC), Origin: in.Origin,
	}, nil
}

func (f *fakeClassArrivalExceptions) DeleteClassArrivalException(_ context.Context, schoolClass string, date timezone.Date) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.deletes = append(f.deletes, schoolClass+"@"+date.String())
	return nil
}

func TestClassDayWeekdayKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mon", classDayWeekdayKey(timezone.NewDate(2026, 8, 3)))
	assert.Equal(t, "tue", classDayWeekdayKey(timezone.NewDate(2026, 8, 4)))
	assert.Equal(t, "wed", classDayWeekdayKey(timezone.NewDate(2026, 8, 5)))
	assert.Equal(t, "thu", classDayWeekdayKey(timezone.NewDate(2026, 8, 6)))
	assert.Equal(t, "fri", classDayWeekdayKey(timezone.NewDate(2026, 8, 7)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 8)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 9)))
}

func TestClassDayRequiresConfiguredDependencies(t *testing.T) {
	t.Parallel()

	svc := NewClassDay(ClassDayDependencies{Caller: fakeClassDayCaller{}, Rosters: fakeDayRosters{}})

	_, err := svc.DayReport(context.Background(), "1a", "2026-08-05", classday.Actor{AccountID: 42, Roles: "lehrkraft"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestClassDayWithoutPhaseListsFullClass(t *testing.T) {
	t.Parallel()

	// Enrollment serves the roster without any covering phase: every child of
	// the class, none registered.
	roster := &DayRoster{
		Rows: []DayRosterRow{
			{StudentID: 1, FirstName: "Mila", LastName: "Anders"},
			{StudentID: 2, FirstName: "Finn", LastName: "Becker"},
		},
		Students: []DayRosterStudent{{ID: 1}, {ID: 2}},
	}
	svc := NewClassDay(ClassDayDependencies{
		Caller:  fakeClassDayCaller{},
		Rosters: fakeDayRosters{roster: roster},
		StatusDays: fakeClassDayStatusDays{entries: []StatusDayEntry{
			{StudentID: 2, Status: statusDayExcused},
		}},
		Times:    fakeDayTimes{},
		CareDays: fakeCareDays{},
	})

	report, err := svc.DayReport(context.Background(), "1a", "2026-08-05", classday.Actor{AccountID: 42, Roles: "lehrkraft"})

	require.NoError(t, err)
	require.Len(t, report.Rows, 2)
	assert.Equal(t, "", report.PhaseName)
	assert.False(t, report.Rows[0].Registered)
	assert.Equal(t, "Anders", report.Rows[0].LastName)
	assert.Equal(t, statusDayExcused, report.Rows[1].Status)
	// Without a covering phase the stays/leaves split is unknowable: the
	// flag says so and the counters stay zero instead of claiming everyone
	// goes home.
	assert.False(t, report.EnrollmentKnown)
	assert.Equal(t, classday.DayTotals{Students: 2, Absent: 1}, report.Totals)
}

func TestRecordClassDayViewAuditDedupesPerActorClassAndDate(t *testing.T) {
	t.Parallel()

	log := &fakeClassDayAccessLog{}
	svc := newDayReports(ClassDayDependencies{AccessLog: log})
	report := &classday.DayReport{SchoolClass: "1a", Date: "2026-08-05"}

	// The view revalidates itself (interval + focus): identical repeat reads
	// must collapse into ONE evidential row.
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 42, "lehrkraft"))
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 42, "lehrkraft"))
	assert.Len(t, log.views, 1)

	// A different class, date, or actor is a distinct access and writes again.
	otherClass := &classday.DayReport{SchoolClass: "1b", Date: "2026-08-05"}
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), otherClass, 42, "lehrkraft"))
	otherDate := &classday.DayReport{SchoolClass: "1a", Date: "2026-08-06"}
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), otherDate, 42, "lehrkraft"))
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 43, "lehrkraft"))
	assert.Len(t, log.views, 4)
	// Every row went to the class day view resource, none to the sheet.
	assert.Empty(t, log.sheets)
	assert.Equal(t, "class_day", log.views[0].Metadata["report"])
	assert.Equal(t, "lehrkraft", log.views[0].ActorRole)
}

func TestClassDayRequiresSchoolClass(t *testing.T) {
	t.Parallel()

	svc := NewClassDay(ClassDayDependencies{Caller: fakeClassDayCaller{}, Rosters: fakeDayRosters{}})

	_, err := svc.DayReport(context.Background(), "  ", "2026-08-05", classday.Actor{AccountID: 42, Roles: "lehrkraft"})

	require.ErrorIs(t, err, classday.ErrInvalidReportFilter)
	assert.EqualError(t, err, "class day report: school class required: enrollment report filter is invalid")
}

// TestClassDayArrivalExceptionLine pins the class-wide arrival exception line
// of the day report (#2962): the row of the date wins, the origin defaults to
// the OGS for rows older than the column, and a deployment without the store
// still serves the sheet.
func TestClassDayArrivalExceptionLine(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 8, 5)
	reason := "  Lehrerkonferenz  "

	t.Run("row of the date", func(t *testing.T) {
		t.Parallel()
		store := &fakeClassArrivalExceptions{rows: []ClassArrivalException{
			{SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 4), ArrivalTime: time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)},
			{SchoolClass: "1a", Date: date, ArrivalTime: time.Date(2000, 1, 1, 9, 45, 0, 0, time.UTC), Reason: &reason},
		}}
		svc := newDayReports(ClassDayDependencies{ClassArrivalExceptions: store})

		line, err := svc.classDayArrivalException(context.Background(), "1a", date)

		require.NoError(t, err)
		assert.Equal(t, [2]timezone.Date{date, date}, store.listRange)
		assert.Equal(t, &classday.DayArrivalException{ArrivalTime: "09:45", Reason: "Lehrerkonferenz", Origin: classday.ArrivalExceptionOriginOGS}, line)
	})

	t.Run("no store", func(t *testing.T) {
		t.Parallel()
		line, err := newDayReports(ClassDayDependencies{}).classDayArrivalException(context.Background(), "1a", date)
		require.NoError(t, err)
		assert.Nil(t, line)
	})

	t.Run("store not configured", func(t *testing.T) {
		t.Parallel()
		store := &fakeClassArrivalExceptions{listErr: fmt.Errorf("wrap: %w", ports.ErrClassArrivalExceptionsNotConfigured)}
		line, err := newDayReports(ClassDayDependencies{ClassArrivalExceptions: store}).classDayArrivalException(context.Background(), "1a", date)
		require.NoError(t, err)
		assert.Nil(t, line)
	})

	t.Run("read failure fails the report", func(t *testing.T) {
		t.Parallel()
		store := &fakeClassArrivalExceptions{listErr: errors.New("boom")}
		_, err := newDayReports(ClassDayDependencies{ClassArrivalExceptions: store}).classDayArrivalException(context.Background(), "1a", date)
		assert.EqualError(t, err, "class day report: load class arrival exception: boom")
	})
}
