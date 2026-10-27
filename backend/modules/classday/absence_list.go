package classday

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Absence list of a class for the Klassenbuch and the Zeugnis (#4793): per
// child the school mornings the child missed in a period, how many of them
// are excused and how many not, and the school days nobody recorded. The
// Lehrkraft reads her assigned classes, the Schulverwaltung every class
// (class_day:all_classes); every read and every export is logged.
//
// One school day counts once per child:
//   - "present" in the morning counts as present, even when a status day
//     says sick (the child was at school);
//   - "absent" in the morning is an absence day: excused with a sick or
//     excused status day (also one the Schulverwaltung entered afterwards,
//     #4791), unexcused without one;
//   - a class trip is no absence day, with or without a morning mark;
//   - without a morning mark a sick or excused status day is an excused
//     absence day, as the class view reads it absent from the report;
//   - a school day with neither is "not recorded", never an absence day.
//
// There are no half days: a status day and a morning mark cover the whole
// day.
type AbsenceList interface {
	// AbsenceListReport builds the list of one class and records the access
	// in the GDPR log.
	AbsenceListReport(ctx context.Context, query AbsenceListQuery) (*AbsenceListReport, error)
	// RenderAbsenceList renders the list as a file and records the export in
	// the GDPR log.
	RenderAbsenceList(ctx context.Context, query AbsenceListQuery, format Format) (File, error)
}

// The columns of the absence list export besides ExportColumnName.
const (
	ExportColumnAbsentDays      ExportColumnID = "absent_days"
	ExportColumnExcusedDays     ExportColumnID = "excused_days"
	ExportColumnUnexcusedDays   ExportColumnID = "unexcused_days"
	ExportColumnNotRecordedDays ExportColumnID = "not_recorded_days"
)

// MaxAbsenceListDays bounds the period of one list: a school year with room
// to spare, so one request cannot read years of marks.
const MaxAbsenceListDays = 400

// AbsenceListQuery selects the list of one class. Without From and To the
// period is the current school half-year of the school calendar.
type AbsenceListQuery struct {
	SchoolClass string
	From        Date
	To          Date
	Actor       Actor
}

// ErrAbsenceListInvalidPeriod refuses a period with only one end, a broken
// date, an end before the start, or more than MaxAbsenceListDays days.
var ErrAbsenceListInvalidPeriod = errors.New("absence list period is invalid")

// ErrAbsenceListNotConfigured is returned when the projection was composed
// without the morning marks, the status days or the school calendar.
var ErrAbsenceListNotConfigured = errors.New("absence list is not configured")

// AbsenceListReport is the absence list of one class in one period.
type AbsenceListReport struct {
	SchoolClass string `json:"school_class"`
	From        Date   `json:"from"`
	To          Date   `json:"to"`
	// PeriodName names the school calendar's half-year the period was
	// preselected from; empty for a chosen period or without a calendar
	// entry.
	PeriodName string `json:"period_name"`
	// CountedUntil is the last day counted: To, or today when To lies
	// ahead. Empty when the whole period lies ahead.
	CountedUntil Date `json:"counted_until"`
	// SchoolDays is the number of school days counted: Monday to Friday
	// without statutory holidays and school holidays.
	SchoolDays int                `json:"school_days"`
	Children   []AbsenceListChild `json:"children"`
	Totals     AbsenceListCounts  `json:"totals"`
}

// AbsenceListChild is one child of the class with its counts. Children with
// an OGS record are listed, also without an OGS place; class-list-only
// entries join once they can carry a report (#4803).
type AbsenceListChild struct {
	StudentID int64  `json:"student_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AbsenceListCounts
}

// AbsenceListCounts are the counted days of one child or of the class.
type AbsenceListCounts struct {
	// AbsentDays is every absence day; Excused and Unexcused split it.
	AbsentDays    int `json:"absent_days"`
	ExcusedDays   int `json:"excused_days"`
	UnexcusedDays int `json:"unexcused_days"`
	// NotRecordedDays are school days without a morning mark and without a
	// status day.
	NotRecordedDays int `json:"not_recorded_days"`
}

// Values the absence list reads through its ports; the root fills them from
// Student Presence, Care Plan and the School Calendar.
type (
	// MorningMark is one stored morning mark of a child: a student
	// (StudentID) or a class-list-only entry (ListEntryID).
	MorningMark struct {
		StudentID   int64
		ListEntryID int64
		Date        calendar.Date
		Status      string
	}
	// DatedStatusDay is one sick, excused or class-trip day of a student
	// that still stands: active, or archived at the end of its day.
	DatedStatusDay struct {
		StudentID int64
		Date      calendar.Date
		Status    string
	}
	// HalfYear is a school half-year of the school calendar.
	HalfYear struct {
		Name string
		From calendar.Date
		To   calendar.Date
	}
)
