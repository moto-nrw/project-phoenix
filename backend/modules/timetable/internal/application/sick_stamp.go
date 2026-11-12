package application

import (
	"context"
	"sort"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The sick stamp is the plan effect of a sick report on one block assignment:
// absent, with the neutral reason and the report's provenance id (#1843). The
// cascade finds the rows a shortened or deleted report must release only by
// that id (#6292). Invariant I-1: every path that creates an assignment, or
// rewrites one from an earlier state, writes it through writeStaffAssignment
// with the sick days sickDaysIn read. #6300 adds no lock and no lock order;
// concurrent sick-report writes are #6232's.

// writeStaffAssignment is the one write of a new or rebuilt assignment
// (I-1). A row that carries the same person's state of the same day keeps
// it: a planner's manual absence or restored presence stays. Every other row
// takes the sick state of its day from sickDays (applySickReportOfDay). A row
// without an id is created, one with an id updated.
func writeStaffAssignment(ctx context.Context, staff timetable.InstanceStaffCommand, row *scheduleModel.InstanceStaff, day calendar.Date, sickDays map[staffDay]int64, carried bool) error {
	if !carried {
		applySickReportOfDay(row, sickDays[staffDay{staffID: row.StaffID, date: day}])
	}
	if row.ID == 0 {
		return createLegacyInstanceStaff(ctx, staff, row)
	}
	return updateLegacyInstanceStaff(ctx, staff, row)
}

// applySickReportOfDay gives a row the sick state of its day: absent with the
// report's stamp on a full sick day, released from a carried stamp otherwise.
// A manual absence stays, as the cascade leaves an already absent row alone.
func applySickReportOfDay(row *scheduleModel.InstanceStaff, absenceID int64) {
	switch {
	case row.IsAbsent && row.SickAbsenceID == nil:
	case absenceID > 0:
		stampSickAbsence(row, absenceID)
	case row.SickAbsenceID != nil:
		copyAbsenceState(row, &scheduleModel.InstanceStaff{})
	}
}

// sickDaysIn reads the open sick reports over the days (I-1). It takes no
// lock: each caller reads them inside the locks its path already holds
// (#6292 adds no lock and no lock order). Without a reader nobody is sick.
func sickDaysIn(ctx context.Context, reports ports.SickReportReader, days ...calendar.Date) (map[staffDay]int64, error) {
	if reports == nil || len(days) == 0 {
		return nil, nil
	}
	days = ascendingDays(days)
	from, to := days[0], days[len(days)-1]
	open, err := reports.OpenSickReports(ctx, from, to)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "sick stamp: load sick reports", Err: err}
	}
	return sickDayIndex(open, from, to), nil
}

// ascendingDays lists the distinct days, ascending.
func ascendingDays(days []calendar.Date) []calendar.Date {
	seen := make(map[calendar.Date]bool, len(days))
	unique := make([]calendar.Date, 0, len(days))
	for _, day := range days {
		if !seen[day] {
			seen[day] = true
			unique = append(unique, day)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].Before(unique[j]) })
	return unique
}

// daysBetween lists every day of [from, to].
func daysBetween(from, to calendar.Date) []calendar.Date {
	days := make([]calendar.Date, 0, from.DaysUntil(to)+1)
	for day := from; !day.After(to); day = day.AddDays(1) {
		days = append(days, day)
	}
	return days
}

// copyAbsenceState carries the absence of src, including its sick
// provenance, onto a rebuilt row.
func copyAbsenceState(dst, src *scheduleModel.InstanceStaff) {
	dst.IsAbsent = src.IsAbsent
	dst.AbsenceReason = src.AbsenceReason
	dst.SickAbsenceID = src.SickAbsenceID
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
