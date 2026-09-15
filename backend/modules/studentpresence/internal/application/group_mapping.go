package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) AddGroupToCombination(ctx context.Context, combinedID, groupID int64) error {
	return s.runWrite(ctx, "add_group_to_combination", func(txCtx context.Context) (ports.Stats, error) {
		if combinedID <= 0 || groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group mapping IDs")
		}
		return s.store.AddGroupToCombination(txCtx, combinedID, groupID)
	})
}
func (s *Service) RemoveGroupFromCombination(ctx context.Context, combinedID, groupID int64) error {
	return s.runWrite(ctx, "remove_group_from_combination", func(txCtx context.Context) (ports.Stats, error) {
		if combinedID <= 0 || groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group mapping IDs")
		}
		return s.store.RemoveGroupFromCombination(txCtx, combinedID, groupID)
	})
}

func (s *Service) ListGroupMappings(ctx context.Context, filter ports.GroupMappingFilter) (result []ports.GroupMapping, err error) {
	err = s.run("list_group_mappings", func() (ports.Stats, error) {
		if filter.CombinedGroupID != nil && *filter.CombinedGroupID <= 0 || filter.ActiveGroupID != nil && *filter.ActiveGroupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group mapping filter")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListGroupMappings(ctx, filter)
		return stats, err
	})
	return result, err
}

func (s *Service) RecordGroupMapping(ctx context.Context, combinedID, groupID int64) (result ports.GroupMapping, err error) {
	err = s.runWrite(ctx, "record_group_mapping", func(txCtx context.Context) (ports.Stats, error) {
		if combinedID <= 0 || groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group mapping IDs")
		}
		var stats ports.Stats
		result, stats, err = s.store.RecordGroupMapping(txCtx, combinedID, groupID)
		return stats, err
	})
	return result, err
}
func (s *Service) DeleteGroupMapping(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_group_mapping", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group mapping ID")
		}
		return s.store.DeleteGroupMapping(txCtx, id)
	})
}
