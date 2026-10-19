package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

func NewParticipantDirectory(find mealplan.ParticipantFinder) participantDirectory {
	return participantDirectory{find: find}
}

type participantDirectory struct{ find mealplan.ParticipantFinder }

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
