package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Combined Group operations

func (s *service) CreateCombinedGroup(ctx context.Context, group *studentpresence.CombinedGroup) error {
	if !validPresenceCombination(group) {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrInvalidData}
	}

	group.TenantID = tenant.FromContext(ctx)
	row, err := s.SchoolPresence.RecordCombination(ctx, group.StartTime, group.EndTime)
	if err != nil {
		return &ActiveError{Op: "CreateCombinedGroup", Err: ErrDatabaseOperation}
	}

	*group = row
	return nil
}

func (s *service) UpdateCombinedGroup(ctx context.Context, group *studentpresence.CombinedGroup) error {
	if !validPresenceCombination(group) || group.ID == 0 {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrInvalidData}
	}

	row, err := s.SchoolPresence.ReviseCombination(ctx, group.ID, group.StartTime, group.EndTime)
	if err != nil {
		return &ActiveError{Op: "UpdateCombinedGroup", Err: ErrDatabaseOperation}
	}

	*group = row
	return nil
}

func (s *service) DeleteCombinedGroup(ctx context.Context, id int64) error {
	_, err := s.SchoolPresence.GetCombinedGroup(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrCombinedGroupNotFound}
	}

	// Delete all group mappings
	mappings, err := s.SchoolPresence.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &id})
	if err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
	}

	for _, mapping := range mappings {
		if err := s.SchoolPresence.DeleteGroupMapping(ctx, mapping.ID); err != nil {
			return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
		}
	}

	// Delete the combined group
	if err := s.SchoolPresence.DeleteCombination(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteCombinedGroup", Err: ErrDatabaseOperation}
	}

	return nil
}

func (s *service) EndCombinedGroup(ctx context.Context, id int64) error {
	// Verify group exists first
	_, err := s.SchoolPresence.GetCombinedGroup(ctx, id)
	if err != nil {
		if errors.Is(err, studentpresence.ErrCombinedGroupNotFound) {
			return &ActiveError{Op: "EndCombinedGroup", Err: ErrCombinedGroupNotFound}
		}
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("failed to verify combined group: database error during find by id: %w", err)}
	}

	if err := s.SchoolPresence.EndCombination(ctx, id, time.Now()); err != nil {
		return &ActiveError{Op: "EndCombinedGroup", Err: fmt.Errorf("end combination failed: %w", err)}
	}
	return nil
}

func (s *service) CreateCombinedGroupWithGroups(ctx context.Context, group *studentpresence.CombinedGroup, groupIDs []int64) error {
	if !validPresenceCombination(group) {
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
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("no transaction in context")}
	}

	// Step 1: Create the combined group
	group.TenantID = tenant.FromContext(ctx)
	row, err := s.SchoolPresence.RecordCombination(ctx, group.StartTime, group.EndTime)
	if err != nil {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: database error during create: %v", ErrDatabaseOperation, err)}
	}
	*group = row

	// Step 2: Verify all active group IDs exist
	existing, err := s.SchoolPresence.ListLiveGroups(ctx, groupIDs)
	if err != nil {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: %v", ErrDatabaseOperation, err)}
	}
	existCount := len(existing)
	if existCount != len(groupIDs) {
		return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: one or more group IDs do not exist (expected %d, found %d)", ErrInvalidData, len(groupIDs), existCount)}
	}

	// Step 3: Insert all group mappings
	for _, gid := range groupIDs {
		if _, err := s.SchoolPresence.RecordGroupMapping(ctx, group.ID, gid); err != nil {
			return &ActiveError{Op: "CreateCombinedGroupWithGroups", Err: fmt.Errorf("%w: %v", ErrDatabaseOperation, err)}
		}
	}

	s.getLogger().Info("combined group created with groups",
		"combined_group_id", group.ID,
		"group_count", len(groupIDs),
	)

	return nil
}

func validPresenceCombination(group *studentpresence.CombinedGroup) bool {
	return group != nil && !group.StartTime.IsZero() && (group.EndTime == nil || !group.StartTime.After(*group.EndTime))
}
