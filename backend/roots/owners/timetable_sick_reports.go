package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// openSickReportStatuses are the statuses a sick report blocks its range
// with; declined and cancelled entries take no plan effect.
var openSickReportStatuses = []string{
	workforce.AbsenceStatusReported, workforce.AbsenceStatusRequested,
	workforce.AbsenceStatusQuestion, workforce.AbsenceStatusApproved,
}

// NewTimetableSickReports binds the materializer's sick-report port to
// Workforce's absence query (#6292). It reads in the caller's tenant
// transaction, so row-level security scopes the reports to the school.
func NewTimetableSickReports(absences workforce.AbsenceQuery) timetablePorts.SickReportReader {
	return timetableSickReports{absences: absences}
}

type timetableSickReports struct {
	absences workforce.AbsenceQuery
}

// OpenSickReports lists the full-day sick reports overlapping [from, to] the
// way Workforce hands them to the sick cascade: a half-day report never
// cascades, and the half boundary days of a range are skipped.
func (r timetableSickReports) OpenSickReports(ctx context.Context, from, to calendar.Date) ([]timetable.SickReport, error) {
	rows, err := r.absences.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		Types:       []string{workforce.AbsenceTypeSick},
		Statuses:    openSickReportStatuses,
		OverlapFrom: from.String(),
		OverlapTo:   to.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list sick reports: %w", err)
	}
	reports := make([]timetable.SickReport, 0, len(rows))
	for _, row := range rows {
		if row.HalfDay {
			continue
		}
		start, err := calendar.ParseDate(row.DateStart)
		if err != nil {
			return nil, fmt.Errorf("sick report %d has an invalid date_start %q: %w", row.ID, row.DateStart, err)
		}
		end, err := calendar.ParseDate(row.DateEnd)
		if err != nil {
			return nil, fmt.Errorf("sick report %d has an invalid date_end %q: %w", row.ID, row.DateEnd, err)
		}
		reports = append(reports, timetable.SickReport{
			AbsenceID: row.ID, StaffID: row.StaffID, DateStart: start, DateEnd: end,
			SkipStartDay: row.StartHalfDay, SkipEndDay: row.EndHalfDay,
		})
	}
	return reports, nil
}
