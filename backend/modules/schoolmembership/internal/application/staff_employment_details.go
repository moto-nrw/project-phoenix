package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// UpdateEmployment is the unit of work of an employment-only write: the
// membership is locked, its employment half read under that lock, changed
// and saved whole, and the membership row is touched, exactly as a full
// UpdateStaff with the unchanged fields would. An offboarded member is
// ErrStaffNotFound and nothing is written.
func (s *Service) UpdateEmployment(ctx context.Context, operation string, id int64, change func(*domain.StaffEmployment)) (result domain.Staff, err error) {
	err = s.runStaffWrite(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		current, found, queryStats, err := s.store.FindStaff(txCtx, id, "UPDATE", false)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStaffNotFound
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
		result = result.WithEmployment(employment)
		return nil
	})
	return result, err
}

// staffEmployment is the employment half a staff member carries.
func staffEmployment(staff domain.Staff) domain.StaffEmployment {
	return domain.StaffEmployment{
		MembershipID: staff.ID, StaffNotes: staff.StaffNotes, EmploymentType: staff.EmploymentType,
		WorkTimeModelID: staff.WorkTimeModelID, PersonnelNumber: staff.PersonnelNumber,
		RotationAnchorDate: staff.RotationAnchorDate, BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut,
	}
}

func (e Engine) UpdateEmploymentDetails(ctx context.Context, input schoolmembership.UpdateEmploymentDetails) (schoolmembership.Staff, error) {
	return e.updateEmployment(ctx, "update_employment_details", input.ID, func(employment *domain.StaffEmployment) {
		if input.SetEmploymentType {
			employment.EmploymentType = input.EmploymentType
		}
		if input.SetPersonnelNumber {
			employment.PersonnelNumber = input.PersonnelNumber
		}
	})
}

func (e Engine) BindSchedule(ctx context.Context, input schoolmembership.BindSchedule) (schoolmembership.Staff, error) {
	return e.updateEmployment(ctx, "bind_schedule", input.ID, func(employment *domain.StaffEmployment) {
		employment.WorkTimeModelID = input.WorkTimeModelID
		employment.RotationAnchorDate = input.RotationAnchorDate
	})
}

// updateEmployment runs one employment-only write and reports it in the
// public vocabulary.
func (e Engine) updateEmployment(ctx context.Context, operation string, id int64, change func(*domain.StaffEmployment)) (schoolmembership.Staff, error) {
	value, err := e.service.UpdateEmployment(ctx, operation, id, change)
	return staffToPublic(value), MapError(err)
}
