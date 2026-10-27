package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Ports of the absence reports (#4791). The OGS's status-day write stores a
// report, the Audit Platform keeps the child's change history, and Delivery
// tells the Schulverwaltung who misses without a report.
type (
	// AbsenceReportStatusDays enters and withdraws the school's sick or
	// excused day through the OGS's status-day write, which re-checks under
	// lock that the child is still in the class. Refusals answer errors.Is
	// for classday.ErrAbsenceReportConflict, ErrAbsenceReportUnknownChild
	// and ErrAbsenceReportNotFound; nothing is written then.
	AbsenceReportStatusDays interface {
		RecordSchoolAbsenceReport(ctx context.Context, schoolClass string, studentID int64, status string, date calendar.Date, actorAccountID int64) error
		// WithdrawSchoolAbsenceReport clears the school's status day of the
		// child on date and returns its status.
		WithdrawSchoolAbsenceReport(ctx context.Context, schoolClass string, studentID int64, date calendar.Date) (string, error)
	}
	// AbsenceReportLog appends one entry to the child's change history.
	AbsenceReportLog interface {
		RecordAbsenceReportChange(ctx context.Context, entry classday.AbsenceReportLogEntry) error
	}
	// UnreportedAbsenceNotifier tells the Schulverwaltung, inside the
	// caller's transaction, which children of the school miss without a
	// report today. The notice names only their number.
	UnreportedAbsenceNotifier interface {
		NotifyUnreportedAbsences(ctx context.Context, notice classday.UnreportedAbsenceNotice) error
	}
)

// absenceReports is the write seam of the Schulverwaltung (#4791): it enters
// sick or excused for a child the Lehrkraft marked absent without a report,
// and takes such a report back. It writes no rows of its own: the status day
// is the OGS's, the change history the Audit Platform's.
type absenceReports struct {
	statusDays AbsenceReportStatusDays
	log        AbsenceReportLog
	notifier   UnreportedAbsenceNotifier
	reads      StatusDays
	morning    MorningAttendance
	rosters    DayRosters
	today      func() calendar.Date
}

func newAbsenceReports(deps ClassDayDependencies) absenceReports {
	today := calendar.TodayDate
	if now := deps.Now; now != nil {
		today = func() calendar.Date { return calendar.DateFromTime(now()) }
	}
	return absenceReports{
		statusDays: deps.ReportStatusDays, log: deps.ReportLog, notifier: deps.UnreportedNotifier,
		reads: deps.StatusDays, morning: deps.Store, rosters: deps.Rosters, today: today,
	}
}

// ReportAbsence enters sick or excused for today after the day, roster and
// "fehlt ohne Meldung" checks, and records it in the child's change history.
func (a absenceReports) ReportAbsence(ctx context.Context, in classday.AbsenceReportWrite) error {
	if a.statusDays == nil || a.log == nil || a.reads == nil || a.morning == nil || a.rosters == nil {
		return classday.ErrAbsenceReportsNotConfigured
	}
	if in.Status != classday.AbsenceReportSick && in.Status != classday.AbsenceReportExcused {
		return classday.ErrAbsenceReportInvalid
	}
	if in.SchoolClass == "" || in.StudentID <= 0 || in.Actor.AccountID <= 0 {
		return classday.ErrAbsenceReportInvalid
	}
	date, err := a.checkDay(ctx, in.Date)
	if err != nil {
		return err
	}
	if err := a.requireUnreported(ctx, in.SchoolClass, in.StudentID, date); err != nil {
		return err
	}
	if err := a.statusDays.RecordSchoolAbsenceReport(ctx, in.SchoolClass, in.StudentID, in.Status, date, in.Actor.AccountID); err != nil {
		return err
	}
	return a.record(ctx, in.StudentID, in.Actor, unreportedLabel(date), absenceReportLabel(in.Status, date))
}

// WithdrawAbsenceReport takes back the report the school entered today and
// records it in the child's change history.
func (a absenceReports) WithdrawAbsenceReport(ctx context.Context, in classday.AbsenceReportWithdrawal) error {
	if a.statusDays == nil || a.log == nil {
		return classday.ErrAbsenceReportsNotConfigured
	}
	if in.SchoolClass == "" || in.StudentID <= 0 || in.Actor.AccountID <= 0 {
		return classday.ErrAbsenceReportInvalid
	}
	date, err := parseDate(in.Date)
	if err != nil {
		return fmt.Errorf("%w: %w", classday.ErrAbsenceReportInvalid, err)
	}
	if date != a.today() {
		return classday.ErrAbsenceReportNotToday
	}
	status, err := a.statusDays.WithdrawSchoolAbsenceReport(ctx, in.SchoolClass, in.StudentID, date)
	if err != nil {
		return err
	}
	return a.record(ctx, in.StudentID, in.Actor, absenceReportLabel(status, date), unreportedLabel(date))
}

func (a absenceReports) record(ctx context.Context, studentID int64, actor classday.AbsenceReportActor, oldValue, newValue string) error {
	if err := a.log.RecordAbsenceReportChange(ctx, classday.AbsenceReportLogEntry{
		StudentID: studentID, Actor: actor, OldValue: oldValue, NewValue: newValue,
	}); err != nil {
		return fmt.Errorf("class day absence report: record change history: %w", err)
	}
	return nil
}

// checkDay refuses any day but today and a day without school.
func (a absenceReports) checkDay(ctx context.Context, day classday.Date) (calendar.Date, error) {
	date, err := parseDate(day)
	if err != nil {
		return "", fmt.Errorf("%w: %w", classday.ErrAbsenceReportInvalid, err)
	}
	if date != a.today() {
		return "", classday.ErrAbsenceReportNotToday
	}
	weekday, err := classDayPlanKey(ctx, date)
	if err != nil {
		return "", err
	}
	if weekday == "" {
		return "", classday.ErrAbsenceReportNoSchoolDay
	}
	return date, nil
}

// requireUnreported refuses a child that is not an OGS child of the class on
// the day, or that does not miss without a report.
func (a absenceReports) requireUnreported(ctx context.Context, schoolClass string, studentID int64, date calendar.Date) error {
	roster, err := a.rosters.ClassRosterDay(ctx, schoolClass, date)
	if err != nil {
		return err
	}
	if !rosterChildren(roster)[studentpresence.MorningAttendanceChild{StudentID: studentID}] {
		return classday.ErrAbsenceReportUnknownChild
	}
	unreported, err := a.unreportedAmong(ctx, []int64{studentID}, date)
	if err != nil {
		return err
	}
	if len(unreported) == 0 {
		return classday.ErrAbsenceReportNotMissing
	}
	return nil
}

// unreportedAmong returns the children of studentIDs that miss without a
// report on date, in ascending order: one read of the morning marks and one
// of the status days of the absent ones.
func (a absenceReports) unreportedAmong(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error) {
	children := make([]studentpresence.MorningAttendanceChild, 0, len(studentIDs))
	for _, id := range studentIDs {
		children = append(children, studentpresence.MorningAttendanceChild{StudentID: id})
	}
	if len(children) == 0 {
		return nil, nil
	}
	marks, err := a.morning.MorningAttendanceOn(ctx, date, children)
	if err != nil {
		return nil, fmt.Errorf("class day absence report: load morning attendance: %w", err)
	}
	absent := make([]int64, 0, len(marks))
	for _, mark := range marks {
		if mark.Child.StudentID > 0 && mark.Status == classday.MorningAttendanceAbsent {
			absent = append(absent, mark.Child.StudentID)
		}
	}
	if len(absent) == 0 {
		return nil, nil
	}
	entries, err := a.reads.ActiveStatusDays(ctx, absent, date)
	if err != nil {
		return nil, fmt.Errorf("class day absence report: load status days: %w", err)
	}
	reported := make(map[int64]bool, len(entries))
	for _, entry := range entries {
		if reportedAbsence(entry.Status) {
			reported[entry.StudentID] = true
		}
	}
	out := make([]int64, 0, len(absent))
	for _, id := range absent {
		if !reported[id] {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// afterMorningMarks runs once a Lehrkraft's morning marks are stored: when a
// marked child misses without a report, it tells the Schulverwaltung how many
// children of the school do.
func (a absenceReports) afterMorningMarks(ctx context.Context, in classday.MorningAttendanceWrite, date calendar.Date) error {
	if a.notifier == nil || a.reads == nil || a.morning == nil || a.rosters == nil {
		return nil
	}
	marked := make([]int64, 0, len(in.Marks))
	for _, mark := range in.Marks {
		if mark.StudentID > 0 && mark.Status == classday.MorningAttendanceAbsent {
			marked = append(marked, mark.StudentID)
		}
	}
	newly, err := a.unreportedAmong(ctx, marked, date)
	if err != nil || len(newly) == 0 {
		return err
	}
	school, err := a.schoolUnreported(ctx, date)
	if err != nil {
		return err
	}
	if len(school) == 0 {
		return nil
	}
	if err := a.notifier.NotifyUnreportedAbsences(ctx, classday.UnreportedAbsenceNotice{
		Date: date, StudentIDs: school, ActorAccountID: in.ActorAccountID,
	}); err != nil {
		return fmt.Errorf("class day morning attendance: notify unreported absences: %w", err)
	}
	return nil
}

// schoolUnreported returns every child of the school's roster that misses
// without a report on date.
func (a absenceReports) schoolUnreported(ctx context.Context, date calendar.Date) ([]int64, error) {
	roster, err := a.rosters.SchoolRosterDay(ctx, date)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	if roster != nil {
		for _, row := range roster.Rows {
			if !row.ListEntry && row.StudentID > 0 {
				ids = append(ids, row.StudentID)
			}
		}
	}
	slices.Sort(ids)
	return a.unreportedAmong(ctx, slices.Compact(ids), date)
}

// unreportedLabel and absenceReportLabel are the change-history values of a
// report: the child was missing without a report, then sick or excused.
func unreportedLabel(date calendar.Date) string {
	return "Fehlt ohne Meldung am " + date.Format("02.01.2006")
}

func absenceReportLabel(status string, date calendar.Date) string {
	label := "Krank"
	if status == classday.AbsenceReportExcused {
		label = "Entschuldigt"
	}
	return label + " am " + date.Format("02.01.2006") + " (nachgetragen in moto schule)"
}
