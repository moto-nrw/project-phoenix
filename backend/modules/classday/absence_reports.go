package classday

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Absence reports the Schulverwaltung enters afterwards (#4791). The
// Lehrkraft marks a child "fehlt" in the morning (#4790); when nobody
// reported the child, the Schulverwaltung learns why (a call from the
// parents) and enters the report: sick or excused for today, the status day
// the OGS enters itself. Only holders of class_day:absence_report_write
// enter one, and only they take it back.
const (
	AbsenceReportSick    = "sick"
	AbsenceReportExcused = "excused"
)

// ErrAbsenceReportsNotConfigured is returned when the projection was composed
// without the status-day or audit seam of the absence reports.
var ErrAbsenceReportsNotConfigured = errors.New("absence reports are not configured")

// Sentinels the HTTP layer classifies for the absence reports (#4791).
var (
	// ErrAbsenceReportInvalid refuses a report without a child or with a
	// status other than sick or excused.
	ErrAbsenceReportInvalid = errors.New("absence report is invalid")
	// ErrAbsenceReportNotToday refuses a report or withdrawal for any day
	// but today: the report explains this morning.
	ErrAbsenceReportNotToday = errors.New("an absence report can only be entered for today")
	// ErrAbsenceReportNoSchoolDay refuses a report on a day without school.
	ErrAbsenceReportNoSchoolDay = errors.New("an absence report can only be entered on a school day")
	// ErrAbsenceReportUnknownChild refuses a child with an OGS record that is
	// not on the class roster of the day.
	ErrAbsenceReportUnknownChild = errors.New("child with an OGS record is not on the class roster of the day")
	// ErrAbsenceReportNotMissing refuses a report for a child that does not
	// miss without a report: not marked absent by the Lehrkraft, or already
	// reported sick, excused or on a class trip.
	ErrAbsenceReportNotMissing = errors.New("the child is not missing without a report")
	// ErrAbsenceReportConflict means the OGS holds an entry for the day the
	// report may not overwrite, such as a partial absence. Nothing is written.
	ErrAbsenceReportConflict = errors.New("the child already has an entry for today")
	// ErrAbsenceReportNotFound answers a withdrawal without a report the
	// school entered.
	ErrAbsenceReportNotFound = errors.New("there is no absence report of the school to withdraw")
)

// AbsenceReportActor is the account that enters or withdraws a report. The
// child's change history names it.
type AbsenceReportActor struct {
	AccountID int64
	Name      string
}

// AbsenceReportWrite is one report the Schulverwaltung enters for a child of
// a class it may see.
type AbsenceReportWrite struct {
	SchoolClass string
	Date        Date
	StudentID   int64
	// Status is AbsenceReportSick or AbsenceReportExcused.
	Status string
	Actor  AbsenceReportActor
}

// AbsenceReportWithdrawal takes back the report the school entered for a
// child on a date.
type AbsenceReportWithdrawal struct {
	SchoolClass string
	Date        Date
	StudentID   int64
	Actor       AbsenceReportActor
}

// AbsenceReports is the absence-report write of moto schule (#4791), a facade
// of its own beside ClassDay. The capability ClassDay returns implements it;
// the HTTP adapter serves it only to holders of
// class_day:absence_report_write. Both writes go into the child's change
// history.
type AbsenceReports interface {
	// ReportAbsence enters sick or excused for today for a child of the
	// class the Lehrkraft marked absent without a report.
	ReportAbsence(ctx context.Context, in AbsenceReportWrite) error
	// WithdrawAbsenceReport takes back the report the school entered today.
	WithdrawAbsenceReport(ctx context.Context, in AbsenceReportWithdrawal) error
}

// AbsenceReportLogEntry is one report or withdrawal in a child's change
// history. The values are German display text.
type AbsenceReportLogEntry struct {
	StudentID int64
	Actor     AbsenceReportActor
	OldValue  string
	NewValue  string
}

// UnreportedAbsenceNotice is the school's children missing without a report
// on Date, after a morning mark of ActorAccountID.
type UnreportedAbsenceNotice struct {
	Date           calendar.Date
	StudentIDs     []int64
	ActorAccountID int64
}
