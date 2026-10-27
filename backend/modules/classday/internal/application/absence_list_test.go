package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// fakeAbsenceMarks serves fixed morning marks and remembers the last window.
type fakeAbsenceMarks struct {
	marks    []MorningMark
	from, to calendar.Date
}

func (f *fakeAbsenceMarks) MorningMarksBetween(_ context.Context, from, to calendar.Date, _, _ []int64) ([]MorningMark, error) {
	f.from, f.to = from, to
	return f.marks, nil
}

// fakeAbsenceStatusDays serves fixed status days.
type fakeAbsenceStatusDays struct{ days []DatedStatusDay }

func (f fakeAbsenceStatusDays) StatusDaysBetween(context.Context, []int64, calendar.Date, calendar.Date) ([]DatedStatusDay, error) {
	return f.days, nil
}

// fakeAbsenceCalendar serves a fixed half-year and fixed school-free days.
type fakeAbsenceCalendar struct {
	halfYear *HalfYear
	free     map[calendar.Date]bool
}

func (f fakeAbsenceCalendar) CurrentHalfYear(context.Context, calendar.Date) (HalfYear, bool, error) {
	if f.halfYear == nil {
		return HalfYear{}, false, nil
	}
	return *f.halfYear, true, nil
}

func (f fakeAbsenceCalendar) SchoolFreeDays(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]bool, error) {
	return f.free, nil
}

// fakeAbsenceListLog records the reads and exports like the access log.
type fakeAbsenceListLog struct {
	views   []AccessRecord
	exports []AccessRecord
}

