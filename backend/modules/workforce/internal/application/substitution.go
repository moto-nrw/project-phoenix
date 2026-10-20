package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) FindGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	if id <= 0 {
		return workforce.GroupSubstitution{}, invalidSubstitution("group substitution ID is required")
	}
	value, err := s.findGroupSubstitution(ctx, "find_group_substitution", id, false)
	return substitutionToPublic(value), MapError(err)
}

func (s *Service) LockGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	if id <= 0 {
		return workforce.GroupSubstitution{}, invalidSubstitution("group substitution ID is required")
	}
	value, err := s.findGroupSubstitution(ctx, "lock_group_substitution", id, true)
	return substitutionToPublic(value), MapError(err)
}

func (s *Service) findGroupSubstitution(ctx context.Context, operation string, id int64, lock bool) (result domain.GroupSubstitution, err error) {
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindGroupSubstitution(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrGroupSubstitutionNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListGroupSubstitutions(ctx context.Context, filter workforce.GroupSubstitutionFilter) ([]workforce.GroupSubstitution, error) {
	storedFilter := domain.GroupSubstitutionFilter{
		TenantID: filter.TenantID, GroupID: filter.GroupID, GroupIDs: filter.GroupIDs, SubstituteStaffID: filter.SubstituteStaffID,
		RegularStaffID: filter.RegularStaffID, StaffID: filter.StaffID, TargetType: filter.TargetType, On: filter.On,
		OverlapFrom: filter.OverlapFrom, OverlapTo: filter.OverlapTo, EndsOnOrAfter: filter.EndsOnOrAfter,
		ReasonContains: filter.ReasonContains, Limit: filter.Limit, Offset: filter.Offset,
	}
	var result []domain.GroupSubstitution
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, MapError(validationErr)
	}
	err = s.run(ctx, "list_group_substitutions", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGroupSubstitutions(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, MapError(storedErr)
	}
	storedResult := make([]workforce.GroupSubstitution, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, substitutionToPublic(value))
	}
	return storedResult, MapError(storedErr)
}

func (s *Service) CreateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	storedValue := substitutionToDomain(value)
	var result domain.GroupSubstitution
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return substitutionToPublic(domain.GroupSubstitution{}), MapError(validationErr)
	}
	err = s.run(ctx, "create_group_substitution", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if err := s.lockSubstitutionStaff(txCtx, storedValue); err != nil {
				return err
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateGroupSubstitution(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return substitutionToPublic(result), MapError(err)
}

func (s *Service) UpdateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	if value.ID <= 0 {
		return workforce.GroupSubstitution{}, invalidSubstitution("group substitution ID is required")
	}
	storedValue := substitutionToDomain(value)
	var result domain.GroupSubstitution
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return substitutionToPublic(domain.GroupSubstitution{}), MapError(validationErr)
	}
	err = s.run(ctx, "update_group_substitution", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if err := s.lockSubstitutionStaff(txCtx, storedValue); err != nil {
				return err
			}
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateGroupSubstitution(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrGroupSubstitutionNotFound
			}
			return err
		})
	})
	return substitutionToPublic(result), MapError(err)
}

// lockSubstitutionStaff serializes a substitution write with the assignment
// writes of its substitute and regular staff while it still runs today or
// later.
func (s *Service) lockSubstitutionStaff(ctx context.Context, value domain.GroupSubstitution) error {
	if value.EndDate < s.clock.Today() {
		return nil
	}
	ids := []int64{value.SubstituteStaffID}
	if value.RegularStaffID != nil {
		ids = append(ids, *value.RegularStaffID)
	}
	return s.lockAssignmentStaff(ctx, ids)
}

func (s *Service) DeleteGroupSubstitution(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidSubstitution("group substitution ID is required")
	}
	return MapError(s.run(ctx, "delete_group_substitution", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteGroupSubstitution(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) LockGroupSubstitutions(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("workforce: group substitution locks require a transaction")
	}
	return MapError(s.run(ctx, "lock_group_substitutions", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockGroupSubstitutions(ctx)
		stats.Add(queryStats)
		return err
	}))
}

func (s *Service) DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidSubstitution("staff ID is required")
	}
	var result int64
	var err error
	if from == "" {
		return 0, MapError(&domain.InvalidGroupSubstitutionError{Reason: "from is required"})
	}
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, MapError(&domain.InvalidGroupSubstitutionError{Reason: validationErr.Error()})
	}
	err = s.run(ctx, "delete_group_substitutions_for_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteGroupSubstitutionsForStaff(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, MapError(err)
}
