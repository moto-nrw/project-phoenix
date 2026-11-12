package studentpresence

import (
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
)

// EffectiveStatus is the resolved status precedence for a set of active
// status-day rows: sick wins over class trip wins over excused, with ties
// broken by the latest ReportedAt.
type EffectiveStatus struct {
	Sick           bool
	ClassTrip      bool
	Excused        bool
	SickSince      *time.Time
	ClassTripSince *time.Time
	ExcusedSince   *time.Time
}

// ResolveEffectiveStatus applies the status-day precedence rule to already
// fetched rows. It is a pure function with no I/O; callers map the result onto
// their own response shape (the excused branch is intentionally reported
// independently so callers can gate it on a pre-existing sick flag).
func ResolveEffectiveStatus(statusRows []*absencerecords.StudentStatusDay) EffectiveStatus {
	var sickRow, classTripRow, excusedRow *absencerecords.StudentStatusDay
	for _, row := range statusRows {
		switch row.Status {
		case absencerecords.StudentStatusDaySick:
			sickRow = latestReportedStatusDay(sickRow, row)
		case absencerecords.StudentStatusDayClassTrip:
			classTripRow = latestReportedStatusDay(classTripRow, row)
		case absencerecords.StudentStatusDayExcused:
			excusedRow = latestReportedStatusDay(excusedRow, row)
		}
	}

	var eff EffectiveStatus
	if sickRow != nil {
		eff.Sick = true
		eff.SickSince = statusDayReportedPtr(sickRow.ReportedAt)
		return eff
	}
	if classTripRow != nil {
		eff.ClassTrip = true
		eff.ClassTripSince = statusDayReportedPtr(classTripRow.ReportedAt)
		return eff
	}
	if excusedRow != nil {
		eff.Excused = true
		eff.ExcusedSince = statusDayReportedPtr(excusedRow.ReportedAt)
	}
	return eff
}

// latestReportedStatusDay keeps the later reported of two rows; the current
// row wins a tie.
func latestReportedStatusDay(current, row *absencerecords.StudentStatusDay) *absencerecords.StudentStatusDay {
	if current == nil || row.ReportedAt.After(current.ReportedAt) {
		return row
	}
	return current
}

func statusDayReportedPtr(v time.Time) *time.Time {
	return &v
}

// Place before status (#6371): a child with an open check-in counts as present
// in every view of its day, exactly once. A sick, excused or class-trip status
// of that day stays a hint at the child and enters no absence figure, status
// filter or status group. A child without an open check-in keeps its status.

// IsCheckedInLocation reports whether a resolved location is an open
// check-in: a room ("Anwesend - <Raum>"), "Anwesend", the schoolyard or on
// the way. "Abwesend", "Schule", "Im Unterricht" and the empty location of a
// dated list are not.
func IsCheckedInLocation(location string) bool {
	switch location {
	case "Anwesend", YardLocationLabel, "Unterwegs":
		return true
	}
	return strings.HasPrefix(location, "Anwesend - ")
}

// WithoutCheckedIn keeps the children of a status figure who have no open
// check-in, in their order: the status of a checked-in child is only a hint.
func WithoutCheckedIn(studentIDs []int64, checkedIn map[int64]bool) []int64 {
	result := make([]int64, 0, len(studentIDs))
	for _, id := range studentIDs {
		if !checkedIn[id] {
			result = append(result, id)
		}
	}
	return result
}