func (f *fakeAbsenceListLog) AbsenceListSeenSince(_ context.Context, actorAccountID int64, schoolClass, from, to string, _ time.Time) (bool, error) {
	for _, entry := range f.views {
		if entry.ActorAccountID == actorAccountID && fmt.Sprint(entry.Metadata["school_class"]) == schoolClass &&
			fmt.Sprint(entry.Metadata["from"]) == from && fmt.Sprint(entry.Metadata["to"]) == to {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAbsenceListLog) RecordAbsenceList(_ context.Context, entry AccessRecord) error {
	f.views = append(f.views, entry)
	return nil
}

func (f *fakeAbsenceListLog) RecordAbsenceListExport(_ context.Context, entry AccessRecord) error {
	f.exports = append(f.exports, entry)
	return nil
}

// fakeAbsenceRenderer keeps the document it was asked to render.
type fakeAbsenceRenderer struct {
	doc      classday.ExportDocument
	format   classday.Format
	filename string
}

func (f *fakeAbsenceRenderer) Render(doc classday.ExportDocument, format classday.Format, filenameBase string) (classday.File, error) {
	f.doc, f.format, f.filename = doc, format, filenameBase
	return classday.File{Data: []byte("file"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

// absenceListToday is Wednesday 16 September 2026 in Berlin.
var absenceListToday = time.Date(2026, time.September, 16, 9, 0, 0, 0, calendar.Berlin)

func absenceListRoster() *DayRoster {
	return &DayRoster{Rows: []DayRosterRow{
		{StudentID: 2, SchoolClass: "3a", FirstName: "Ben", LastName: "Busch"},
		{StudentID: 1, SchoolClass: "3a", FirstName: "Anna", LastName: "Arndt"},
		{SchoolClass: "3a", ListEntry: true, ListEntryID: 9, FirstName: "Lea", LastName: "Lang"},
	}}
}

type absenceListFixture struct {
	marks    *fakeAbsenceMarks
	log      *fakeAbsenceListLog
	renderer *fakeAbsenceRenderer
	list     classday.AbsenceList
}

func newAbsenceListFixture(marks []MorningMark, statuses []DatedStatusDay, cal fakeAbsenceCalendar) absenceListFixture {
	f := absenceListFixture{
		marks: &fakeAbsenceMarks{marks: marks}, log: &fakeAbsenceListLog{}, renderer: &fakeAbsenceRenderer{},
	}
	f.list = NewClassDay(ClassDayDependencies{
		Caller:                        fakeClassDayCaller{classes: []string{"3a"}},
		Rosters:                       fakeDayRosters{roster: absenceListRoster()},
		MorningAttendanceDependencies: MorningAttendanceDependencies{Now: func() time.Time { return absenceListToday }},
		AbsenceListDependencies: AbsenceListDependencies{
			Marks: f.marks, StatusDays: fakeAbsenceStatusDays{days: statuses}, Calendar: cal, Log: f.log, Renderer: f.renderer,
		},
	}).(classday.AbsenceList)
	return f
}

func absenceQuery(from, to classday.Date) classday.AbsenceListQuery {
	return classday.AbsenceListQuery{SchoolClass: "3a", From: from, To: to, Actor: classday.Actor{AccountID: 42, Roles: "lehrkraft"}}
}

// TestAbsenceListCountsTheDecidedRules pins the counting rules of #4793 on
// the school days from Monday 7 to Wednesday 16 September: present in the
// morning wins over sick, an absent morning is excused only by a sick or
// excused status day (the school's own report included), a class trip is no
// absence day, a status day without a mark counts on a school day only, and a
// school day with nothing is "not recorded". Days after today, weekends and
// holidays never count as "not recorded".
func TestAbsenceListCountsTheDecidedRules(t *testing.T) {
	t.Parallel()

	marks := []MorningMark{
		// Anna: present although reported sick, absent with a sick day,
		// absent without a report, absent on the class trip.
		{StudentID: 1, Date: "2026-09-07", Status: classday.MorningAttendancePresent},
		{StudentID: 1, Date: "2026-09-08", Status: classday.MorningAttendanceAbsent},
		{StudentID: 1, Date: "2026-09-09", Status: classday.MorningAttendanceAbsent},
		{StudentID: 1, Date: "2026-09-10", Status: classday.MorningAttendanceAbsent},
		// Ben: absent with the school's afterwards report (excused).
		{StudentID: 2, Date: "2026-09-07", Status: classday.MorningAttendanceAbsent},
		// The class-list entry stays out of the list (#4803).
		{ListEntryID: 9, Date: "2026-09-07", Status: classday.MorningAttendanceAbsent},
	}
	statuses := []DatedStatusDay{
		{StudentID: 1, Date: "2026-09-07", Status: statusDaySick},
		{StudentID: 1, Date: "2026-09-08", Status: statusDaySick},
		{StudentID: 1, Date: "2026-09-10", Status: statusDayClassTrip},
		{StudentID: 2, Date: "2026-09-07", Status: statusDayExcused},
		// Ben: sick without a mark on a school day, sick in the holidays,
		// a class trip without a mark.
		{StudentID: 2, Date: "2026-09-08", Status: statusDaySick},
		{StudentID: 2, Date: "2026-09-11", Status: statusDaySick},
		{StudentID: 2, Date: "2026-09-14", Status: statusDayClassTrip},
	}
	// Friday the 11th is a holiday; the period ends on Friday the 18th,
	// after today.
	f := newAbsenceListFixture(marks, statuses, fakeAbsenceCalendar{free: map[calendar.Date]bool{"2026-09-11": true}})

	report, err := f.list.AbsenceListReport(context.Background(), absenceQuery("2026-09-07", "2026-09-18"))

	require.NoError(t, err)
	assert.Equal(t, classday.Date("2026-09-16"), report.CountedUntil)
	// Mon 7 to Wed 16 without the weekend and the holiday on Friday 11.
	assert.Equal(t, 7, report.SchoolDays)
	assert.Equal(t, calendar.Date("2026-09-16"), f.marks.to, "the marks are read up to today only")
	require.Len(t, report.Children, 2)
	assert.Equal(t, classday.AbsenceListChild{
		StudentID: 1, FirstName: "Anna", LastName: "Arndt",
		AbsenceListCounts: classday.AbsenceListCounts{AbsentDays: 2, ExcusedDays: 1, UnexcusedDays: 1, NotRecordedDays: 3},
	}, report.Children[0])
	assert.Equal(t, classday.AbsenceListChild{
		StudentID: 2, FirstName: "Ben", LastName: "Busch",
		AbsenceListCounts: classday.AbsenceListCounts{AbsentDays: 2, ExcusedDays: 2, NotRecordedDays: 4},
	}, report.Children[1])
	assert.Equal(t, classday.AbsenceListCounts{AbsentDays: 4, ExcusedDays: 3, UnexcusedDays: 1, NotRecordedDays: 7}, report.Totals)
}

// TestAbsenceListPreselectsTheHalfYear takes the period from the school
// calendar's half-year without a requested period, and falls back to the
// school year's half without a calendar entry.
func TestAbsenceListPreselectsTheHalfYear(t *testing.T) {
	t.Parallel()

	half := HalfYear{Name: "1. Halbjahr 2026/27", From: "2026-08-10", To: "2027-01-29"}
	f := newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{halfYear: &half})
	report, err := f.list.AbsenceListReport(context.Background(), absenceQuery("", ""))
	require.NoError(t, err)
	assert.Equal(t, classday.Date("2026-08-10"), report.From)
	assert.Equal(t, classday.Date("2027-01-29"), report.To)
	assert.Equal(t, "1. Halbjahr 2026/27", report.PeriodName)

	f = newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{})
	report, err = f.list.AbsenceListReport(context.Background(), absenceQuery("", ""))
	require.NoError(t, err)
	assert.Equal(t, classday.Date("2026-08-01"), report.From)
	assert.Equal(t, classday.Date("2027-01-31"), report.To)
	assert.Empty(t, report.PeriodName)

	assert.Equal(t, HalfYear{From: "2026-02-01", To: "2026-07-31"}, fallbackHalfYear("2026-05-04"))
	assert.Equal(t, HalfYear{From: "2025-08-01", To: "2026-01-31"}, fallbackHalfYear("2026-01-20"))
}

// TestAbsenceListRefusesABrokenPeriod refuses a period with one end, a
// broken date, an end before the start or more days than allowed.
func TestAbsenceListRefusesABrokenPeriod(t *testing.T) {
	t.Parallel()

	f := newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{})
	for _, period := range [][2]classday.Date{
		{"2026-09-01", ""}, {"", "2026-09-01"}, {"2026-13-01", "2026-12-01"},
		{"2026-09-10", "2026-09-09"}, {"2026-01-01", "2027-02-05"},
	} {
		_, err := f.list.AbsenceListReport(context.Background(), absenceQuery(period[0], period[1]))
		assert.True(t, errors.Is(err, classday.ErrAbsenceListInvalidPeriod), "period %v: %v", period, err)
	}
	assert.Empty(t, f.log.views, "a refused read is not logged")
}

// TestAbsenceListAheadCountsNothing lists the children of a period that lies
// ahead without counting a day.
func TestAbsenceListAheadCountsNothing(t *testing.T) {
	t.Parallel()

	f := newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{})
	report, err := f.list.AbsenceListReport(context.Background(), absenceQuery("2027-02-01", "2027-07-31"))

	require.NoError(t, err)
	assert.Empty(t, report.CountedUntil)
	assert.Zero(t, report.SchoolDays)
	require.Len(t, report.Children, 2)
	assert.Equal(t, classday.AbsenceListCounts{}, report.Totals)
}

// TestAbsenceListLogsReadsAndEveryExport logs a read once per actor, class,
// period and day, and every export with its format.
func TestAbsenceListLogsReadsAndEveryExport(t *testing.T) {
	t.Parallel()

	f := newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{})
	ctx := context.Background()
	query := absenceQuery("2026-09-07", "2026-09-11")

	_, err := f.list.AbsenceListReport(ctx, query)
	require.NoError(t, err)
	_, err = f.list.AbsenceListReport(ctx, query)
	require.NoError(t, err)
	require.Len(t, f.log.views, 1)
	view := f.log.views[0]
	assert.Equal(t, int64(42), view.ActorAccountID)
	assert.Equal(t, "lehrkraft", view.ActorRole)
	assert.Nil(t, view.StudentID)
	assert.Equal(t, map[string]any{"report": "absence_list", "school_class": "3a", "from": "2026-09-07", "to": "2026-09-11", "student_count": 2}, view.Metadata)

	for range 2 {
		file, err := f.list.RenderAbsenceList(ctx, query, classday.FormatXLSX)
		require.NoError(t, err)
		assert.Equal(t, []byte("file"), file.Data)
	}
	require.Len(t, f.log.exports, 2)
	assert.Equal(t, "xlsx", f.log.exports[1].Metadata["format"])
	assert.Equal(t, classday.FormatXLSX, f.renderer.format)
	assert.Equal(t, "fehlzeiten-3a-2026-09-07-bis-2026-09-11", f.renderer.filename)
}

