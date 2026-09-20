package compose

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// classArrivalExceptionRepository serves the retained class arrival exception
// contract from the Timetable persistence adapter (#3220).
type classArrivalExceptionRepository struct {
	store *postgres.Store
}

// NewClassArrivalExceptionRepository binds the retained class arrival
// exception contract. Timetable owns education.class_arrival_exceptions.
func NewClassArrivalExceptionRepository(db *bun.DB) schedule.ClassArrivalExceptionRepository {
	return classArrivalExceptionRepository{store: postgres.New(databaseRuntime(db))}
}

// Upsert replaces the exception of one class and date.
func (r classArrivalExceptionRepository) Upsert(ctx context.Context, row *schedule.ClassArrivalException) error {
	if err := r.store.UpsertClassArrivalException(ctx, row); err != nil {
		return &modelBase.DatabaseError{Op: "upsert class arrival exception", Err: translateNotFound(err)}
	}
	return nil
}
