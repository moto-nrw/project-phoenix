package api

import (
	"context"
	"errors"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// The personnel-record administration (#3752) lives in Workforce. What it does
// not own stays where it is: the staff and person rows behind a record are
// School Membership's and People Directory's, and the audit trail belongs to
// the audit platform. The two ports below bind them over the retained
// repositories, so every read and write joins the caller's tenant transaction
// and reports its failure exactly as the repositories always did.

// staffAdminStaffRows are the retained staff rows the administration reads
// and locks. The employment-only writes go through School Membership.
type staffAdminStaffRows interface {
	FindWithPerson(context.Context, int64) (*userModels.Staff, error)
	FindByID(context.Context, any) (*userModels.Staff, error)
	FindByIDForUpdate(context.Context, int64) (*userModels.Staff, error)
}

// staffAdminPersonRows are the retained person rows behind a staff member.
type staffAdminPersonRows interface {
	FindByIDForUpdate(context.Context, int64) (*userModels.Person, error)
}

// staffAdminPersonDetails writes the names and the birthday of the person
// behind a staff member through the People Directory.
type staffAdminPersonDetails interface {
	SetPersonDetails(ctx context.Context, personID int64, details peopledirectory.PersonDetails) error
}

// staffAdminPersons reads the person behind a staff member from the retained
// rows and writes its details through the People Directory.
type staffAdminPersons struct {
	rows    staffAdminPersonRows
	details staffAdminPersonDetails
}

// staffDataAccessTrail is the Audit Platform's data access log.
type staffDataAccessTrail interface {
	Create(context.Context, *auditModels.DataAccessLog) error
}

// staffAdminOwners are the retained collaborators of the personnel-record
// administration: the staff and person rows and the audit trail.
type staffAdminOwners struct {
	Staff      staffAdminStaffRows
	Persons    staffAdminPersons
	Membership schoolmembership.Capability

	MasterDataAudit auditModels.StaffMasterDataChangeCreator
	PersonnelNumber auditModels.PersonnelNumberChangeCreator
	DataAccessLog   staffDataAccessTrail
}

// composeStaffAdmin composes the personnel-record administration over the
// retained staff, person and audit repositories. admin carries what the
// caller binds (the database, the observer, the logger and the clock); the
// subjects, the audit trail and the permission matcher are bound here.
func composeStaffAdmin(admin workforceCompose.StaffAdminDependencies, owners staffAdminOwners) (*workforceModule.StaffAdmin, error) {
	if owners.Staff == nil || owners.Persons.rows == nil || owners.Persons.details == nil || owners.Membership == nil ||
		owners.MasterDataAudit == nil || owners.PersonnelNumber == nil || owners.DataAccessLog == nil {
		return nil, errors.New("staff admin: staff, person, membership and audit repositories are required")
	}
	admin.Subjects = staffAdminSubjects{staff: owners.Staff, persons: owners.Persons, membership: owners.Membership}
	admin.Audit = staffAdminAudit{masterData: owners.MasterDataAudit, personnelNumber: owners.PersonnelNumber, accessLog: owners.DataAccessLog}
	admin.Allows = securityruntime.HasPermission
	return workforceCompose.NewStaffAdmin(admin)
}

type staffAdminSubjects struct {
	staff      staffAdminStaffRows
	persons    staffAdminPersons
	membership schoolmembership.Capability
}

func (s staffAdminSubjects) StaffWithPerson(ctx context.Context, staffID int64) (workforceModule.StaffProfile, error) {
	staff, err := s.staff.FindWithPerson(ctx, staffID)
	if err != nil {
		return workforceModule.StaffProfile{}, err
	}
	return *publicStaffProfile(staff), nil
}

func (s staffAdminSubjects) StaffExists(ctx context.Context, staffID int64) error {
	_, err := s.staff.FindByID(ctx, staffID)
	return err
}

func (s staffAdminSubjects) LockStaff(ctx context.Context, staffID int64, withPerson bool) (workforceModule.StaffProfile, error) {
	staff, err := s.staff.FindByIDForUpdate(ctx, staffID)
	if err != nil {
		return workforceModule.StaffProfile{}, err
	}
	if withPerson && staff.Person == nil {
		// FindByIDForUpdate does not preload; lock the person row too so
		// person-section diffs read the last committed values.
		person, err := s.persons.rows.FindByIDForUpdate(ctx, staff.PersonID)
		if err != nil {
			return workforceModule.StaffProfile{}, err
		}
		if person == nil {
			return workforceModule.StaffProfile{}, fmt.Errorf("staff %d has no person record", staffID)
		}
		staff.Person = person
	}
	return *publicStaffProfile(staff), nil
}

func (s staffAdminSubjects) UpdatePerson(ctx context.Context, personID int64, firstName, lastName string, birthday *string) error {
	details := peopledirectory.PersonDetails{FirstName: firstName, LastName: lastName}
	if birthday != nil {
		details.Birthday = *birthday
	}
	return s.persons.details.SetPersonDetails(ctx, personID, details)
}

func (s staffAdminSubjects) SetEmploymentType(ctx context.Context, staffID int64, value *string) error {
	_, err := s.membership.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{ID: staffID, SetEmploymentType: true, EmploymentType: value})
	return membershipError("update staff", err)
}

