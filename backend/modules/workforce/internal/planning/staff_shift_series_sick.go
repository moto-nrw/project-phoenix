package planning

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SeriesSickReportReader lists the staff member's absences the series
// materializer cancels sick days by (#6302). The Workforce capability
// serves it.
type SeriesSickReportReader interface {
	ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
}

// WithStaffShiftSeriesSickReports binds the sick reports a materialized
// occurrence takes the cascade's state from. Without it (unit tests) nobody
// is sick.
func WithStaffShiftSeriesSickReports(reader SeriesSickReportReader) StaffShiftSeriesOption {
	return func(s *staffShiftSeriesService) { s.sickReports = reader }
}

// sickDaysOf reads the staff member's open sick reports over [from, to] and
// returns the report behind each full sick day. It takes no lock of its own:
// the materializer reads under the per-staff shift lock it already holds,
// the lock the sick cascade takes before it reads the shifts (#6302 adds no
// lock and no lock order; the remaining races are #6232's).
func (s *staffShiftSeriesService) sickDaysOf(ctx context.Context, staffID int64, from, to calendar.Date) (map[calendar.Date]int64, error) {
	if s.sickReports == nil {
		return nil, nil
	}
	absences, err := s.sickReports.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		StaffID:     staffID,
		Types:       []string{workforce.AbsenceTypeSick},
		Statuses:    workforce.RangeBlockingAbsenceStatuses(),
		OverlapFrom: from.String(),
		OverlapTo:   to.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list sick reports for series: %w", err)
	}
	return sickDayIndex(absences, from, to)
}

// sickDayIndex maps each full sick day of [from, to] to its report. The day
// rule is the cascade's: a half-day report never cascades, and the half
// boundary days of a range are skipped (timetable.SickReport.CoversFullDay).
// Overlapping reports are merged on write; should two still cover a day, the
// lower ID wins, as in the Betreuungsplan's index.
func sickDayIndex(absences []workforce.StaffAbsence, from, to calendar.Date) (map[calendar.Date]int64, error) {
	days := make(map[calendar.Date]int64)
	for _, absence := range absences {
		if absence.AbsenceType != workforce.AbsenceTypeSick || absence.HalfDay {
			continue
		}
		report, err := sickReportOf(absence)
		if err != nil {
			return nil, err
		}
		indexFullSickDays(days, report, from, to)
	}
	return days, nil
}

// indexFullSickDays enters the report's full sick days within [from, to].
func indexFullSickDays(days map[calendar.Date]int64, report timetable.SickReport, from, to calendar.Date) {
	for day := maxDate(report.DateStart, from); !day.After(report.DateEnd) && !day.After(to); day = day.AddDays(1) {
		if !report.CoversFullDay(day) {
			continue
		}
		if current, ok := days[day]; !ok || report.AbsenceID < current {
			days[day] = report.AbsenceID
		}
	}
}

func sickReportOf(absence workforce.StaffAbsence) (timetable.SickReport, error) {
	start, err := calendar.ParseDate(absence.DateStart)
	if err != nil {
		return timetable.SickReport{}, fmt.Errorf("sick report %d has an invalid date_start %q: %w", absence.ID, absence.DateStart, err)
	}
	end, err := calendar.ParseDate(absence.DateEnd)
	if err != nil {
		return timetable.SickReport{}, fmt.Errorf("sick report %d has an invalid date_end %q: %w", absence.ID, absence.DateEnd, err)
	}
	return timetable.SickReport{
		AbsenceID: absence.ID, StaffID: absence.StaffID, DateStart: start, DateEnd: end,
		SkipStartDay: absence.StartHalfDay, SkipEndDay: absence.EndHalfDay,
	}, nil
}

func maxDate(a, b calendar.Date) calendar.Date {
	if a.After(b) {
		return a
	}
	return b
}

// applySickReport gives a new occurrence the state the sick cascade gives an
// occurrence planned before the report: cancelled with the neutral reason and
// the report's stamp, and detached, because the cascade's cancellation is an
// edit of the occurrence (#1889). Deleting or shortening the report then
// reactivates it like any other stamped shift.
func applySickReport(shift *workforce.ShiftRecord, absenceID int64) {
	reason := workforce.SickShiftChangeReason
	shift.Cancelled = true
	shift.ChangeReason = &reason
	shift.SickAbsenceID = &absenceID
	shift.Detached = true
}
