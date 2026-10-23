package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
// the staff and external caregiver record repositories and the school's open invitations.
type EducationExternalCaregivers struct {
	staff interface {
		ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
	}
	records interface {
		ActiveExternalCaregiverRecords(ctx context.Context) ([]*peopleValues.ExternalCaregiverRecord, error)
	}
	invitations pendingInvitations
}

// pendingInvitations lists the school's unused, unexpired invitations.
type pendingInvitations interface {
	ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error)
}

// NewEducationExternalCaregivers binds the external-caregiver directory to
// the staff and external caregiver record repositories. The open invitations come from the
// Identity & Access owner, which the root builds later (WithInvitations).
func NewEducationExternalCaregivers(staff interface {
	ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
}, records interface {
	ActiveExternalCaregiverRecords(ctx context.Context) ([]*peopleValues.ExternalCaregiverRecord, error)
}) EducationExternalCaregivers {
	return EducationExternalCaregivers{staff: staff, records: records}
}

// WithInvitations binds the school's open invitations, which decide whether
// an external caregiver without an account may be assigned (#4573).
func (s EducationExternalCaregivers) WithInvitations(invitations pendingInvitations) EducationExternalCaregivers {
	s.invitations = invitations
	return s
}

// ListExternalCaregivers returns the currently valid external caregivers
// (#3823), with or without an account (#4573), in the caregiver directory's
// order. Which staff members count and which may be assigned is School
// Membership's rule; the root maps the retained rows onto it.
func (s EducationExternalCaregivers) ListExternalCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	records, err := s.records.ActiveExternalCaregiverRecords(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	activeRecordStaffIDs := make([]int64, 0, len(records))
	organizations := make(map[int64]string, len(records))
	for _, record := range records {
		if record != nil {
			activeRecordStaffIDs = append(activeRecordStaffIDs, record.StaffID)
			organizations[record.StaffID] = record.Organization
		}
	}
	candidates := externalCaregiverCandidates(members)
	if err := s.markOpenInvitations(ctx, candidates, members); err != nil {
		return nil, err
	}
	externals := schoolmembership.ActiveExternalCaregivers(activeRecordStaffIDs, candidates)
	result := make([]*educationRepo.Caregiver, 0, len(externals))
	for _, external := range externals {
		result = append(result, &educationRepo.Caregiver{
			StaffID: external.StaffID, FullName: external.FullName,
			Assignable: external.Assignable, AccountStatus: external.AccountStatus,
			Organization: organizations[external.StaffID],
		})
	}
	return result, nil
}

func externalCaregiverCandidates(members []*peopleValues.Staff) []schoolmembership.ExternalCaregiverCandidate {
	candidates := make([]schoolmembership.ExternalCaregiverCandidate, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		candidate := schoolmembership.ExternalCaregiverCandidate{StaffID: member.ID, HasExternalCaregiverRecord: member.HasExternalCaregiverRecord, HasPerson: member.Person != nil}
		if member.Person != nil {
			candidate.HasAccount = member.Person.AccountID != nil
			candidate.FirstName, candidate.LastName = member.Person.FirstName, member.Person.LastName
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// markOpenInvitations marks the candidates an open invitation names. The
// invitations are read only when an external caregiver without an account is among them,
// so a school without such externals pays no extra query.
func (s EducationExternalCaregivers) markOpenInvitations(ctx context.Context, candidates []schoolmembership.ExternalCaregiverCandidate, members []*peopleValues.Staff) error {
	personIDs := make(map[int64]int64, len(candidates))
	for _, member := range members {
		if member != nil && member.IsExternalCaregiver && member.Person != nil && member.Person.AccountID == nil {
			personIDs[member.ID] = member.Person.ID
		}
	}
	if len(personIDs) == 0 {
		return nil
	}
	if s.invitations == nil {
		return errors.New("external caregiver directory: open invitations are not bound")
	}
	pending, err := s.invitations.ListPendingSchoolInvitations(ctx)
	if err != nil {
		return err
	}
	invited := identityaccess.PendingInvitationsByPerson(pending)
	for i := range candidates {
		if personID, ok := personIDs[candidates[i].StaffID]; ok {
			candidates[i].InvitationPending = len(invited[personID]) > 0
		}
	}
	return nil
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
