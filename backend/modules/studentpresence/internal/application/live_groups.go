package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListLiveGroups(ctx context.Context, ids []int64) ([]ports.LiveGroup, error) {
	if ids == nil {
		ids = []int64{}
	}
	return s.QueryLiveGroups(ctx, ports.LiveGroupFilter{IDs: ids})
}

func (s *Service) QueryLiveGroups(ctx context.Context, filter ports.LiveGroupFilter) (result []ports.LiveGroup, err error) {
	err = s.run("query_live_groups", func() (ports.Stats, error) {
		if filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid pagination")
		}
		for _, ids := range [][]int64{filter.IDs, filter.ActivityGroupIDs} {
			for _, id := range ids {
				if id <= 0 {
					return ports.Stats{}, errors.New("student presence: invalid group filter ID")
				}
			}
		}
		for _, id := range []*int64{filter.RoomID, filter.DeviceID} {
			if id != nil && *id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid group filter ID")
			}
		}
		if filter.From != nil && filter.Until != nil && filter.From.After(*filter.Until) {
			return ports.Stats{}, errors.New("student presence: invalid group time range")
		}
		var stats ports.Stats
		result, stats, err = s.store.QueryLiveGroups(ctx, filter)
		return stats, err
	})
	return result, err
}

// ListSupervisedLiveGroups returns the open sessions the staff member
// supervises on date. A supervision counts from its start date up to, not
// including, its end date.
func (s *Service) ListSupervisedLiveGroups(ctx context.Context, staffID int64, date ports.Date) (result []ports.LiveGroup, err error) {
	err = s.run("list_supervised_live_groups", func() (ports.Stats, error) {
		if staffID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid staff ID")
		}
		supervisions, stats, err := s.store.QueryGroupSupervisions(ctx, ports.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &date})
		if err != nil {
			return stats, err
		}
		groupIDs := make([]int64, 0, len(supervisions))
		for _, supervision := range supervisions {
			groupIDs = append(groupIDs, supervision.GroupID)
		}
		var sessionStats ports.Stats
		result, sessionStats, err = s.store.QueryLiveGroups(ctx, ports.LiveGroupFilter{IDs: groupIDs, OpenOnly: true})
		stats.Queries += sessionStats.Queries
		stats.Rows += sessionStats.Rows
		stats.StatementDuration += sessionStats.StatementDuration
		return stats, err
	})
	return result, err
}

func (s *Service) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) (result []int64, err error) {
	err = s.run("occupied_activity_group_ids", func() (ports.Stats, error) {
		for _, id := range ids {
			if id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid activity group ID")
			}
		}
		var stats ports.Stats
		result, stats, err = s.store.OccupiedActivityGroupIDs(ctx, ids)
		return stats, err
	})
	return result, err
}

func (s *Service) ListGroupSupervisions(ctx context.Context, groupID int64) ([]ports.GroupSupervision, error) {
	return s.QueryGroupSupervisions(ctx, ports.GroupSupervisionFilter{GroupIDs: []int64{groupID}})
}
func (s *Service) QueryGroupSupervisions(ctx context.Context, filter ports.GroupSupervisionFilter) (result []ports.GroupSupervision, err error) {
	err = s.run("query_group_supervisions", func() (ports.Stats, error) {
		if filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision pagination")
		}
		for _, id := range filter.StaffIDs {
			if id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid staff ID")
			}
		}
		for _, id := range filter.IDs {
			if id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid supervision ID")
			}
		}
		if filter.ForUpdate {
			if err := s.tx.Require(ctx); err != nil {
				return ports.Stats{}, err
			}
		}
		for _, id := range filter.GroupIDs {
			if id <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid group ID")
			}
		}
		if filter.StaffID != nil && *filter.StaffID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid staff ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.QueryGroupSupervisions(ctx, filter)
		return stats, err
	})
	return result, err
}

func (s *Service) StaffIDsWithSupervisionOn(ctx context.Context, date ports.Date) (result []int64, err error) {
	err = s.run("staff_ids_with_supervision", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.StaffIDsWithSupervisionOn(ctx, date)
		return stats, err
	})
	return result, err
}

func (s *Service) SupervisedRoomsOn(ctx context.Context, date ports.Date) (result []ports.StaffRoomSupervision, err error) {
	err = s.run("supervised_rooms_on", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.SupervisedRoomsOn(ctx, date)
		return stats, err
	})
	return result, err
}
