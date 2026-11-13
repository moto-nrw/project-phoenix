package owners

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func mealPlanParticipantDaysFinder(people peopledirectory.Capability, now func() time.Time) mealplan.ParticipantDaysFinder {
	return func(ctx context.Context, dates []string) (map[string][]mealplan.ParticipantCandidate, error) {
		rows, err := peopledirectory.StudentRostersOnDates(ctx, people, dates, calendar.DateFromTime(now()).String())
		if err != nil {
			return nil, err
		}
		result := make(map[string][]mealplan.ParticipantCandidate, len(rows))
		for date, entries := range rows {
			candidates := make([]mealplan.ParticipantCandidate, 0, len(entries))
			for _, row := range entries {
				if row.Record == nil {
					continue
				}
				candidates = append(candidates, mealplan.ParticipantCandidate{
					StudentID: row.Record.ID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.Record.SchoolClass,
				})
			}
			result[date] = candidates
		}
		return result, nil
	}
}
