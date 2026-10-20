package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FindStaffIDByPerson returns the live staff ID of a person from the
// membership row alone, without the Workforce employment half (#2753).
func (s *Service) FindStaffIDByPerson(ctx context.Context, personID int64) (int64, bool, error) {
	if personID <= 0 {
		return 0, false, invalid("person ID is required")
	}
	staff, err := s.findStaffMembershipByPerson(ctx, personID)
	if errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return staff.ID, true, nil
}

// ResolveStaffIdentityByAccount runs the whole Account → Person → Staff →
// Teacher chain in one read of the context's tenant. It never takes the
// admin fallback: the identity read answers for one school, so a missing
// tenant is an error.
func (s *Service) ResolveStaffIdentityByAccount(ctx context.Context, accountID int64, persons schoolmembership.AccountPersons) (schoolmembership.StaffIdentity, error) {
	if accountID <= 0 {
		return schoolmembership.StaffIdentity{}, invalid("account ID is required")
	}
	if persons == nil {
		return schoolmembership.StaffIdentity{}, invalid("a person directory is required")
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return schoolmembership.StaffIdentity{}, fmt.Errorf("school membership: staff identity: %w", err)
	}
	var identity schoolmembership.StaffIdentity
	err := s.runRead(ctx, "resolve_staff_identity", func(ctx context.Context, _ *domain.OperationStats) error {
		personID, found, err := persons.FindPersonIDByAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if !found {
			identity = schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNoPerson}
			return nil
		}
		identity, err = s.resolveStaffIdentity(ctx, personID)
		return err
	})
	if err != nil {
		return schoolmembership.StaffIdentity{}, err
	}
	return identity, nil
}

func (s *Service) resolveStaffIdentity(ctx context.Context, personID int64) (schoolmembership.StaffIdentity, error) {
	if personID <= 0 {
		return schoolmembership.StaffIdentity{}, invalid("person ID is required")
	}
	staff, err := s.findStaffMembershipByPerson(ctx, personID)
	if errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNotStaff, PersonID: personID}, nil
	}
	if err != nil {
		return schoolmembership.StaffIdentity{}, err
	}
	teacher, err := s.FindTeacherByStaff(ctx, staff.ID)
	if errors.Is(err, schoolmembership.ErrTeacherNotFound) {
		return schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkStaff, PersonID: personID, StaffID: staff.ID}, nil
	}
	if err != nil {
		return schoolmembership.StaffIdentity{}, err
	}
	return schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkTeacher, PersonID: personID, StaffID: staff.ID, TeacherID: teacher.ID}, nil
}
