package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// EducationStaff serves the class assignments' staff check from the retained
// staff repository.
type EducationStaff struct{ staff educationStaffRows }

// educationStaffRows are the staff reads the staff check and lock take.
type educationStaffRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.Staff, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.Staff, error)
}

// NewEducationStaff binds the staff check to the staff repository.
func NewEducationStaff(staff educationStaffRows) EducationStaff {
	return EducationStaff{staff: staff}
}

// StaffExists reports a missing staff member as (false, nil) and keeps every
// other failure.
func (s EducationStaff) StaffExists(ctx context.Context, id int64) (bool, error) {
	if _, err := s.staff.FindByID(ctx, id); err != nil {
		if peoplePorts.IsRepositoryNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LockStaff locks the staff member's row until the transaction ends.
func (s EducationStaff) LockStaff(ctx context.Context, id int64) error {
	_, err := s.staff.FindByIDForUpdate(ctx, id)
	return err
}

// EducationExternalCaregivers serves the running-supervision directory from
// the staff and guest repositories.
type EducationExternalCaregivers struct {
	staff interface {
		ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
	}
	guests interface {
		ActiveGuests(ctx context.Context) ([]*peopleValues.Guest, error)
	}
}

// NewEducationExternalCaregivers binds the external-caregiver directory to
// the staff and guest repositories.
func NewEducationExternalCaregivers(staff interface {
	ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
}, guests interface {
	ActiveGuests(ctx context.Context) ([]*peopleValues.Guest, error)
}) EducationExternalCaregivers {
	return EducationExternalCaregivers{staff: staff, guests: guests}
}

// ListExternalCaregivers returns the currently valid external caregivers
// (#3823) in the caregiver directory's order. Which staff members count is
// School Membership's rule; the root maps the retained rows onto it.
func (s EducationExternalCaregivers) ListExternalCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	guests, err := s.guests.ActiveGuests(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	activeGuestStaffIDs := make([]int64, 0, len(guests))
	for _, guest := range guests {
		if guest != nil {
			activeGuestStaffIDs = append(activeGuestStaffIDs, guest.StaffID)
		}
	}
	candidates := make([]schoolmembership.ExternalCaregiverCandidate, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		candidate := schoolmembership.ExternalCaregiverCandidate{StaffID: member.ID, IsGuest: member.IsGuest, HasPerson: member.Person != nil}
		if member.Person != nil {
			candidate.HasAccount = member.Person.AccountID != nil
			candidate.FirstName, candidate.LastName = member.Person.FirstName, member.Person.LastName
		}
		candidates = append(candidates, candidate)
	}
	externals := schoolmembership.ActiveExternalCaregivers(activeGuestStaffIDs, candidates)
	result := make([]*educationRepo.Caregiver, 0, len(externals))
	for _, external := range externals {
		result = append(result, &educationRepo.Caregiver{StaffID: external.StaffID, FullName: external.FullName})
	}
	return result, nil
}

// EducationCaregivers serves the substitution module's caregiver directory
// from the retained teacher repository.
type EducationCaregivers struct{ teachers caregiverRows }

// caregiverRows are the active caregivers the substitution module offers.
type caregiverRows interface {
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*peopleValues.ActiveCaregiver, error)
	ListActiveCaregivers(ctx context.Context) ([]*peopleValues.ActiveCaregiver, error)
}

// NewEducationCaregivers binds the caregiver directory to the teacher
// repository.
func NewEducationCaregivers(teachers caregiverRows) EducationCaregivers {
	return EducationCaregivers{teachers: teachers}
}

// FindActiveCaregiverByAccountID returns the active caregiver bound to the
// account, nil when the account is none.
func (c EducationCaregivers) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*educationRepo.Caregiver, error) {
	caregiver, err := c.teachers.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil || caregiver == nil {
		return nil, err
	}
	return educationCaregiver(caregiver), nil
}

// ListActiveCaregivers returns every active caregiver of the school, ordered
// by name.
func (c EducationCaregivers) ListActiveCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	caregivers, err := c.teachers.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*educationRepo.Caregiver, 0, len(caregivers))
	for _, caregiver := range caregivers {
		if caregiver != nil {
			result = append(result, educationCaregiver(caregiver))
		}
	}
	return result, nil
}

func educationCaregiver(caregiver *peopleValues.ActiveCaregiver) *educationRepo.Caregiver {
	return &educationRepo.Caregiver{StaffID: caregiver.StaffID, TeacherID: caregiver.TeacherID, FullName: caregiver.FullName()}
}
