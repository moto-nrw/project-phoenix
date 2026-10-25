package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewParticipantDirectory(find mealplan.ParticipantFinder, names mealplan.StudentNameFinder) participantDirectory {
	return participantDirectory{find: find, names: names}
}

type participantDirectory struct {
	find  mealplan.ParticipantFinder
	names mealplan.StudentNameFinder
}

func (d participantDirectory) FindStudentNames(ctx context.Context, ids []int64) (map[int64]domain.DailyCandidate, domain.OperationStats, error) {
	started := time.Now()
	rows, err := d.names(ctx, ids)
	names := make(map[int64]domain.DailyCandidate, len(rows))
	for _, row := range rows {
		names[row.StudentID] = domain.DailyCandidate(row)
	}
	return names, domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}, err
}

func (d participantDirectory) FindDailyCandidates(ctx context.Context, date domain.Date) ([]domain.DailyCandidate, domain.OperationStats, error) {
	started := time.Now()
	rows, err := d.find(ctx, date.String())
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	candidates := make([]domain.DailyCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, domain.DailyCandidate{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		})
	}
	return candidates, stats, nil
}

func NewSchoolCalendar(find mealplan.CalendarFinder) schoolCalendar {
	return schoolCalendar{find: find}
}

type schoolCalendar struct{ find mealplan.CalendarFinder }

func (c schoolCalendar) Days(ctx context.Context, from, to domain.Date) (domain.CalendarDays, domain.OperationStats, error) {
	started := time.Now()
	closed, periods, err := c.find(ctx, from.String(), to.String())
	stats := domain.OperationStats{Queries: 3, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.CalendarDays{}, stats, err
	}
	holidays := make([]domain.HolidayPeriod, 0, len(periods))
	for _, period := range periods {
		start, startErr := calendar.ParseDate(period.Start)
		end, endErr := calendar.ParseDate(period.End)
		if startErr != nil || endErr != nil {
			return domain.CalendarDays{}, stats, fmt.Errorf("meal plan: holiday period %q to %q: %w", period.Start, period.End, errors.Join(startErr, endErr))
		}
		holidays = append(holidays, domain.HolidayPeriod{Start: start, End: end})
	}
	return domain.NewCalendarDays(closed, holidays, from, to), stats, nil
}
