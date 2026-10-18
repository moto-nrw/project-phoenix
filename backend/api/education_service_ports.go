package api

import (
	"context"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
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
	FindByID(ctx context.Context, id any) (*userModels.Staff, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Staff, error)
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
		ListAllWithPerson(ctx context.Context) ([]*userModels.Staff, error)
	}
	guests interface {
		List(ctx context.Context, filters map[string]any) ([]*userModels.Guest, error)
	}
}

// NewEducationExternalCaregivers binds the external-caregiver directory to
// the staff and guest repositories.
func NewEducationExternalCaregivers(staff interface {
	ListAllWithPerson(ctx context.Context) ([]*userModels.Staff, error)
}, guests interface {
	List(ctx context.Context, filters map[string]any) ([]*userModels.Guest, error)
}) EducationExternalCaregivers {
	return EducationExternalCaregivers{staff: staff, guests: guests}
}

// ListExternalCaregivers returns the currently valid external caregivers
// (#3823): staff members recorded with an active guest profile and without a
// moto account, ordered like the caregiver directory. They may help with a
// running supervision but never take over a group, so the substitution module
// keeps them out of the handover targets. Other staff without an account (an
// import without an e-mail address) stay out: nobody vouched for them as
// caregivers.
func (s EducationExternalCaregivers) ListExternalCaregivers(ctx context.Context) ([]*educationRepo.Caregiver, error) {
	guests, err := s.guests.List(ctx, map[string]any{"active": true})
	if err != nil {
		return nil, err
	}
	members, err := s.staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	externals := externalCaregiverStaff(guests, members)
	// Same German dictionary order as the caregiver directory, so both
	// halves of the picker sort alike; one collator per call.
	collator := collate.New(language.German, collate.IgnoreCase)
	sort.SliceStable(externals, func(i, j int) bool {
		a, b := externals[i].Person, externals[j].Person
		if c := collator.CompareString(a.FirstName, b.FirstName); c != 0 {
			return c < 0
		}
		if c := collator.CompareString(a.LastName, b.LastName); c != 0 {
			return c < 0
		}
		return externals[i].ID < externals[j].ID
	})
	result := make([]*educationRepo.Caregiver, 0, len(externals))
	for _, member := range externals {
		result = append(result, &educationRepo.Caregiver{StaffID: member.ID, FullName: member.Person.FirstName + " " + member.Person.LastName})
	}
	return result, nil
}

// externalCaregiverStaff keeps the guest staff members without a moto account
// whose guest profile is active.
func externalCaregiverStaff(guests []*userModels.Guest, members []*userModels.Staff) []*userModels.Staff {
	activeStaffIDs := make(map[int64]struct{}, len(guests))
	for _, guest := range guests {
		if guest != nil {
			activeStaffIDs[guest.StaffID] = struct{}{}
		}
	}
	externals := make([]*userModels.Staff, 0, len(members))
	for _, member := range members {
		if member == nil || !member.IsGuest || member.Person == nil || member.Person.AccountID != nil {
			continue
		}
		if _, active := activeStaffIDs[member.ID]; active {
			externals = append(externals, member)
		}
	}
	return externals
}

// EducationCaregivers serves the substitution module's caregiver directory
// from the retained teacher repository.
type EducationCaregivers struct{ teachers caregiverRows }

// caregiverRows are the active caregivers the substitution module offers.
type caregiverRows interface {
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error)
	ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error)
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

func educationCaregiver(caregiver *userModels.ActiveCaregiver) *educationRepo.Caregiver {
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
