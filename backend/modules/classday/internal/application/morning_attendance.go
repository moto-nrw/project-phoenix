package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Ports of the morning attendance (#4790). Student Presence stores the
// marks; the school's setting decides whether moto schule may write them.
type (
	// MorningAttendance reads and writes Student Presence's morning marks.
	MorningAttendance interface {
		MorningAttendanceOn(ctx context.Context, date calendar.Date, children []studentpresence.MorningAttendanceChild) ([]studentpresence.MorningAttendanceEntry, error)
		RecordMorningAttendance(ctx context.Context, in studentpresence.MorningAttendanceWrite) error
	}
	// MorningAttendanceScope applies operations.school_portal_morning_attendance:
	// true while the school leaves the morning attendance on.
	MorningAttendanceScope func(ctx context.Context) (bool, error)
)

// MorningAttendanceDependencies wire the morning attendance (#4790). Without
// Store or Scope the report fills in nothing beyond reported absences, the
// write verdict stays false and the write answers
// classday.ErrMorningAttendanceNotConfigured. Now is the clock "today" is
// taken from; nil uses the wall clock.
type MorningAttendanceDependencies struct {
	Store MorningAttendance
	Scope MorningAttendanceScope
	Now   func() time.Time
	// The reports the Schulverwaltung enters for a child the Lehrkraft
	// marked absent without one (#4791), and the notice a morning mark
	// sends: without ReportStatusDays or ReportLog the reports answer
	// classday.ErrAbsenceReportsNotConfigured; without UnreportedNotifier no
	// notice goes out.
	ReportStatusDays   AbsenceReportStatusDays
	ReportLog          AbsenceReportLog
	UnreportedNotifier UnreportedAbsenceNotifier
}

// morningAttendance is the second write seam of the class-day view (#4790):
// the Lehrkraft marks each child of an assigned class present or absent for
// today's school morning. The marks live in Student Presence apart from the
// OGS attendance; a reported absence (sick, excused, class trip) fills in
// "absent" until the Lehrkraft marks the child.
type morningAttendance struct {
	store   MorningAttendance
	scope   MorningAttendanceScope
	rosters DayRosters
	today   func() calendar.Date
	// after runs once the marks are stored (#4791); nil runs nothing.
	after func(ctx context.Context, in classday.MorningAttendanceWrite, date calendar.Date) error
}

func newMorningAttendance(deps ClassDayDependencies) morningAttendance {
	today := calendar.TodayDate
	if now := deps.Now; now != nil {
		today = func() calendar.Date { return calendar.DateFromTime(now()) }
	}
	return morningAttendance{store: deps.Store, scope: deps.Scope, rosters: deps.Rosters, today: today}
}

// mayRecord applies the setting. Without the store or the setting the
// verdict is false.
func (m morningAttendance) mayRecord(ctx context.Context) (bool, error) {
	if m.store == nil || m.scope == nil {
		return false, nil
	}
	may, err := m.scope(ctx)
	if err != nil {
		return false, fmt.Errorf("class day morning attendance: resolve setting: %w", err)
	}
	return may, nil
}

// RecordMorningAttendance stores the marks of one class for today after the
// setting, date, school-day and roster checks.
func (m morningAttendance) RecordMorningAttendance(ctx context.Context, in classday.MorningAttendanceWrite) error {
	if m.store == nil || m.scope == nil || m.rosters == nil {
		return classday.ErrMorningAttendanceNotConfigured
	}
	if len(in.Marks) == 0 || in.RecordedBy <= 0 {
		return classday.ErrMorningAttendanceInvalid
	}
	may, err := m.mayRecord(ctx)
	if err != nil {
		return err
	}
	if !may {
		return classday.ErrMorningAttendanceDisabled
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return fmt.Errorf("%w: %w", classday.ErrMorningAttendanceInvalid, err)
	}
	if date != m.today() {
		return classday.ErrMorningAttendanceNotToday
	}
	weekday, err := classDayPlanKey(ctx, date)
	if err != nil {
		return err
	}
	if weekday == "" {
		return classday.ErrMorningAttendanceNoSchoolDay
	}
	roster, err := m.rosters.ClassRosterDay(ctx, in.SchoolClass, date)
	if err != nil {
		return err
	}
	marks, err := rosterMarks(roster, in.Marks)
	if err != nil {
		return err
	}
	err = m.store.RecordMorningAttendance(ctx, studentpresence.MorningAttendanceWrite{Date: date, RecordedBy: in.RecordedBy, Marks: marks})
	if errors.Is(err, studentpresence.ErrMorningAttendanceInvalid) {
		return fmt.Errorf("%w: %w", classday.ErrMorningAttendanceInvalid, err)
	}
	if err != nil || m.after == nil {
		return err
	}
	return m.after(ctx, in, date)
}

// rosterMarks maps the marks onto Student Presence's children and refuses
// one for a child that is not on the roster of the day.
func rosterMarks(roster *DayRoster, in []classday.MorningAttendanceMark) ([]studentpresence.MorningAttendanceMark, error) {
	onRoster := rosterChildren(roster)
	out := make([]studentpresence.MorningAttendanceMark, 0, len(in))
	for _, mark := range in {
		child, err := markChild(mark)
		if err != nil {
			return nil, err
		}
		if !onRoster[child] {
			return nil, classday.ErrMorningAttendanceUnknownChild
		}
		out = append(out, studentpresence.MorningAttendanceMark{Child: child, Status: mark.Status})
	}
	return out, nil
}

// rosterChildren is the set of children on the roster of the day.
func rosterChildren(roster *DayRoster) map[studentpresence.MorningAttendanceChild]bool {
	children := map[studentpresence.MorningAttendanceChild]bool{}
	if roster == nil {
		return children
	}
	for _, row := range roster.Rows {
		switch {
		case row.ListEntry:
			children[studentpresence.MorningAttendanceChild{ClassListEntryID: row.ListEntryID}] = true
		case row.StudentID > 0:
			children[studentpresence.MorningAttendanceChild{StudentID: row.StudentID}] = true
		}
	}
	return children
}

// markChild validates one mark: a known status and exactly one child ID.
func markChild(mark classday.MorningAttendanceMark) (studentpresence.MorningAttendanceChild, error) {
	if mark.Status != classday.MorningAttendancePresent && mark.Status != classday.MorningAttendanceAbsent {
		return studentpresence.MorningAttendanceChild{}, classday.ErrMorningAttendanceInvalid
	}
	if (mark.StudentID > 0) == (mark.ListEntryID > 0) || mark.StudentID < 0 || mark.ListEntryID < 0 {
		return studentpresence.MorningAttendanceChild{}, classday.ErrMorningAttendanceInvalid
	}
	return studentpresence.MorningAttendanceChild{StudentID: mark.StudentID, ClassListEntryID: mark.ListEntryID}, nil
}

// applyMorningAttendance fills in the rows' morning status and the report's
// write verdict on a school day. A stored mark wins; a child without one and
// with a reported sick, excused or class-trip day reads "absent" from the
// report. The derived "cancelled" (OGS care called off) says nothing about
// the morning and fills in nothing.
func (m morningAttendance) applyMorningAttendance(ctx context.Context, report *classday.DayReport, date calendar.Date) error {
	if !report.SchoolDay {
		return nil
	}
	if err := m.fillMorningAttendance(ctx, report.Rows, date); err != nil {
		return err
	}
	if date != m.today() {
		return nil
	}
	// The verdict of the day; the HTTP adapter adds the permission (#4791).
	report.CanReportAbsences = true
	may, err := m.mayRecord(ctx)
	if err != nil {
		return err
	}
	report.CanRecordMorningAttendance = may
	return nil
}

// fillMorningAttendance sets the morning status of the rows from one read
// of the stored marks, falling back to a reported absence.
func (m morningAttendance) fillMorningAttendance(ctx context.Context, rows []classday.DayRow, date calendar.Date) error {
	stored := map[studentpresence.MorningAttendanceChild]string{}
	if m.store != nil {
		children := make([]studentpresence.MorningAttendanceChild, 0, len(rows))
		for _, row := range rows {
			children = append(children, morningChild(row))
		}
		entries, err := m.store.MorningAttendanceOn(ctx, date, children)
		if err != nil {
			return fmt.Errorf("class day report: load morning attendance: %w", err)
		}
		for _, entry := range entries {
			stored[entry.Child] = entry.Status
		}
	}
	for i := range rows {
		row := &rows[i]
		if status, ok := stored[morningChild(*row)]; ok {
			row.MorningAttendance = &classday.DayMorningAttendance{Status: status, Source: classday.MorningAttendanceSourceTeacher}
			row.Unreported = unreportedAbsence(*row)
			continue
		}
		if reportedAbsence(row.Status) {
			row.MorningAttendance = &classday.DayMorningAttendance{Status: classday.MorningAttendanceAbsent, Source: classday.MorningAttendanceSourceReport}
		}
	}
	return nil
}

func morningChild(row classday.DayRow) studentpresence.MorningAttendanceChild {
	if row.ListEntry {
		return studentpresence.MorningAttendanceChild{ClassListEntryID: row.ListEntryID}
	}
	return studentpresence.MorningAttendanceChild{StudentID: row.StudentID}
}

// unreportedAbsence reports a child the Lehrkraft marked absent that nobody
// reported sick, excused or on a class trip ("fehlt ohne Meldung", #4791).
// The derived "cancelled" (OGS care called off) explains no school morning.
// A class-list-only entry has no status day to report through yet; it joins
// with #4803.
func unreportedAbsence(row classday.DayRow) bool {
	return !row.ListEntry && row.StudentID > 0 &&
		row.MorningAttendance != nil &&
		row.MorningAttendance.Source == classday.MorningAttendanceSourceTeacher &&
		row.MorningAttendance.Status == classday.MorningAttendanceAbsent &&
		!reportedAbsence(row.Status)
}

// reportedAbsence reports a status day that excuses the morning.
func reportedAbsence(status string) bool {
	switch status {
	case statusDaySick, statusDayExcused, statusDayClassTrip:
		return true
	default:
		return false
	}
}
