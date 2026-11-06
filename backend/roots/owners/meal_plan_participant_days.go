package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type mealPlanPeople interface {
	studentNames
	ListStudentRosterOnDates(context.Context, []string, string) (map[string][]peopledirectory.StudentRosterEntry, error)
}

func mealPlanParticipantDaysFinder(people mealPlanPeople, now func() time.Time) mealplan.ParticipantDaysFinder {
	return func(ctx context.Context, dates []string) (map[string][]mealplan.ParticipantCandidate, error) {
		rows, err := people.ListStudentRosterOnDates(ctx, dates, calendar.DateFromTime(now()).String())
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
