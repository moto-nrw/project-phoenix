package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func validateSupervision(row ports.GroupSupervision) error {
	if row.GroupID <= 0 || row.StaffID <= 0 || row.Role == "" || row.StartDate.IsZero() {
		return errors.New("student presence: invalid supervision")
	}
	if row.EndDate != nil && row.StartDate.After(*row.EndDate) {
		return errors.New("student presence: invalid supervision date range")
	}
	return nil
}

func (s *Service) RecordSupervision(ctx context.Context, row ports.GroupSupervision) (result ports.GroupSupervision, err error) {
	err = s.runWrite(ctx, "record_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if row.ID != 0 {
			return ports.Stats{}, errors.New("student presence: new supervision must not have an ID")
		}
		if err := validateSupervision(row); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.RecordSupervision(txCtx, row)
		return stats, err
	})
	return result, err
}

func (s *Service) ReviseSupervision(ctx context.Context, row ports.GroupSupervision) (result ports.GroupSupervision, err error) {
	err = s.runWrite(ctx, "revise_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if row.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision ID")
		}
		if err := validateSupervision(row); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.ReviseSupervision(txCtx, row)
		return stats, err
	})
	return result, err
}

func (s *Service) RemoveSupervision(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "remove_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision ID")
		}
		return s.store.RemoveSupervision(txCtx, id)
	})
}

func (s *Service) SetSupervisionEnd(ctx context.Context, id int64, date ports.Date, at time.Time) (count int64, err error) {
	err = s.runWrite(ctx, "set_supervision_end", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid supervision end")
		}
		var stats ports.Stats
		count, stats, err = s.store.SetSupervisionEnd(txCtx, id, date, at)
		return stats, err
	})
	return count, err
}

func (s *Service) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, date ports.Date) (count int, err error) {
	err = s.runWrite(ctx, "end_open_group_supervisions", func(txCtx context.Context) (ports.Stats, error) {
		if groupID <= 0 || staffID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision group or staff ID")
		}
		var stats ports.Stats
		count, stats, err = s.store.EndOpenGroupSupervisions(txCtx, groupID, staffID, date)
		return stats, err
	})
	return count, err
}
