package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) RecordGroupActivity(ctx context.Context, id int64, at time.Time) error {
	return s.runWrite(ctx, "record_group_activity", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid group activity")
		}
		return s.store.RecordGroupActivity(txCtx, id, at)
	})
}

func (s *Service) DeleteGroup(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_group", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid group ID")
		}
		return s.store.DeleteGroup(txCtx, id)
	})
}

func (s *Service) ReviseGroup(ctx context.Context, group ports.LiveGroup) (result ports.LiveGroup, err error) {
	err = s.runWrite(ctx, "revise_group", func(txCtx context.Context) (ports.Stats, error) {
		if group.ID <= 0 || group.RoomID <= 0 || group.StartTime.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid group")
		}
		if group.ActivityGroupID != nil && *group.ActivityGroupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid activity group ID")
		}
		if group.EndTime != nil && group.StartTime.After(*group.EndTime) {
			return ports.Stats{}, errors.New("student presence: invalid group time range")
		}
		var stats ports.Stats
		result, stats, err = s.store.ReviseGroup(txCtx, group)
		return stats, err
	})
	return result, err
}

func (s *Service) RecordGroup(ctx context.Context, group ports.LiveGroup) (result ports.LiveGroup, err error) {
	err = s.runWrite(ctx, "record_group", func(txCtx context.Context) (ports.Stats, error) {
		if group.ID != 0 || group.RoomID <= 0 || group.StartTime.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid new group")
		}
		if group.ActivityGroupID != nil && *group.ActivityGroupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid activity group ID")
		}
		if group.EndTime != nil && group.StartTime.After(*group.EndTime) {
			return ports.Stats{}, errors.New("student presence: invalid group time range")
		}
		var stats ports.Stats
		result, stats, err = s.store.RecordGroup(txCtx, group)
		return stats, err
	})
	return result, err
}

func (s *Service) EndSupervisionOn(ctx context.Context, id int64, date ports.Date) (count int, err error) {
	err = s.runWrite(ctx, "id_end_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision id")
		}
		var stats ports.Stats
		count, stats, err = s.store.EndSupervisionOn(txCtx, id, date)
		return stats, err
	})
	return count, err
}

func (s *Service) EndStaffSupervisionsOn(ctx context.Context, id int64, date ports.Date) (count int, err error) {
	err = s.runWrite(ctx, "staff_id_end_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision staff_id")
		}
		var stats ports.Stats
		count, stats, err = s.store.EndStaffSupervisionsOn(txCtx, id, date)
		return stats, err
	})
	return count, err
}
