package planning

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// SickReportReader lists a staff member's absences the shift writes take the
// absence state of a day from (#6302): the series materializer and the single
// create and move. Since #6847 every absence type in force counts, not only a
// sick report. The Workforce capability serves it.
type SickReportReader interface {
	ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
}

// WithStaffShiftSeriesSickReports binds the absences a materialized
// occurrence takes the cascade's state from. Without it (unit tests) nobody
// is absent.
func WithStaffShiftSeriesSickReports(reader SickReportReader) StaffShiftSeriesOption {
	return func(s *staffShiftSeriesService) { s.sickReports = reader }
}

// absenceStamp names the absence behind one full absence day: its id and its
// canonical type, which decides the shift's label (#6847).
type absenceStamp struct {
	absenceID   int64
	absenceType string
}

// sickDaysIn reads the staff member's absences in force over [from, to] and
// returns the absence behind each full absence day. It takes no lock of its
// own: every caller reads under the per-staff shift lock it already holds,
// the lock the sick cascade takes before it reads the shifts (#6302 adds no
// lock and no lock order; the remaining races are #6232's). Without a reader
// nobody is absent.
func sickDaysIn(ctx context.Context, reports SickReportReader, staffID int64, from, to calendar.Date) (map[calendar.Date]absenceStamp, error) {
	if reports == nil {
		return nil, nil
	}
	absences, err := reports.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		StaffID:     staffID,
		Statuses:    workforce.PlanEffectiveAbsenceStatuses(),
		OverlapFrom: from.String(),
		OverlapTo:   to.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list absences: %w", err)
	}
	return sickDayIndex(absences, from, to)
}

// sickDayIndex maps each full absence day of [from, to] to its absence. The
// day rule is the cascade's: a single half day never cascades, and the half
// boundary days of a range are skipped (timetable.SickReport.CoversFullDay).
// An absence not in force (a pending, declined or canceled request) is left
// out. Overlapping absences are merged on write; should two still cover a
// day, the lower ID wins, as in the Betreuungsplan's index.
func sickDayIndex(absences []workforce.StaffAbsence, from, to calendar.Date) (map[calendar.Date]absenceStamp, error) {
	days := make(map[calendar.Date]absenceStamp)
	for _, absence := range absences {
		if !slices.Contains(workforce.PlanEffectiveAbsenceStatuses(), absence.Status) {
			continue
		}
		report, err := sickReportOf(absence)
		if err != nil {
			return nil, err
		}
		indexFullSickDays(days, report, absence.AbsenceType, from, to)
	}
	return days, nil
}

// indexFullSickDays enters the absence's full days within [from, to].
func indexFullSickDays(days map[calendar.Date]absenceStamp, report timetable.SickReport, absenceType string, from, to calendar.Date) {
	for day := maxDate(report.DateStart, from); !day.After(report.DateEnd) && !day.After(to); day = day.AddDays(1) {
		if !report.CoversFullDay(day) {
			continue
		}
		if current, ok := days[day]; !ok || report.AbsenceID < current.absenceID {
			days[day] = absenceStamp{absenceID: report.AbsenceID, absenceType: absenceType}
		}
	}
}

func sickReportOf(absence workforce.StaffAbsence) (timetable.SickReport, error) {
	start, err := calendar.ParseDate(absence.DateStart)
	if err != nil {
		return timetable.SickReport{}, fmt.Errorf("absence %d has an invalid date_start %q: %w", absence.ID, absence.DateStart, err)
	}
	end, err := calendar.ParseDate(absence.DateEnd)
	if err != nil {
		return timetable.SickReport{}, fmt.Errorf("absence %d has an invalid date_end %q: %w", absence.ID, absence.DateEnd, err)
	}
	skipStart, skipEnd := absence.HalfBoundaryDays()
	return timetable.SickReport{
		AbsenceID: absence.ID, StaffID: absence.StaffID, Sick: absence.AbsenceType == workforce.AbsenceTypeSick,
		DateStart: start, DateEnd: end, SkipStartDay: skipStart, SkipEndDay: skipEnd,
	}, nil
}

func maxDate(a, b calendar.Date) calendar.Date {
	if a.After(b) {
		return a
	}
	return b
}

// applySickReport gives a shift the state the sick cascade gives a shift
// planned before the absence: cancelled with the neutral reason of its type
// and the absence's stamp. A series row is also detached, because the cascade's
// cancellation is an edit of the occurrence (#1889). Deleting or shortening
// the report then reactivates it like any other stamped shift.
func applySickReport(shift *workforce.ShiftRecord, stamp absenceStamp) {
	reason := workforce.ShiftChangeReasonFor(stamp.absenceType)
	absenceID := stamp.absenceID
	shift.Cancelled = true
	shift.ChangeReason = &reason
	shift.SickAbsenceID = &absenceID
	shift.Detached = shift.Detached || shift.SeriesID != nil
}
