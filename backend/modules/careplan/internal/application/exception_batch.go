package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// ApplyExceptionBatch validates the entire change before deleting anything.
func (s *Service) ApplyExceptionBatch(ctx context.Context, batch careplan.ExceptionBatch) error {
	if err := validateExceptionBatch(batch); err != nil {
		return err
	}
	return s.run(ctx, "apply_exception_batch", func(ctx context.Context, stats *domain.OperationStats) error {
		removedArrivals, err := s.store.DeleteArrivalExceptions(ctx, batch.DeleteArrivals)
		stats.Add(removedArrivals)
		if err != nil {
			return err
		}
		removedPickups, err := s.store.DeletePickupExceptions(ctx, batch.DeletePickups)
		stats.Add(removedPickups)
		if err != nil {
			return err
		}
		addedArrivals, err := s.store.CreateArrivalExceptions(ctx, batch.Arrivals)
		stats.Add(addedArrivals)
		if err != nil {
			return err
		}
		addedPickups, err := s.store.CreatePickupExceptions(ctx, batch.Pickups)
		stats.Add(addedPickups)
		return err
	})
}

func validateExceptionBatch(batch careplan.ExceptionBatch) error {
	for _, value := range batch.Arrivals {
		if !validateArrivalException(value) {
			return scheduleInvalid()
		}
	}
	for _, value := range batch.Pickups {
		if !validatePickupException(value) {
			return scheduleInvalid()
		}
		if err := value.Validate(); err != nil {
			return err
		}
	}
	for _, ids := range [][]int64{batch.DeleteArrivals, batch.DeletePickups} {
		for _, id := range ids {
			if id <= 0 {
				return scheduleInvalid()
			}
		}
	}
	return nil
}
