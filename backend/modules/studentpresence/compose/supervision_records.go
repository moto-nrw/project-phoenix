package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

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

func supervisionToRecord(row studentpresence.GroupSupervision) (ports.GroupSupervision, error) {
	start, err := timezone.ParseDate(row.StartDate)
	if err != nil {
		return ports.GroupSupervision{}, err
	}
	result := ports.GroupSupervision{
		ID: row.ID, GroupID: row.GroupID, StaffID: row.StaffID,
		Role: row.Role, StartDate: start, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.EndDate != nil {
		date, err := timezone.ParseDate(*row.EndDate)
		if err != nil {
			return ports.GroupSupervision{}, err
		}
		result.EndDate = &date
	}
	return result, nil
}

func (e engine) RecordSupervision(ctx context.Context, value studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	row, err := supervisionToRecord(value)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	row, err = e.Service.RecordSupervision(ctx, row)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return supervisionToPublic(row), nil
}

func (e engine) ReviseSupervision(ctx context.Context, value studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	row, err := supervisionToRecord(value)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	row, err = e.Service.ReviseSupervision(ctx, row)
	if errors.Is(err, ports.ErrSupervisionNotFound) {
		return studentpresence.GroupSupervision{}, studentpresence.ErrSupervisionNotFound
	}
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return supervisionToPublic(row), nil
}

func (e engine) RemoveSupervision(ctx context.Context, id int64) error {
	return e.Service.RemoveSupervision(ctx, id)
}

func (e engine) SetSupervisionEnd(ctx context.Context, id int64, endDate string, at time.Time) (int64, error) {
	date, err := timezone.ParseDate(endDate)
	if err != nil {
		return 0, err
	}
	return e.Service.SetSupervisionEnd(ctx, id, date, at)
}

func (e engine) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, eligibleOn string) (int, error) {
	date, err := timezone.ParseDate(eligibleOn)
	if err != nil {
		return 0, err
	}
	return e.Service.EndOpenGroupSupervisions(ctx, groupID, staffID, date)
}
