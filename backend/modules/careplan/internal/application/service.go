package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type Service struct {
	store   ports.Store
	observe ports.Observer
}

func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("care plan application: store and observer are required")
	}
	return &Service{store: store, observe: observe}
}

func (s *Service) FindCareOffering(ctx context.Context, id int64) (result domain.CareOffering, err error) {
	err = s.run("find_care_offering", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCareOffering(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrCareOfferingNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListCareOfferings(ctx context.Context, filter domain.CareOfferingFilter) (result []domain.CareOffering, err error) {
	err = s.run("list_care_offerings", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListCareOfferings(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (result int, err error) {
	err = s.run("count_care_offerings_by_phase", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountCareOfferingsByPhase(ctx, phaseID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateCareOffering(ctx context.Context, fields domain.CareOfferingFields) (result domain.CareOffering, err error) {
	err = s.run("create_care_offering", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateCareOffering(ctx, fields)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		queryStats, err = s.store.ReplaceAutoAddTriggers(ctx, result.ID, fields.AutoAddTriggerOfferingIDs)
		stats.Add(queryStats)
		if err == nil {
			result.AutoAddTriggerOfferingIDs = append([]int64(nil), fields.AutoAddTriggerOfferingIDs...)
		}
		return err
	})
	return result, err
}

func (s *Service) UpdateCareOffering(ctx context.Context, id int64, fields domain.CareOfferingFields) (result domain.CareOffering, err error) {
	err = s.run("update_care_offering", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UpdateCareOffering(ctx, id, fields)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		queryStats, err = s.store.ReplaceAutoAddTriggers(ctx, result.ID, fields.AutoAddTriggerOfferingIDs)
		stats.Add(queryStats)
		if err == nil {
			result.AutoAddTriggerOfferingIDs = append([]int64(nil), fields.AutoAddTriggerOfferingIDs...)
		}
		return err
	})
	return result, err
}

func (s *Service) DeleteCareOffering(ctx context.Context, id int64) (err error) {
	return s.run("delete_care_offering", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteCareOffering(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	return s.run("replace_care_offering_auto_triggers", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.ReplaceAutoAddTriggers(ctx, id, triggers)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) FindOfferingChange(ctx context.Context, id int64, lock bool) (result domain.OfferingChangeRequest, err error) {
	err = s.run("find_offering_change", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindOfferingChange(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrOfferingChangeNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListOfferingChanges(ctx context.Context, filter domain.OfferingChangeFilter) (result []domain.OfferingChangeRequest, err error) {
	err = s.run("list_offering_changes", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListOfferingChanges(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateOfferingChange(ctx context.Context, row domain.OfferingChangeRequest) (result domain.OfferingChangeRequest, err error) {
	err = s.run("create_offering_change", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateOfferingChange(ctx, row)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) error {
	return s.run("update_offering_change_effective_from", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateOfferingChangeEffectiveFrom(ctx, id, date)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	return s.run("update_offering_change_withdrawal", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateApprovedCompleteWithdrawal(ctx, id, complete)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdatePendingOfferingChange(ctx context.Context, input domain.UpdatePendingOfferingChange) error {
	return s.run("update_pending_offering_change", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdatePendingOfferingChange(ctx, input)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DecideOfferingChange(ctx context.Context, input domain.DecideOfferingChange) error {
	return s.run("decide_offering_change", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.DecideOfferingChange(ctx, input)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return s.run("update_offering_change_snapshot", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateOfferingChangeSnapshot(ctx, id, snapshot)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (rows int64, err error) {
	err = s.run("close_pending_offering_changes", func(stats *domain.OperationStats) error {
		queryStats, commandErr := s.store.ClosePendingOfferingChanges(ctx, studentIDs, reason, reviewedBy, at)
		stats.Add(queryStats)
		rows = queryStats.Rows
		return commandErr
	})
	return rows, err
}

func (s *Service) run(operation string, command func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	return command(&stats)
}
