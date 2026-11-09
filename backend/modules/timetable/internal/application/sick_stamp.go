package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The sick stamp is the plan effect of a sick report on one block assignment:
// absent, with the neutral reason and the report's provenance id (#1843). The
// cascade finds the rows a shortened or deleted report must release only by
// that id, so every path that rebuilds an assignment carries the stamp with
// the absence, and every path that creates one for a sick person applies it
// (#6292). Invariant: an assignment made absent by a sick report names it.

// copyAbsenceState carries the absence of src, including its sick
// provenance, onto a rebuilt row.
func copyAbsenceState(dst, src *scheduleModel.InstanceStaff) {
	dst.IsAbsent = src.IsAbsent
	dst.AbsenceReason = src.AbsenceReason
	dst.SickAbsenceID = src.SickAbsenceID
}

// sameAbsenceState reports whether two rows carry the same absence.
func sameAbsenceState(a, b *scheduleModel.InstanceStaff) bool {
	return a.IsAbsent == b.IsAbsent && equalOptional(a.AbsenceReason, b.AbsenceReason) && equalOptional(a.SickAbsenceID, b.SickAbsenceID)
}

func equalOptional[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// stampSickAbsence marks a row absent for one sick report.
func stampSickAbsence(row *scheduleModel.InstanceStaff, absenceID int64) {
	reason := timetable.SickBlockAbsenceReason
	row.IsAbsent = true
	row.AbsenceReason = &reason
	row.SickAbsenceID = &absenceID
}

// staffDay keys one staff member on one calendar day.
type staffDay struct {
	staffID int64
	date    calendar.Date
}

// sickDayIndex maps every full sick day in [from, to] to the report that
// covers it. Overlapping reports of one person are merged by Workforce; should
// two still cover a day, the lower id wins so reruns stamp the same report.
func sickDayIndex(reports []timetable.SickReport, from, to calendar.Date) map[staffDay]int64 {
	index := make(map[staffDay]int64)
	for _, report := range reports {
		for day := from; !day.After(to); day = day.AddDays(1) {
			if !report.CoversFullDay(day) {
				continue
			}
			key := staffDay{staffID: report.StaffID, date: day}
			if current, taken := index[key]; !taken || report.AbsenceID < current {
				index[key] = report.AbsenceID
			}
		}
	}
	return index
}
