package api

import (
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// legacyStaffToMembership projects a legacy staff model onto the capability
// value the HTTP adapter renders.
func legacyStaffToMembership(staff *peopleValues.Staff) schoolmembership.Staff {
	if staff == nil {
		return schoolmembership.Staff{}
	}
	result := schoolmembership.Staff{
		ID: staff.ID, TenantID: staff.TenantID, CreatedAt: staff.CreatedAt, UpdatedAt: staff.UpdatedAt,
		PersonID: staff.PersonID, StaffNotes: staff.StaffNotes, EmploymentType: staff.EmploymentType,
		WorkTimeModelID: staff.WorkTimeModelID, PersonnelNumber: staff.PersonnelNumber,
		BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut, DeletedAt: staff.DeletedAt, HasExternalCaregiverRecord: staff.HasExternalCaregiverRecord,
	}
	if staff.RotationAnchorDate != nil {
		result.RotationAnchorDate = staff.RotationAnchorDate.String()
	}
	return result
}

func legacyTeacherToMembership(teacher *peopleValues.Teacher) *schoolmembership.Teacher {
	if teacher == nil {
		return nil
	}
	return &schoolmembership.Teacher{
		ID: teacher.ID, TenantID: teacher.TenantID, CreatedAt: teacher.CreatedAt, UpdatedAt: teacher.UpdatedAt,
		StaffID: teacher.StaffID, Specialization: teacher.Specialization, Role: teacher.Role,
		Qualifications: teacher.Qualifications, DeletedAt: teacher.DeletedAt,
	}
}
