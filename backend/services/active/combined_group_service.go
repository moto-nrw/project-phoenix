package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Combined Group operations

func (s *service) GetCombinedGroup(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	group, err := s.CombinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetCombinedGroup", Err: ErrCombinedGroupNotFound}
	}
	return group, nil
}

func (s *service) CreateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrInvalidData}
	}

	group.SetTenantID(tenant.FromContext(ctx))
	if s.CombinedGroupRepo.Create(ctx, group) != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	if group == nil || group.ID == 0 || group.Validate() != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrInvalidData}
	}

	if s.CombinedGroupRepo.Update(ctx, group) != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) DeleteCombinedGroup(ctx context.Context, id int64) error {
	_, err := s.CombinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrCombinedGroupNotFound}
	}

	// Delete all group mappings
	mappings, err := s.GroupMappingRepo.FindByActiveCombinedGroupID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
	}

	for _, mapping := range mappings {
		if err := s.GroupMappingRepo.Delete(ctx, mapping.ID); err != nil {
			return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
		}
	}

	// Delete the combined group
	if err := s.CombinedGroupRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error) {
	groups, err := s.CombinedGroupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListCombinedGroups", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveCombinedGroups(ctx context.Context) ([]*active.CombinedGroup, error) {
	groups, err := s.CombinedGroupRepo.FindActive(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveCombinedGroups", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindCombinedGroupsByTimeRange", Err: ErrInvalidTimeRange}
	}

	groups, err := s.CombinedGroupRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindCombinedGroupsByTimeRange", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndCombinedGroup(ctx context.Context, id int64) error {
	// Verify group exists first
	_, err := s.CombinedGroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "EndCombinedGroup", Err: ErrCombinedGroupNotFound}
		}
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("failed to verify combined group: %w", err)}
	}

	if err := s.CombinedGroupRepo.EndCombination(ctx, id); err != nil {
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("end combination failed: %w", err)}
	}
	return nil
}

func (s *service) GetCombinedGroupWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	combinedGroup, err := s.CombinedGroupRepo.FindWithGroups(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetCombinedGroupWithGroups", Err: ErrCombinedGroupNotFound}
	}
	return combinedGroup, nil
}

func (s *service) CreateCombinedGroupWithGroups(ctx context.Context, group *active.CombinedGroup, groupIDs []int64) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: ErrInvalidData}
	}

	// No group IDs: delegate to simple creation
	if len(groupIDs) == 0 {
		return s.CreateCombinedGroup(ctx, group)
	}

	// Deduplicate upfront before touching the database
	seen := make(map[int64]bool, len(groupIDs))
	for _, gid := range groupIDs {
		if seen[gid] {
			return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: duplicate group ID %d", ErrInvalidData, gid)}
		}
		seen[gid] = true
	}

	// The repository calls below join the handler's WithTenantTx transaction
	// via the context; without it the multi-step create would not be atomic.
	if _, ok := base.TxFromContext(ctx); !ok {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("no transaction in context")}
	}

	// Step 1: Create the combined group
	group.SetTenantID(tenant.FromContext(ctx))
	if err := s.CombinedGroupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: %v", ErrDatabaseOperation, err)}
	}

	// Step 2: Verify all active group IDs exist
	existOptions := base.NewQueryOptions()
	existOptions.Filter = base.NewFilter().In("id", int64Args(groupIDs)...)
	existCount, err := s.GroupRepo.CountWithOptions(ctx, existOptions)
	if err != nil {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: %v", ErrDatabaseOperation, err)}
	}
	if existCount != len(groupIDs) {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: one or more group IDs do not exist (expected %d, found %d)", ErrInvalidData, len(groupIDs), existCount)}
	}

	// Step 3: Insert all group mappings
	for _, gid := range groupIDs {
		mapping := &active.GroupMapping{
			ActiveCombinedGroupID: group.ID,
			ActiveGroupID:         gid,
		}
		mapping.SetTenantID(tenant.FromContext(ctx))
		if err := s.GroupMappingRepo.Create(ctx, mapping); err != nil {
			return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: %v", ErrDatabaseOperation, err)}
		}
	}

	s.getLogger().Info("combined group created with groups",
		"combined_group_id", group.ID,
		"group_count", len(groupIDs),
	)

	return nil
}

// Group Mapping operations

func (s *service) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	// Check if the mapping already exists
	mappings, err := s.GroupMappingRepo.FindByActiveCombinedGroupID(ctx, combinedGroupID)
	if err != nil {
		return &ActiveError{Op: "AddGroupToCombination", Err: ErrDatabaseOperation}
	}

	for _, mapping := range mappings {
		if mapping.ActiveGroupID == activeGroupID {
			return &ActiveError{Op: "AddGroupToCombination", Err: ErrGroupAlreadyInCombination}
		}
	}

	// Create the mapping
	if s.GroupMappingRepo.AddGroupToCombination(ctx, combinedGroupID, activeGroupID) != nil {
		return &ActiveError{Op: "AddGroupToCombination", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	if s.GroupMappingRepo.RemoveGroupFromCombination(ctx, combinedGroupID, activeGroupID) != nil {
		return &ActiveError{Op: "RemoveGroupFromCombination", Err: ErrDatabaseOperation}
	}
	return nil
}

func (s *service) GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	mappings, err := s.GroupMappingRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupMappingsByActiveGroupID", Err: ErrDatabaseOperation}
	}
	return mappings, nil
}

func (s *service) GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	mappings, err := s.GroupMappingRepo.FindByActiveCombinedGroupID(ctx, combinedGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "GetGroupMappingsByCombinedGroupID", Err: ErrDatabaseOperation}
	}
	return mappings, nil
}

// int64Args widens an int64 slice for the variadic Filter.In helper.
func int64Args(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
