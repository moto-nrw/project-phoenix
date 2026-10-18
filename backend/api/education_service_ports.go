package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// The ports of the retained School Structure services the root's service
// graph binds over the retained repositories (#2742). Each adapter translates the
// People Directory, Facilities, Workforce and Audit Platform rows into the
// School Structure values the services own (#3556), and changes nothing about
// which reads and writes run.

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

// The substitution module may not name the Audit Platform, so School Structure
// mirrors the actions the trail stores, and the binding below passes them
// through unchanged. A drift must not compile: a false comparison repeats the
// false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	auditModels.SubstitutionAssigned == educationRepo.SubstitutionAssigned &&
		auditModels.SubstitutionEnded == educationRepo.SubstitutionEnded: {},
}

// EducationSubstitutionAudit appends the substitution module's changes to the
// audit.substitution_changes trail.
type EducationSubstitutionAudit struct {
	changes auditModels.SubstitutionChangeCreator
}

// NewEducationSubstitutionAudit binds the trail to its repository.
func NewEducationSubstitutionAudit(changes auditModels.SubstitutionChangeCreator) EducationSubstitutionAudit {
	return EducationSubstitutionAudit{changes: changes}
}

// RecordSubstitutionChange appends one change.
func (a EducationSubstitutionAudit) RecordSubstitutionChange(ctx context.Context, change educationRepo.SubstitutionChange) error {
	row := &auditModels.SubstitutionChange{
		SubstitutionID: change.SubstitutionID, TargetType: change.TargetType, Action: string(change.Action),
		GroupID: change.GroupID, TargetStaffID: change.TargetStaffID, ActorAccountID: change.ActorAccountID,
		StartDate: auditModels.Date(change.StartDate),
	}
	if change.EndDate != nil {
		endDate := auditModels.Date(*change.EndDate)
		row.EndDate = &endDate
	}
	return a.changes.Create(ctx, row)
}

// EducationClassAssignmentAudit appends class assignment rewrites (#1772) to
// the Stammdaten audit trail.
type EducationClassAssignmentAudit struct {
	changes auditModels.StaffMasterDataChangeCreator
}

// NewEducationClassAssignmentAudit binds the trail to its repository.
func NewEducationClassAssignmentAudit(changes auditModels.StaffMasterDataChangeCreator) EducationClassAssignmentAudit {
	return EducationClassAssignmentAudit{changes: changes}
}

// RecordSchoolClassChange appends one rewrite in the school classes section.
func (a EducationClassAssignmentAudit) RecordSchoolClassChange(ctx context.Context, change educationRepo.SchoolClassChange) error {
	return a.changes.Create(ctx, &auditModels.StaffMasterDataChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		Section:   auditModels.StammdatenSectionSchoolClasses,
		FieldName: auditModels.StammdatenFieldSchoolClasses,
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
	})
}
