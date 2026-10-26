package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SchoolDayReport builds the school-wide day overview (#4802): the numbers of
// the class view for every class the caller may see, from ONE school-wide
// roster and one read of each day fact for all its children, instead of one
// class report per class. The rows keep the class view's rules, so a class
// reads the same numbers here and on its own page. Every call records the
// access in the GDPR log.
func (s *dayReports) SchoolDayReport(ctx context.Context, day classday.Date, actor classday.Actor) (*classday.SchoolDayReport, error) {
	date, err := parseDate(day)
	if err != nil {
		return nil, err
	}
	if s.caller == nil || s.statusDays == nil || s.careDays == nil || s.times == nil {
		return nil, fmt.Errorf("school day report: caller/status/schedule dependencies not configured")
	}
	classes, err := s.caller.GetMySchoolClasses(ctx)
	if err != nil {
		return nil, err
	}
	roster, err := s.rosters.SchoolRosterDay(ctx, date)
	if err != nil {
		return nil, err
	}
	weekday, err := classDayPlanKey(ctx, date)
	if err != nil {
		return nil, err
	}
	enrollmentKnown := len(roster.PhaseNames) > 0
	reports, err := s.schoolDayClassReports(ctx, groupRosterByClass(classes, roster.Rows), date, weekday, enrollmentKnown)
	if err != nil {
		return nil, err
	}
	out := buildSchoolDayReport(date, weekday, enrollmentKnown, reports)
	if err := s.recordSchoolDayViewAudit(ctx, out, actor); err != nil {
		return nil, err
	}
	return out, nil
}

// schoolDayClassReports builds the class reports of the overview: one read
// of the day facts and one of the morning marks for the whole school.
func (s *dayReports) schoolDayClassReports(ctx context.Context, groups []rosterClassGroup, date calendar.Date, weekday string, enrollmentKnown bool) ([]*classday.DayReport, error) {
	facts := newClassDayFacts()
	total := 0
	var studentIDs []int64
	for _, group := range groups {
		total += len(group.rows)
		studentIDs = append(studentIDs, rosterStudentIDs(group.rows)...)
	}
	if weekday != "" {
		var err error
		if facts, err = s.classDayPlanFacts(ctx, studentIDs, date, false); err != nil {
			return nil, err
		}
	}
	// Every class's rows are a window of one backing slice, so the morning
	// marks are read and filled once for the whole school.
	rows := make([]classday.DayRow, 0, total)
	reports := make([]*classday.DayReport, 0, len(groups))
	for _, group := range groups {
		report := buildClassDayReportFor(group.class, date, weekday, "", group.rows, facts)
		applyEnrollmentKnown(report, enrollmentKnown)
		start := len(rows)
		rows = append(rows, report.Rows...)
		report.Rows = rows[start:len(rows):len(rows)]
		reports = append(reports, report)
	}
	if weekday == "" {
		return reports, nil
	}
	if err := s.morning.fillMorningAttendance(ctx, rows, date); err != nil {
		return nil, err
	}
	return reports, nil
}

// rosterClassGroup is the roster rows of one class.
type rosterClassGroup struct {
	class string
	rows  []DayRosterRow
}

// groupRosterByClass sorts the school-wide roster into the caller's classes,
// in the caller's class order, matching class names the way the class
// assignment does. Rows of a class the caller does not see are dropped; a
// class without rows stays in the list with zero children.
func groupRosterByClass(classes []string, rows []DayRosterRow) []rosterClassGroup {
	groups := make([]rosterClassGroup, 0, len(classes))
	index := make(map[string]int, len(classes))
	for _, class := range classes {
		key := schoolstructure.NormalizeClass(class)
		if _, seen := index[key]; seen || strings.TrimSpace(class) == "" {
			continue
		}
		index[key] = len(groups)
		groups = append(groups, rosterClassGroup{class: class})
	}
	for _, row := range rows {
		if at, ok := index[schoolstructure.NormalizeClass(row.SchoolClass)]; ok {
			groups[at].rows = append(groups[at].rows, row)
		}
	}
	return groups
}

// buildSchoolDayReport reduces the class reports to the overview's children
// and numbers and sums them for the school. The class view's totals carry
// over unchanged; only the morning, the single statuses and the changes are
// counted here.
func buildSchoolDayReport(date calendar.Date, weekday string, enrollmentKnown bool, reports []*classday.DayReport) *classday.SchoolDayReport {
	out := &classday.SchoolDayReport{
		Date:            classday.Date(date.String()),
		Weekday:         weekday,
		SchoolDay:       weekday != "",
		EnrollmentKnown: enrollmentKnown,
		Classes:         make([]classday.SchoolDayClass, 0, len(reports)),
	}
	for _, report := range reports {
		class := classday.SchoolDayClass{
			SchoolClass: report.SchoolClass,
			Totals:      classday.SchoolDayTotals{DayTotals: report.Totals},
			Children:    make([]classday.SchoolDayChild, 0, len(report.Rows)),
		}
		for _, row := range report.Rows {
			child := schoolDayChild(row, out.SchoolDay, enrollmentKnown)
			class.Children = append(class.Children, child)
			countSchoolDayChild(&class.Totals, child)
		}
		addSchoolDayTotals(&out.Totals, class.Totals)
		out.Classes = append(out.Classes, class)
	}
	return out
}

// schoolDayChild reduces one class-view row to the overview's child. The
// after-school split follows the class view's totals: a day status wins, a
// class-list-only entry neither stays nor leaves, and without school or a
// covering enrollment phase nobody is split.
func schoolDayChild(row classday.DayRow, schoolDay, enrollmentKnown bool) classday.SchoolDayChild {
	child := classday.SchoolDayChild{
		StudentID: row.StudentID, ListEntry: row.ListEntry, ListEntryID: row.ListEntryID,
		FirstName: row.FirstName, LastName: row.LastName,
	}
	if !schoolDay {
		return child
	}
	child.Status = row.Status
	child.Missing = row.MorningAttendance != nil && row.MorningAttendance.Status == classday.MorningAttendanceAbsent
	child.Changed = !row.ListEntry && (row.Status != "" || row.PickupChanged)
	if row.ListEntry || row.Status != "" || !enrollmentKnown {
		return child
	}
	if row.StaysToday {
		child.AfterSchool = classday.SchoolDayStays
	} else {
		child.AfterSchool = classday.SchoolDayGoesHome
	}
	return child
}

// countSchoolDayChild adds the overview's own counts of one child.
func countSchoolDayChild(totals *classday.SchoolDayTotals, child classday.SchoolDayChild) {
	if child.Missing {
		totals.Missing++
	}
	if child.Changed {
		totals.Changes++
	}
	switch child.Status {
	case statusDaySick:
		totals.Sick++
	case statusDayExcused:
		totals.Excused++
	case statusDayClassTrip:
		totals.ClassTrip++
	}
}

func addSchoolDayTotals(sum *classday.SchoolDayTotals, add classday.SchoolDayTotals) {
	sum.Students += add.Students
	sum.Staying += add.Staying
	sum.Leaving += add.Leaving
	sum.Absent += add.Absent
	sum.ListEntries += add.ListEntries
	sum.Missing += add.Missing
	sum.Sick += add.Sick
	sum.Excused += add.Excused
	sum.ClassTrip += add.ClassTrip
	sum.Changes += add.Changes
}

// recordSchoolDayViewAudit writes the GDPR access log for the school-wide day
// overview: day data of every counted class served to one account. Like the
// class view (recordClassDayViewAudit) it writes one row per actor, report
// date and calendar day of access, so the overview's revalidation does not
// flood the append-only table, and it names the classes it served.
func (s *dayReports) recordSchoolDayViewAudit(ctx context.Context, report *classday.SchoolDayReport, actor classday.Actor) error {
	if s.accessLog == nil {
		return nil
	}
	const errPrefix = "school day view audit"
	date := calendar.Date(report.Date)
	seen, err := s.accessLog.SchoolDayViewSeenSince(ctx, actor.AccountID, date.String(), calendar.TodayDate().BerlinMidnight())
	if err != nil {
		return fmt.Errorf("%s dedupe: %w", errPrefix, err)
	}
	if seen {
		return nil
	}
	entry, err := newAccessRecord(errPrefix, actor.AccountID, actor.Roles, date.BerlinMidnight(), date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	classes := make([]string, 0, len(report.Classes))
	for _, class := range report.Classes {
		classes = append(classes, class.SchoolClass)
	}
	entry.Metadata = map[string]any{
		"report":         "school_day",
		"date":           date.String(),
		"school_classes": classes,
		"student_count":  report.Totals.Students,
	}
	if err := s.accessLog.RecordSchoolDayView(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}
