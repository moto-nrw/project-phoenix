package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) UpdateEmploymentDetails(ctx context.Context, input schoolmembership.UpdateEmploymentDetails) (schoolmembership.Staff, error) {
	if input.ID <= 0 {
		return schoolmembership.Staff{}, invalid("staff ID is required")
	}
	if input.SetEmploymentType {
		if err := validateEmploymentType(input.EmploymentType); err != nil {
			return schoolmembership.Staff{}, err
		}
	}
	if input.SetPersonnelNumber {
		input.PersonnelNumber = normalizePersonnelNumber(input.PersonnelNumber)
	}
	return s.updateEmployment(ctx, "update_employment_details", input.ID, func(employment *domain.StaffEmployment) {
		if input.SetEmploymentType {
			employment.EmploymentType = input.EmploymentType
		}
		if input.SetPersonnelNumber {
			employment.PersonnelNumber = input.PersonnelNumber
		}
	})
}

func (s *Service) BindSchedule(ctx context.Context, input schoolmembership.BindSchedule) (schoolmembership.Staff, error) {
	if input.ID <= 0 {
		return schoolmembership.Staff{}, invalid("staff ID is required")
	}
	if input.WorkTimeModelID != nil && *input.WorkTimeModelID <= 0 {
		return schoolmembership.Staff{}, invalid("work time model ID must be positive")
	}
	if err := validateDate(input.RotationAnchorDate, "rotation anchor date"); err != nil {
		return schoolmembership.Staff{}, err
	}
	return s.updateEmployment(ctx, "bind_schedule", input.ID, func(employment *domain.StaffEmployment) {
		employment.WorkTimeModelID = input.WorkTimeModelID
		employment.RotationAnchorDate = input.RotationAnchorDate
	})
}

// LockLiveStaff locks a live membership. Inside the caller's transaction the
// membership stays locked until it commits.
func (s *Service) LockLiveStaff(ctx context.Context, staffID int64) error {
	_, err := s.FindStaffForMutation(ctx, staffID)
	return err
}

// updateEmployment is the unit of work of an employment-only write: the
// membership is locked, its employment half read under that lock, changed
// and saved whole, and the membership row is touched, exactly as a full
// UpdateStaff with the unchanged fields would. An offboarded member is
// ErrStaffNotFound and nothing is written.
func (s *Service) updateEmployment(ctx context.Context, operation string, id int64, change func(*domain.StaffEmployment)) (result schoolmembership.Staff, err error) {
	err = s.runStaffWrite(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		current, found, queryStats, err := s.store.FindStaff(txCtx, id, "UPDATE", false)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return schoolmembership.ErrStaffNotFound
		}
		if current, err = s.withEmployment(txCtx, current, stats); err != nil {
			return err
		}
		employment := staffEmployment(current)
		change(&employment)
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateStaff(txCtx, id, current.PersonID)
		stats.Add(updateStats)
		if err != nil {
			return err
		}
		stats.Queries++
		if err := s.employment.SaveStaffEmployment(txCtx, employment); err != nil {
			return err
		}
		result = staffWithEmployment(result, employment)
		return nil
	})
	return result, err
}
