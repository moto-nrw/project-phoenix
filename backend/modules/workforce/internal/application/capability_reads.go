package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) FindStaffShift(ctx context.Context, id int64) (workforce.StaffShift, error) {
	if id <= 0 {
		return workforce.StaffShift{}, invalidShift("staff shift ID is required")
	}
	value, err := s.ReadShift(ctx, id)
	var missing interface{ RepositoryNotFound() }
	if errors.As(err, &missing) {
		return workforce.StaffShift{}, workforce.ErrStaffShiftNotFound
	}
	return shiftReadToPublic(value), err
}

func (s *Service) ListStaffShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]workforce.StaffShift, error) {
	values, err := s.ReadShifts(ctx, filter)
	if values == nil {
		return nil, err
	}
	return shiftReadsToPublic(values), err
}

func (s *Service) ListStaffTargetOverrides(ctx context.Context, staffID int64) ([]workforce.StaffTargetOverride, error) {
	if staffID <= 0 {
		return nil, invalid("staff ID is required")
	}
	rows, _, err := s.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]workforce.StaffTargetOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, targetOverrideToPublic(row))
	}
	return result, nil
}

func (s *Service) StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (workforce.TargetOverrideDays, error) {
	if len(staffIDs) == 0 {
		return workforce.TargetOverrideDays{}, nil
	}
	_, days, err := s.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: staffIDs, From: from, To: to, Days: true})
	if err != nil {
		return nil, MapError(err)
	}
	return workforce.TargetOverrideDays(days), nil
}

func (s *Service) OpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	if staffID <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("staff ID is required")
	}
	value, err := s.openWorkSessionOn(ctx, staffID, date, false)
	return value, mapWorkSessionError(err)
}

func (s *Service) LockOpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	if staffID <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("staff ID is required")
	}
	value, err := s.openWorkSessionOn(ctx, staffID, date, true)
	return value, mapWorkSessionError(err)
}
