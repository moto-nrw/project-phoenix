package services

import (
	"context"
	"errors"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The personnel-record administration (#3752) lives in Workforce. What it does
// not own stays where it is: the staff and person rows behind a record are
// School Membership's and People Directory's, and the audit trail belongs to
// the audit platform. The two ports below bind them over the retained
// repositories, so every read and write joins the caller's tenant transaction
// and reports its failure exactly as the repositories always did.

// StaffAdminOwners are the retained collaborators of the personnel-record
// administration: the staff and person rows and the audit trail.
type StaffAdminOwners struct {
	Staff      userModels.StaffRepository
	Persons    userModels.PersonRepository
	Membership schoolmembership.Capability

	MasterDataAudit auditModels.StaffMasterDataChangeCreator
	PersonnelNumber auditModels.PersonnelNumberChangeCreator
	DataAccessLog   auditModels.DataAccessLogRepository
}

// ComposeStaffAdmin composes the personnel-record administration over the
// retained staff, person and audit repositories. admin carries what the
// caller binds (the database, the observer, the logger and the clock); the
// subjects, the audit trail and the permission matcher are bound here.
func ComposeStaffAdmin(admin workforceCompose.StaffAdminDependencies, owners StaffAdminOwners) (*workforce.StaffAdmin, error) {
	if owners.Staff == nil || owners.Persons == nil || owners.Membership == nil ||
		owners.MasterDataAudit == nil || owners.PersonnelNumber == nil || owners.DataAccessLog == nil {
		return nil, errors.New("staff admin: staff, person, membership and audit repositories are required")
	}
	admin.Subjects = staffAdminSubjects{staff: owners.Staff, persons: owners.Persons, membership: owners.Membership}
	admin.Audit = staffAdminAudit{masterData: owners.MasterDataAudit, personnelNumber: owners.PersonnelNumber, accessLog: owners.DataAccessLog}
	admin.Allows = securityruntime.HasPermission
	return workforceCompose.NewStaffAdmin(admin)
}

type staffAdminSubjects struct {
	staff      userModels.StaffRepository
	persons    userModels.PersonRepository
	membership schoolmembership.Capability
}

func (s staffAdminSubjects) StaffWithPerson(ctx context.Context, staffID int64) (workforce.StaffProfile, error) {
	staff, err := s.staff.FindWithPerson(ctx, staffID)
	if err != nil {
		return workforce.StaffProfile{}, err
	}
	return staffProfileValue(staff), nil
}

func (s staffAdminSubjects) StaffExists(ctx context.Context, staffID int64) error {
	_, err := s.staff.FindByID(ctx, staffID)
	return err
}

func (s staffAdminSubjects) LockStaff(ctx context.Context, staffID int64, withPerson bool) (workforce.StaffProfile, error) {
	staff, err := s.staff.FindByIDForUpdate(ctx, staffID)
	if err != nil {
		return workforce.StaffProfile{}, err
	}
	if withPerson && staff.Person == nil {
		// FindByIDForUpdate does not preload; lock the person row too so
		// person-section diffs read the last committed values.
		person, err := s.persons.FindByIDForUpdate(ctx, staff.PersonID)
		if err != nil {
			return workforce.StaffProfile{}, err
		}
		if person == nil {
			return workforce.StaffProfile{}, fmt.Errorf("staff %d has no person record", staffID)
		}
		staff.Person = person
	}
	return staffProfileValue(staff), nil
}

func (s staffAdminSubjects) UpdatePerson(ctx context.Context, personID int64, firstName, lastName string, birthday *string) error {
	person, err := s.persons.FindByIDForUpdate(ctx, personID)
	if err != nil {
		return err
	}
	person.FirstName = firstName
	person.LastName = lastName
	person.Birthday = nil
	if birthday != nil {
		day, err := timezone.ParseDate(*birthday)
		if err != nil {
			return err
		}
		person.Birthday = &day
	}
	return s.persons.Update(ctx, person)
}

func (s staffAdminSubjects) SetEmploymentType(ctx context.Context, staffID int64, value *string) error {
	staff, err := s.staff.FindByIDForUpdate(ctx, staffID)
	if err != nil {
		return err
	}
	staff.EmploymentType = value
	return s.staff.Update(ctx, staff)
}

func (s staffAdminSubjects) SetPersonnelNumber(ctx context.Context, staffID int64, value *string) error {
	staff, err := s.staff.FindByIDForUpdate(ctx, staffID)
	if err != nil {
		return err
	}
	staff.PersonnelNumber = value
	if err := s.staff.Update(ctx, staff); err != nil {
		// School Membership classifies the per-tenant duplicate itself.
		if errors.Is(err, userModels.ErrPersonnelNumberConflict) {
			return workforce.ErrPersonnelNumberTaken
		}
		return err
	}
	return nil
}

func (s staffAdminSubjects) OffboardedStaffIDs(ctx context.Context) ([]int64, error) {
	members, err := s.membership.ListStaff(ctx, schoolmembership.StaffFilter{IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	offboarded := make([]int64, 0, len(members))
	for _, member := range members {
		if member.IsDeleted() {
			offboarded = append(offboarded, member.ID)
		}
	}
	return offboarded, nil
}

// staffProfileValue is the public staff profile of a retained row, with the
// person's fields when the row was read with it.
func staffProfileValue(staff *userModels.Staff) workforce.StaffProfile {
	return *publicStaffProfile(staff)
}

type staffAdminAudit struct {
	masterData      auditModels.StaffMasterDataChangeCreator
	personnelNumber auditModels.PersonnelNumberChangeCreator
	accessLog       auditModels.DataAccessLogRepository
}

var staffAuditSections = map[string]string{
	workforce.StaffAuditSectionPerson:         auditModels.StammdatenSectionPerson,
	workforce.StaffAuditSectionKontakt:        auditModels.StammdatenSectionKontakt,
	workforce.StaffAuditSectionArbeitsvertrag: auditModels.StammdatenSectionArbeitsvertrag,
	workforce.StaffAuditSectionQualifikation:  auditModels.StammdatenSectionQualifikation,
	workforce.StaffAuditSectionBankSteuer:     auditModels.StammdatenSectionBankSteuer,
	workforce.StaffAuditSectionDokumente:      auditModels.StammdatenSectionDokumente,
}

var staffDataAccessResources = map[string]string{
	workforce.StaffDataAccessFinancialView:    auditModels.ResourceTypeStaffFinancialView,
	workforce.StaffDataAccessFinancialReveal:  auditModels.ResourceTypeStaffFinancialReveal,
	workforce.StaffDataAccessDocumentDownload: auditModels.ResourceTypeStaffDocumentDownload,
}

func (a staffAdminAudit) RecordMasterDataChange(ctx context.Context, change workforce.StaffMasterDataChange) error {
	section, ok := staffAuditSections[change.Section]
	if !ok {
		return fmt.Errorf("unknown stammdaten audit section %q", change.Section)
	}
	return a.masterData.Create(ctx, &auditModels.StaffMasterDataChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		Section:   section,
		FieldName: change.Field,
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
		Note:      change.Note,
	})
}

func (a staffAdminAudit) RecordPersonnelNumberChange(ctx context.Context, change workforce.PersonnelNumberChange) error {
	return a.personnelNumber.Create(ctx, &auditModels.PersonnelNumberChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
		Note:      change.Note,
	})
}

func (a staffAdminAudit) RecordDataAccess(ctx context.Context, access workforce.StaffDataAccess) error {
	resource, ok := staffDataAccessResources[access.Resource]
	if !ok {
		return fmt.Errorf("unknown data access resource %q", access.Resource)
	}
	metadata := map[string]interface{}{"staff_id": access.StaffID}
	if access.Resource == workforce.StaffDataAccessDocumentDownload {
		metadata["document_id"] = access.DocumentID
		metadata["category"] = access.Category
	}
	return a.accessLog.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.ActorAccountID,
		ActorRole:      access.ActorRole,
		ResourceType:   resource,
		RangeStart:     access.At,
		RangeEnd:       access.At,
		AccessedAt:     access.At,
		Metadata:       metadata,
	})
}
