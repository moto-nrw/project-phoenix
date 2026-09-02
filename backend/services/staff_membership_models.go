package services

import (
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// legacyStaffToMembership projects a legacy staff model onto the capability
// value the HTTP adapter renders.
func legacyStaffToMembership(staff *userModels.Staff) schoolmembership.Staff {
	if staff == nil {
		return schoolmembership.Staff{}
	}
	result := schoolmembership.Staff{
		ID: staff.ID, TenantID: staff.TenantID, CreatedAt: staff.CreatedAt, UpdatedAt: staff.UpdatedAt,
		PersonID: staff.PersonID, StaffNotes: staff.StaffNotes, EmploymentType: staff.EmploymentType,
		WorkTimeModelID: staff.WorkTimeModelID, PersonnelNumber: staff.PersonnelNumber,
		BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut, DeletedAt: staff.DeletedAt,
	}
	if staff.RotationAnchorDate != nil {
		result.RotationAnchorDate = staff.RotationAnchorDate.String()
	}
	return result
}

func legacyTeacherToMembership(teacher *userModels.Teacher) *schoolmembership.Teacher {
	if teacher == nil {
		return nil
	}
	return &schoolmembership.Teacher{
		ID: teacher.ID, TenantID: teacher.TenantID, CreatedAt: teacher.CreatedAt, UpdatedAt: teacher.UpdatedAt,
		StaffID: teacher.StaffID, Specialization: teacher.Specialization, Role: teacher.Role,
		Qualifications: teacher.Qualifications, DeletedAt: teacher.DeletedAt,
	}
}
