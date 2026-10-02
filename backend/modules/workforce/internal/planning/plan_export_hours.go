package planning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/planexport"
)

// planExportHours serves the hours sheet of the printable Dienstplan (#3819)
// from the same weekly summaries the Dienstplan row header shows, so the
// sheet cannot disagree with the screen it was printed from.
type planExportHours struct {
	overview StaffScheduleOverviewGetter
}

func (h planExportHours) WeeklyHours(ctx context.Context, from, to planexport.Date) (planexport.WeeklyHours, error) {
	fromDate, err := planningDate(string(from), "from")
	if err != nil {
		return planexport.WeeklyHours{}, err
	}
	toDate, err := planningDate(string(to), "to")
	if err != nil {
		return planexport.WeeklyHours{}, err
	}
	overview, err := h.overview.GetOverview(ctx, fromDate, toDate)
	if err != nil {
		return planexport.WeeklyHours{}, err
	}
	return weeklyHoursFromOverview(overview), nil
}

func weeklyHoursFromOverview(overview *StaffScheduleOverview) planexport.WeeklyHours {
	if overview == nil {
		return planexport.WeeklyHours{}
	}
	hours := planexport.WeeklyHours{
		Staff:     make([]*planexport.StaffMember, 0, len(overview.Staff)),
		Summaries: make([]planexport.WeeklySummary, 0, len(overview.WeeklySummaries)),
	}
	for _, member := range overview.Staff {
		if member == nil || member.Person == nil {
			continue
		}
		hours.Staff = append(hours.Staff, &planexport.StaffMember{ID: member.ID, FirstName: member.Person.FirstName, LastName: member.Person.LastName})
	}
	for _, summary := range overview.WeeklySummaries {
		byType := make([]planexport.ShiftTypeMinutes, 0, len(summary.ByShiftType))
		for _, entry := range summary.ByShiftType {
			byType = append(byType, planexport.ShiftTypeMinutes{ShiftTypeID: entry.ShiftTypeID, Minutes: entry.Minutes})
		}
		hours.Summaries = append(hours.Summaries, planexport.WeeklySummary{
			StaffID: summary.StaffID, WeekStart: planexport.Date(summary.WeekStart.String()),
			PlannedMinutes: summary.PlannedMinutes, TargetMinutes: summary.TargetMinutes,
			DeltaMinutes: summary.DeltaMinutes, ByShiftType: byType,
		})
	}
	return hours
}
