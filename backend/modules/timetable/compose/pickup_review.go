package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

// NewPickupReviewQueries needs no roster-wide reader or mutation collaborators.
func NewPickupReviewQueries(db *bun.DB, observe func(Observation)) (timetable.PickupReviewQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("pickup review queries: database and observer are required")
	}
	return pickupReviewQueries{query: application.NewPickupReviewQueries(postgres.New(databaseRuntime(db)), observe)}, nil
}

type pickupReviewQueries struct {
	query *application.PickupReviewQueries
}

func (q pickupReviewQueries) PreviewPickupBlocks(ctx context.Context, input timetable.PickupReviewInput) ([]timetable.PartialAbsenceBlock, error) {
	rows, err := q.query.PreviewPickupBlocks(ctx, input.StudentID, input.Date, input.From, input.Enrolled, input.AutoExceptionIDs)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.PartialAbsenceBlock, 0, len(rows))
	for _, row := range rows {
		result = append(result, timetable.PartialAbsenceBlock{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return result, nil
}
