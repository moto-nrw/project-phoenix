package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// The supervision record keeps its calendar dates typed; the public value
// carries them as YYYY-MM-DD.
func supervisionToPublic(row ports.GroupSupervision) studentpresence.GroupSupervision {
	result := studentpresence.GroupSupervision{
		ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
		Role: row.Role, StartDate: row.StartDate.String(), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.EndDate != nil {
		date := row.EndDate.String()
		result.EndDate = &date
	}
	return result
}

func supervisionsToPublic(rows []ports.GroupSupervision) []studentpresence.GroupSupervision {
	result := make([]studentpresence.GroupSupervision, 0, len(rows))
	for _, row := range rows {
		result = append(result, supervisionToPublic(row))
	}
	return result
}

func SupervisionToRecord(row studentpresence.GroupSupervision) (ports.GroupSupervision, error) {
	start, err := calendar.ParseDate(row.StartDate)
	if err != nil {
		return ports.GroupSupervision{}, err
	}
	result := ports.GroupSupervision{
		ID: row.ID, GroupID: row.GroupID, StaffID: row.StaffID,
		Role: row.Role, StartDate: start, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.EndDate != nil {
		date, err := calendar.ParseDate(*row.EndDate)
		if err != nil {
			return ports.GroupSupervision{}, err
		}
		result.EndDate = &date
	}
	return result, nil
}

func (s *Service) RecordSupervision(ctx context.Context, value studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	row, err := SupervisionToRecord(value)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	var result ports.GroupSupervision
	err = s.runWrite(ctx, "record_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if row.ID != 0 {
			return ports.Stats{}, errors.New("student presence: new supervision must not have an ID")
		}
		if err := validateSupervision(row); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		var err error
		result, stats, err = s.store.RecordSupervision(txCtx, row)
		return stats, err
	})
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return supervisionToPublic(result), nil
}

func (s *Service) ReviseSupervision(ctx context.Context, value studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	row, err := SupervisionToRecord(value)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	var result ports.GroupSupervision
	err = s.runWrite(ctx, "revise_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if row.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision ID")
		}
		if err := validateSupervision(row); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		var err error
		result, stats, err = s.store.ReviseSupervision(txCtx, row)
		return stats, err
	})
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return supervisionToPublic(result), nil
}

func (s *Service) RemoveSupervision(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "remove_supervision", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid supervision ID")
		}
		return s.store.RemoveSupervision(txCtx, id)
	})
}

func (s *Service) SetSupervisionEnd(ctx context.Context, id int64, endDate string, at time.Time) (count int64, err error) {
	date, err := calendar.ParseDate(endDate)
	if err != nil {
		return 0, err
	}
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

// EndOpenGroupSupervisions ends open assignments that started by eligibleOn, using the database's current date.
func (s *Service) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, eligibleOn string) (count int, err error) {
	date, err := calendar.ParseDate(eligibleOn)
	if err != nil {
		return 0, err
	}
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