func (s staffAdminSubjects) SetPersonnelNumber(ctx context.Context, staffID int64, value *string) error {
	_, err := s.membership.UpdateEmploymentDetails(ctx, schoolmembership.UpdateEmploymentDetails{ID: staffID, SetPersonnelNumber: true, PersonnelNumber: value})
	if errors.Is(err, schoolmembership.ErrPersonnelNumberConflict) {
		return workforceModule.ErrPersonnelNumberTaken
	}
	return membershipError("update staff", err)
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

type staffAdminAudit struct {
	masterData      auditModels.StaffMasterDataChangeCreator
	personnelNumber auditModels.PersonnelNumberChangeCreator
	accessLog       staffDataAccessTrail
}

var staffAuditSections = map[string]string{
	workforceModule.StaffAuditSectionPerson:         auditModels.StammdatenSectionPerson,
	workforceModule.StaffAuditSectionKontakt:        auditModels.StammdatenSectionKontakt,
	workforceModule.StaffAuditSectionArbeitsvertrag: auditModels.StammdatenSectionArbeitsvertrag,
	workforceModule.StaffAuditSectionQualifikation:  auditModels.StammdatenSectionQualifikation,
	workforceModule.StaffAuditSectionBankSteuer:     auditModels.StammdatenSectionBankSteuer,
	workforceModule.StaffAuditSectionDokumente:      auditModels.StammdatenSectionDokumente,
}

var staffDataAccessResources = map[string]string{
	workforceModule.StaffDataAccessFinancialView:    auditModels.ResourceTypeStaffFinancialView,
	workforceModule.StaffDataAccessFinancialReveal:  auditModels.ResourceTypeStaffFinancialReveal,
	workforceModule.StaffDataAccessDocumentDownload: auditModels.ResourceTypeStaffDocumentDownload,
}

func (a staffAdminAudit) RecordMasterDataChange(ctx context.Context, change workforceModule.StaffMasterDataChange) error {
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

func (a staffAdminAudit) RecordPersonnelNumberChange(ctx context.Context, change workforceModule.PersonnelNumberChange) error {
	return a.personnelNumber.Create(ctx, &auditModels.PersonnelNumberChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
		Note:      change.Note,
	})
}

func (a staffAdminAudit) RecordDataAccess(ctx context.Context, access workforceModule.StaffDataAccess) error {
	resource, ok := staffDataAccessResources[access.Resource]
	if !ok {
		return fmt.Errorf("unknown data access resource %q", access.Resource)
	}
	metadata := map[string]interface{}{"staff_id": access.StaffID}
	if access.Resource == workforceModule.StaffDataAccessDocumentDownload {
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