// TestAbsenceListDocumentPrintsEveryChildAndTheClass prints one row per child
// and the class total, with the period and the counting rules on top.
func TestAbsenceListDocumentPrintsEveryChildAndTheClass(t *testing.T) {
	t.Parallel()

	doc := absenceListDocument(&classday.AbsenceListReport{
		SchoolClass: "3a", From: "2026-08-10", To: "2027-01-29", PeriodName: "1. Halbjahr 2026/27", CountedUntil: "2026-09-16", SchoolDays: 27,
		Children: []classday.AbsenceListChild{{
			StudentID: 1, FirstName: "Anna", LastName: "Arndt",
			AbsenceListCounts: classday.AbsenceListCounts{AbsentDays: 3, ExcusedDays: 2, UnexcusedDays: 1, NotRecordedDays: 4},
		}},
		Totals: classday.AbsenceListCounts{AbsentDays: 3, ExcusedDays: 2, UnexcusedDays: 1, NotRecordedDays: 4},
	}, absenceListToday)

	assert.Equal(t, "Fehlzeiten der Klasse 3a", doc.Title)
	assert.Equal(t, "1. Halbjahr 2026/27 · 10.08.2026 bis 29.01.2027", doc.Subtitle)
	assert.Equal(t, "Gezählt bis 16.09.2026 · 27 Schultage", doc.Filters[0])
	require.Len(t, doc.Rows, 2)
	assert.Equal(t, map[classday.ExportColumnID]string{
		classday.ExportColumnName: "Arndt, Anna", classday.ExportColumnAbsentDays: "3", classday.ExportColumnExcusedDays: "2",
		classday.ExportColumnUnexcusedDays: "1", classday.ExportColumnNotRecordedDays: "4",
	}, doc.Rows[0].Values)
	assert.Equal(t, "Klasse gesamt", doc.Rows[1].Values[classday.ExportColumnName])
	assert.Equal(t, "Fehlzeiten der Klasse 3b", absenceListDocument(&classday.AbsenceListReport{SchoolClass: "Klasse 3b"}, absenceListToday).Title,
		"a class stored as \"Klasse 3b\" is not named twice")
}

