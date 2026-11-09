package timetable

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// SickBlockAbsenceReason is the neutral label a sick report writes on the
// block assignments it marks absent (#1843). The absence note never reaches
// the plan: plan surfaces are team-visible and the note may carry private
// medical detail.
const SickBlockAbsenceReason = "Krankmeldung"

// SickReport is one sick report as the Betreuungsplan sees it: who is sick,
// for which range, and whether its boundary days are half days (#1843). Both
// the sick cascade and the materializer decide the affected days with
// CoversFullDay, so a block assignment created before or after the report
// gets the same absence (#6292).
type SickReport struct {
	AbsenceID    int64
	StaffID      int64
	DateStart    calendar.Date
	DateEnd      calendar.Date
	SkipStartDay bool
	SkipEndDay   bool
}

// CoversFullDay reports whether day is a full sick day of the report. A half
// boundary day never makes a block absent: the person works the other half,
// and which block that touches is the admin's call.
func (r SickReport) CoversFullDay(day calendar.Date) bool {
	if day.Before(r.DateStart) || day.After(r.DateEnd) {
		return false
	}
	if r.SkipStartDay && day == r.DateStart {
		return false
	}
	return !r.SkipEndDay || day != r.DateEnd
}
