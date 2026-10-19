//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/uptrace/bun"
)

// classArrivalExceptionRepository serves the retained contract over Timetable (#3220).
type classArrivalExceptionRepository struct {
	store *postgres.Store
}

// NewClassArrivalExceptionRepository binds Timetable-owned education.class_arrival_exceptions.
func NewClassArrivalExceptionRepository(db *bun.DB) ports.ClassArrivalExceptionRecords {
	return classArrivalExceptionRepository{store: postgres.New(databaseRuntime(db))}
}

// Upsert replaces the exception of one class and date.
func (r classArrivalExceptionRepository) Upsert(ctx context.Context, row *schedule.ClassArrivalException) error {
	if err := r.store.UpsertClassArrivalException(ctx, row); err != nil {
		return &domain.StoreError{Op: "upsert class arrival exception", Err: postgres.TranslateNotFound(err)}
	}
	return nil
}