// TestAbsenceListWithoutPortsIsNotConfigured fails instead of counting
// nothing when the root left a port out.
func TestAbsenceListWithoutPortsIsNotConfigured(t *testing.T) {
	t.Parallel()

	list := NewClassDay(ClassDayDependencies{
		Caller:  fakeClassDayCaller{classes: []string{"3a"}},
		Rosters: fakeDayRosters{roster: absenceListRoster()},
	}).(classday.AbsenceList)

	_, err := list.AbsenceListReport(context.Background(), absenceQuery("", ""))
	assert.ErrorIs(t, err, classday.ErrAbsenceListNotConfigured)
}

// TestAbsenceListCountsNoWeekendAsSchoolDay keeps Saturday and Sunday out of
// the school days even when the OGS cares on the weekend by Friday's plan:
// the setting is about the OGS, not about school mornings.
func TestAbsenceListCountsNoWeekendAsSchoolDay(t *testing.T) {
	t.Parallel()

	f := newAbsenceListFixture(nil, nil, fakeAbsenceCalendar{})
	ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return true, nil })

	report, err := f.list.AbsenceListReport(ctx, absenceQuery("2026-09-07", "2026-09-16"))

	require.NoError(t, err)
	assert.Equal(t, 8, report.SchoolDays)
	assert.Equal(t, 8, report.Children[0].NotRecordedDays)
}
