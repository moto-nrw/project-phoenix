package studentpresence

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
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
			if sickRow == nil || row.ReportedAt.After(sickRow.ReportedAt) {
				sickRow = row
			}
		case absencerecords.StudentStatusDayClassTrip:
			if classTripRow == nil || row.ReportedAt.After(classTripRow.ReportedAt) {
				classTripRow = row
			}
		case absencerecords.StudentStatusDayExcused:
			if excusedRow == nil || row.ReportedAt.After(excusedRow.ReportedAt) {
				excusedRow = row
			}
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

func statusDayReportedPtr(v time.Time) *time.Time {
	return &v
}
