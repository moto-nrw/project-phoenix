package services

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// NewRepositorySchoolIdentityForTests returns a school identity double for
// unit tests that compose a retained flow over repository doubles and no
// database. It walks the person -> staff -> caregiver chain over the given
// repositories with the owner's role classification. Transponders, import
// hints and the child-record refusal are Identity & Access behaviour; the
// module's own tests cover them, and this double refuses a transponder
// rather than pretending to check it.
func NewRepositorySchoolIdentityForTests(
	persons userModels.PersonRepository,
	staff userModels.StaffRepository,
	teachers userModels.TeacherRepository,
) auth.SchoolIdentityProvisioning {
	return repositorySchoolIdentity{persons: persons, staff: staff, teachers: teachers}
}

type repositorySchoolIdentity struct {
	persons  userModels.PersonRepository
	staff    userModels.StaffRepository
	teachers userModels.TeacherRepository
}

func (r repositorySchoolIdentity) RoleNeedsStaffRecord(role *auth.RoleFacts) bool {
	return identityaccess.RoleNeedsStaffRecord(roleFacts(role))
}

func (r repositorySchoolIdentity) RoleNeedsCaregiverProfile(role *auth.RoleFacts) bool {
	return identityaccess.RoleNeedsCaregiverProfile(roleFacts(role))
}

func (r repositorySchoolIdentity) IsPlatformCaregiverRole(role *auth.RoleFacts) bool {
	return identityaccess.IsPlatformCaregiverRole(roleFacts(role))
}

func (r repositorySchoolIdentity) EnsureSchoolIdentity(ctx context.Context, in auth.SchoolIdentityInput) (*auth.SchoolIdentity, error) {
	if !r.RoleNeedsStaffRecord(in.Role) {
		return nil, nil
	}
	if in.TagID != nil && *in.TagID != "" {
		return nil, auth.ErrSchoolIdentityTagUnknown
	}
	person, err := r.persons.FindByAccountID(ctx, in.AccountID)
	if err != nil && !auth.IsRowMissing(err) {
		return nil, err
	}
	if person == nil || person.DeletedAt != nil {
		if !in.CreatePerson {
			return nil, nil
		}
		if in.FirstName == "" || in.LastName == "" {
			return nil, auth.ErrSchoolIdentityNamesRequired
		}
		person = &userModels.Person{FirstName: in.FirstName, LastName: in.LastName}
		person.SetTenantID(in.TenantID)
		if err := r.persons.Create(ctx, person); err != nil {
			return nil, fmt.Errorf("create person: %w", err)
		}
		if err := r.persons.LinkToAccount(ctx, person.ID, in.AccountID); err != nil {
			return nil, fmt.Errorf("link person to account: %w", err)
		}
	}
	staff, err := r.staff.FindByPersonID(ctx, person.ID)
	if err != nil && !auth.IsRowMissing(err) {
		return nil, err
	}
	if staff == nil || staff.DeletedAt != nil {
		staff = &userModels.Staff{PersonID: person.ID}
		staff.SetTenantID(in.TenantID)
		if err := r.staff.Create(ctx, staff); err != nil {
			return nil, fmt.Errorf("create staff: %w", err)
		}
	}
	identity := &auth.SchoolIdentity{PersonID: person.ID, StaffID: staff.ID}
	if !r.RoleNeedsCaregiverProfile(in.Role) && !in.CaregiverUpgrade {
		return identity, nil
	}
	teacher, err := r.teachers.FindByStaffID(ctx, staff.ID)
	if err != nil && !auth.IsRowMissing(err) {
		return nil, err
	}
	if teacher == nil || teacher.DeletedAt != nil {
		teacher = &userModels.Teacher{StaffID: staff.ID, Role: in.Position}
		teacher.SetTenantID(in.TenantID)
		if err := r.teachers.Create(ctx, teacher); err != nil {
			return nil, fmt.Errorf("create teacher: %w", err)
		}
	}
	identity.TeacherID = teacher.ID
	return identity, nil
}
