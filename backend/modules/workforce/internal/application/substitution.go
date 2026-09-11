package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) FindGroupSubstitution(ctx context.Context, id int64) (domain.GroupSubstitution, error) {
	return s.findGroupSubstitution(ctx, "find_group_substitution", id, false)
}

func (s *Service) LockGroupSubstitution(ctx context.Context, id int64) (domain.GroupSubstitution, error) {
	return s.findGroupSubstitution(ctx, "lock_group_substitution", id, true)
}

func (s *Service) findGroupSubstitution(ctx context.Context, operation string, id int64, lock bool) (result domain.GroupSubstitution, err error) {
	err = s.run(operation, func(stats *domain.OperationStats) error {
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

func (s *Service) ListGroupSubstitutions(ctx context.Context, filter domain.GroupSubstitutionFilter) (result []domain.GroupSubstitution, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_group_substitutions", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGroupSubstitutions(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateGroupSubstitution(ctx context.Context, value domain.GroupSubstitution) (result domain.GroupSubstitution, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.GroupSubstitution{}, validationErr
	}
	err = s.run("create_group_substitution", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if value.EndDate >= s.clock.Today() {
				ids := []int64{value.SubstituteStaffID}
				if value.RegularStaffID != nil {
					ids = append(ids, *value.RegularStaffID)
				}
				if err := s.lockAssignmentStaff(txCtx, ids); err != nil {
					return err
				}
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateGroupSubstitution(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateGroupSubstitution(ctx context.Context, value domain.GroupSubstitution) (result domain.GroupSubstitution, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.GroupSubstitution{}, validationErr
	}
	err = s.run("update_group_substitution", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if value.EndDate >= s.clock.Today() {
				ids := []int64{value.SubstituteStaffID}
				if value.RegularStaffID != nil {
					ids = append(ids, *value.RegularStaffID)
				}
				if err := s.lockAssignmentStaff(txCtx, ids); err != nil {
					return err
				}
			}
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateGroupSubstitution(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrGroupSubstitutionNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteGroupSubstitution(ctx context.Context, id int64) error {
	return s.run("delete_group_substitution", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteGroupSubstitution(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (result int64, err error) {
	if from == "" {
		return 0, &domain.InvalidGroupSubstitutionError{Reason: "from is required"}
	}
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, &domain.InvalidGroupSubstitutionError{Reason: validationErr.Error()}
	}
	err = s.run("delete_group_substitutions_for_staff", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteGroupSubstitutionsForStaff(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}
