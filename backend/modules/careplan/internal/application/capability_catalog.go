package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) FindCareOffering(ctx context.Context, id int64) (result careplan.CareOffering, err error) {
	if id <= 0 {
		return careplan.CareOffering{}, invalid(careplan.ErrInvalidCareOffering, "care offering ID is required")
	}
	err = s.run(ctx, "find_care_offering", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCareOffering(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return careplan.ErrCareOfferingNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) (result []careplan.CareOffering, err error) {
	if filter, err = normalizeCareOfferingFilter(filter); err != nil {
		return nil, err
	}
	err = s.run(ctx, "list_care_offerings", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListCareOfferings(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (result int, err error) {
	if phaseID <= 0 {
		return 0, invalid(careplan.ErrInvalidCareOffering, "phase ID is required")
	}
	err = s.run(ctx, "count_care_offerings_by_phase", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountCareOfferingsByPhase(ctx, phaseID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateCareOffering(ctx context.Context, input careplan.CreateCareOffering) (result careplan.CareOffering, err error) {
	fields, err := normalizeCareOfferingFields(input.CareOfferingFields)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	fields.AutoAddTriggerOfferingIDs = normalizedTriggers(0, fields.AutoAddTriggerOfferingIDs)
	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "create_care_offering", func(ctx context.Context, stats *domain.OperationStats) error {
			var queryStats domain.OperationStats
			var commandErr error
			result, queryStats, commandErr = s.store.CreateCareOffering(ctx, fields)
			stats.Add(queryStats)
			if commandErr != nil {
				return commandErr
			}
			return s.replaceStoredTriggers(ctx, stats, &result, fields.AutoAddTriggerOfferingIDs)
		})
	})
	return result, err
}

func (s *Service) UpdateCareOffering(ctx context.Context, input careplan.UpdateCareOffering) (result careplan.CareOffering, err error) {
	if input.ID <= 0 {
		return careplan.CareOffering{}, invalid(careplan.ErrInvalidCareOffering, "care offering ID is required")
	}
	fields, err := normalizeCareOfferingFields(input.CareOfferingFields)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	fields.AutoAddTriggerOfferingIDs = normalizedTriggers(input.ID, fields.AutoAddTriggerOfferingIDs)
	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "update_care_offering", func(ctx context.Context, stats *domain.OperationStats) error {
			var queryStats domain.OperationStats
			var commandErr error
			result, queryStats, commandErr = s.store.UpdateCareOffering(ctx, input.ID, fields)
			stats.Add(queryStats)
			if commandErr != nil {
				return commandErr
			}
			return s.replaceStoredTriggers(ctx, stats, &result, fields.AutoAddTriggerOfferingIDs)
		})
	})
	return result, err
}

// replaceStoredTriggers replaces the selection links of a just written
// offering in the same transaction and reports them on the result.
func (s *Service) replaceStoredTriggers(ctx context.Context, stats *domain.OperationStats, offering *careplan.CareOffering, triggers []int64) error {
	queryStats, err := s.store.ReplaceAutoAddTriggers(ctx, offering.ID, triggers)
	stats.Add(queryStats)
	if err == nil {
		offering.AutoAddTriggerOfferingIDs = append([]int64(nil), triggers...)
	}
	return err
}

func (s *Service) DeleteCareOffering(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid(careplan.ErrInvalidCareOffering, "care offering ID is required")
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "delete_care_offering", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.DeleteCareOffering(ctx, id)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) ReplaceAutoAddTriggers(ctx context.Context, targetID int64, triggerIDs []int64) error {
	if targetID <= 0 {
		return invalid(careplan.ErrInvalidCareOffering, "target care offering ID is required")
	}
	triggers := normalizedTriggers(targetID, triggerIDs)
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "replace_care_offering_auto_triggers", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.ReplaceAutoAddTriggers(ctx, targetID, triggers)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) FindOfferingChange(ctx context.Context, id int64, lock bool) (result careplan.OfferingChangeRequest, err error) {
	if id <= 0 {
		return careplan.OfferingChangeRequest{}, invalid(careplan.ErrInvalidOfferingChange, "offering change request ID is required")
	}
	err = s.run(ctx, "find_offering_change", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindOfferingChange(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return careplan.ErrOfferingChangeNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListOfferingChanges(ctx context.Context, filter careplan.OfferingChangeFilter) (result []careplan.OfferingChangeRequest, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)
	for _, status := range filter.Statuses {
		if !validOfferingChangeStatus(status) {
			return nil, invalid(careplan.ErrInvalidOfferingChange, "offering change status filter is invalid")
		}
	}
	if filter.UrgentOnly != nil {
		if err := validateDate(filter.UrgentDate, "urgent date", careplan.ErrInvalidOfferingChange); err != nil {
			return nil, err
		}
	}
	if filter.Limit < 0 {
		return nil, invalid(careplan.ErrInvalidOfferingChange, "limit must not be negative")
	}
	if filter.Order == "" {
		filter.Order = careplan.ChangeOrderCreated
	}
	switch filter.Order {
	case careplan.ChangeOrderReviewed, careplan.ChangeOrderCreated, careplan.ChangeOrderUpdated:
	default:
		return nil, invalid(careplan.ErrInvalidOfferingChange, "offering change order is invalid")
	}
	err = s.run(ctx, "list_offering_changes", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListOfferingChanges(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) CreateOfferingChange(ctx context.Context, row careplan.OfferingChangeRequest) (result careplan.OfferingChangeRequest, err error) {
	if row.StudentID <= 0 || row.RequestChildID <= 0 || row.SubmittedBy <= 0 {
		return careplan.OfferingChangeRequest{}, invalid(careplan.ErrInvalidOfferingChange, "student, request child, and submitter are required")
	}
	if row.Status != careplan.OfferingChangePending {
		return careplan.OfferingChangeRequest{}, invalid(careplan.ErrInvalidOfferingChange, "a new offering change request must be pending")
	}
	if !validRequiredJSON(row.Payload) {
		return careplan.OfferingChangeRequest{}, invalid(careplan.ErrInvalidOfferingChange, "payload must be valid non-null JSON")
	}
	if err := validateDate(row.EffectiveFrom, "effective_from", careplan.ErrInvalidOfferingChange); err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "create_offering_change", func(ctx context.Context, stats *domain.OperationStats) error {
			var queryStats domain.OperationStats
			var commandErr error
			result, queryStats, commandErr = s.store.CreateOfferingChange(ctx, row)
			stats.Add(queryStats)
			return commandErr
		})
	})
	return result, err
}

func (s *Service) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) error {
	if id <= 0 {
		return invalid(careplan.ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if err := validateDate(date, "effective_from", careplan.ErrInvalidOfferingChange); err != nil {
		return err
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "update_offering_change_effective_from", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.UpdateOfferingChangeEffectiveFrom(ctx, id, date)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	if id <= 0 {
		return invalid(careplan.ErrInvalidOfferingChange, "offering change request ID is required")
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "update_offering_change_withdrawal", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.UpdateApprovedCompleteWithdrawal(ctx, id, complete)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) UpdatePendingOfferingChange(ctx context.Context, input careplan.UpdatePendingOfferingChange) error {
	if input.ID <= 0 {
		return invalid(careplan.ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if err := validateDate(input.EffectiveFrom, "effective_from", careplan.ErrInvalidOfferingChange); err != nil {
		return err
	}
	if !validRequiredJSON(input.Payload) {
		return invalid(careplan.ErrInvalidOfferingChange, "payload must be valid non-null JSON")
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "update_pending_offering_change", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.UpdatePendingOfferingChange(ctx, input)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) DecideOfferingChange(ctx context.Context, input careplan.DecideOfferingChange) error {
	if input.ID <= 0 || !terminalOfferingChangeStatus(input.Status) {
		return invalid(careplan.ErrInvalidOfferingChange, "offering change request ID and terminal status are required")
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "decide_offering_change", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.DecideOfferingChange(ctx, input)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	if id <= 0 {
		return invalid(careplan.ErrInvalidOfferingChange, "offering change request ID is required")
	}
	if len(snapshot) > 0 && !json.Valid(snapshot) {
		return invalid(careplan.ErrInvalidOfferingChange, "decision snapshot must be valid JSON")
	}
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "update_offering_change_snapshot", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, err := s.store.UpdateOfferingChangeSnapshot(ctx, id, snapshot)
			stats.Add(queryStats)
			return err
		})
	})
}

func (s *Service) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (rows int64, err error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if strings.TrimSpace(reason) == "" || at.IsZero() {
		return 0, invalid(careplan.ErrInvalidOfferingChange, "care-end reason and timestamp are required")
	}
	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		return s.run(txCtx, "close_pending_offering_changes", func(ctx context.Context, stats *domain.OperationStats) error {
			queryStats, commandErr := s.store.ClosePendingOfferingChanges(ctx, studentIDs, reason, reviewedBy, at)
			stats.Add(queryStats)
			rows = queryStats.Rows
			return commandErr
		})
	})
	return rows, err
}
