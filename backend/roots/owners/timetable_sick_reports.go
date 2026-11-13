package owners

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// SickReportAbsences is the one Workforce absence read the materializer's
// sick-report port needs.
type SickReportAbsences interface {
	ListStaffAbsences(context.Context, workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
}

// NewTimetableSickReports binds the materializer's sick-report port to
// Workforce's absence query (#6292). It reads in the caller's tenant
// transaction, so row-level security scopes the reports to the school.
func NewTimetableSickReports(absences SickReportAbsences) timetablePorts.SickReportReader {
	return timetableSickReports{absences: absences}
}

type timetableSickReports struct {
	absences SickReportAbsences
}

// OpenSickReports lists the absences in force overlapping [from, to] the way
// Workforce hands them to the sick cascade (#6292): every type since #6847, a
// reported entry or an approved request; a single half day never cascades,
// and the half boundary days of a range are skipped.
func (r timetableSickReports) OpenSickReports(ctx context.Context, from, to calendar.Date) ([]timetable.SickReport, error) {
	rows, err := r.absences.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		Statuses:    workforce.PlanEffectiveAbsenceStatuses(),
		OverlapFrom: from.String(),
		OverlapTo:   to.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list absences: %w", err)
	}
	reports := make([]timetable.SickReport, 0, len(rows))
	for _, row := range rows {
		start, err := calendar.ParseDate(row.DateStart)
		if err != nil {
			return nil, fmt.Errorf("absence %d has an invalid date_start %q: %w", row.ID, row.DateStart, err)
		}
		end, err := calendar.ParseDate(row.DateEnd)
		if err != nil {
			return nil, fmt.Errorf("absence %d has an invalid date_end %q: %w", row.ID, row.DateEnd, err)
		}
		skipStart, skipEnd := row.HalfBoundaryDays()
		reports = append(reports, timetable.SickReport{
			AbsenceID: row.ID, StaffID: row.StaffID, Sick: row.AbsenceType == workforce.AbsenceTypeSick,
			DateStart: start, DateEnd: end, SkipStartDay: skipStart, SkipEndDay: skipEnd,
		})
	}
	return reports, nil
}
