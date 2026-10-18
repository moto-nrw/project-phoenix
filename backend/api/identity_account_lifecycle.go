package api

import (
	"context"
	"errors"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// Identity & Access owns the admin staff-view preview, staff offboarding
// access, the school identity chain, parent accounts and guardian relative
// access (#3225). This file binds the seams those flows need to the owners:
// the identity chain's persons and invited guardian contacts are created
// through the People Directory, as are the guardian profile and relationship
// writes; staff and caregiver profiles go through School Membership, and the
// guardian role class comes from Security Runtime. The reads, the audit
// ledger and the guardian invitation storage and delivery are still bound to
// the retained repositories the root composes.

// lifecyclePeople are the People Directory commands the lifecycle writes
// persons, invited guardian contacts and guardian relationships through.
type lifecyclePeople interface {
	CreateSchoolIdentityPerson(context.Context, peopledirectory.SchoolIdentityPerson) (int64, error)
	LinkAccount(ctx context.Context, personID, accountID int64) error
	LinkTag(ctx context.Context, personID int64, tagID string) error
	CreateInvitedGuardian(context.Context, peopledirectory.InvitedGuardian) (int64, error)
	DeleteGuardianProfile(ctx context.Context, guardianID int64) error
	LinkGuardianProfileToAccount(ctx context.Context, guardianID, accountID int64) error
	LinkInvitedGuardian(context.Context, peopledirectory.InvitedGuardianLink) (bool, error)
	PromoteGuardianLinkToLegalGuardian(ctx context.Context, linkID int64) error
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
}

// lifecycleMembership are the School Membership commands the lifecycle
// creates staff and caregiver profiles through.
type lifecycleMembership interface {
	CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error)
	CreateTeacher(context.Context, schoolmembership.CreateTeacher) (schoolmembership.Teacher, error)
}

// LifecycleRepositories are the identity chain the lifecycle seams read and
// write: the retained repositories and the owner commands.
type LifecycleRepositories struct {
	Persons          personRows
	Staff            staffRows
	Teachers         teacherRows
	Students         studentRows
	GuardianProfiles guardianProfileRows
	StudentGuardians studentGuardianRows
	authEvents       authEventLedger
	// People and Membership create the persons, invited guardian contacts,
	// staff and caregiver profiles.
	People     lifecyclePeople
	Membership lifecycleMembership
}

func (r LifecycleRepositories) complete() bool {
	return r.Persons != nil && r.Staff != nil && r.Teachers != nil && r.Students != nil &&
		r.GuardianProfiles != nil && r.StudentGuardians != nil && r.authEvents != nil && r.People != nil && r.Membership != nil
}

// lifecycleDependencies binds the lifecycle seams to the identity chain in
// repos (the session repositories' Lifecycle, which carries the auth event
// ledger), the audit command, the Lehrkraft guard the role administration
// reads, and the guardian invitation delivery and enrollment claim.
func lifecycleDependencies(repos LifecycleRepositories, audit auditModels.Command, caregivers CaregiverProfiles, delivery identityPorts.GuardianInvitationDelivery, enrollments identityPorts.GuardianEnrollments, logger *slog.Logger) (*identityaccessCompose.LifecycleDependencies, error) {
	if !repos.complete() || audit == nil || !caregivers.complete() {
		return nil, errors.New("identity access composition: every lifecycle repository and owner command, the audit command and the caregiver profiles are required")
	}
	if delivery == nil {
		return nil, errors.New("identity access composition: the lifecycle needs the guardian invitation delivery")
	}
	return &identityaccessCompose.LifecycleDependencies{
		Staff:       staffDirectory{repos: repos, caregivers: caregivers},
		Audit:       previewAudit{events: repos.authEvents},
		Passwords:   passwordPolicy{},
		Guardians:   guardianDirectory{repos: repos},
		Delivery:    delivery,
		Enrollments: enrollments,
		Financial:   financialAudit{command: audit},
		Logger:      logger,
	}, nil
}

// --- staff directory --------------------------------------------------------

type staffDirectory struct {
	repos      LifecycleRepositories
	caregivers liveCaregiverProfile
}

func personRecord(person *peopledirectory.DirectoryPersonSnapshot) identityPorts.PersonRecord {
	return identityPorts.PersonRecord{
		ID: person.ID, TenantID: person.TenantID, AccountID: person.AccountID,
		FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID, Deleted: person.DeletedAt != nil,
	}
}

func staffMember(staff *peopledirectory.Staff) identityPorts.StaffMember {
	return identityPorts.StaffMember{ID: staff.ID, TenantID: staff.GetTenantID(), PersonID: staff.PersonID, Deleted: staff.DeletedAt != nil}
}

func (d staffDirectory) FindStaff(ctx context.Context, staffID int64) (identityPorts.StaffMember, bool, error) {
	staff, err := d.repos.Staff.FindByID(ctx, staffID)
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.StaffMember{}, false, nil
		}
		return identityPorts.StaffMember{}, false, err
	}
	if staff == nil {
		return identityPorts.StaffMember{}, false, nil
	}
	return staffMember(staff), true, nil
}

func (d staffDirectory) findPerson(person *peopledirectory.DirectoryPersonSnapshot, err error) (identityPorts.PersonRecord, bool, error) {
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.PersonRecord{}, false, nil
		}
		return identityPorts.PersonRecord{}, false, err
	}
	if person == nil {
		return identityPorts.PersonRecord{}, false, nil
	}
	return personRecord(person), true, nil
}

func (d staffDirectory) FindPerson(ctx context.Context, personID int64) (identityPorts.PersonRecord, bool, error) {
	return d.findPerson(d.repos.Persons.FindByID(ctx, personID))
}

func (d staffDirectory) FindPersonByAccount(ctx context.Context, accountID int64) (identityPorts.PersonRecord, bool, error) {
	return d.findPerson(d.repos.Persons.FindByAccountID(ctx, accountID))
}

func (d staffDirectory) FindPersonByTag(ctx context.Context, tagID string) (identityPorts.PersonRecord, bool, error) {
	return d.findPerson(d.repos.Persons.FindByTagID(ctx, tagID))
}

func personNames(persons map[int64]*peopledirectory.DirectoryPersonSnapshot) map[int64]identityPorts.PersonName {
	names := make(map[int64]identityPorts.PersonName, len(persons))
	for id, person := range persons {
		if person != nil {
			names[id] = identityPorts.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return names
}

func (d staffDirectory) FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]identityPorts.PersonName, error) {
	persons, err := d.repos.Persons.FindByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	return personNames(persons), nil
}

// CreatePerson creates the person at the school in context; the flows run in
// that school's transaction, so record.TenantID names the same school.
func (d staffDirectory) CreatePerson(ctx context.Context, record identityPorts.PersonRecord) (int64, error) {
	return d.repos.People.CreateSchoolIdentityPerson(ctx, peopledirectory.SchoolIdentityPerson{
		FirstName: record.FirstName, LastName: record.LastName, TagID: record.TagID,
	})
}

func (d staffDirectory) LinkPersonToAccount(ctx context.Context, personID, accountID int64) error {
	return d.repos.People.LinkAccount(ctx, personID, accountID)
}

func (d staffDirectory) LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return d.repos.People.LinkTag(ctx, personID, tagID)
}

// IsStudentPerson: the repository reports "not a student" as sql.ErrNoRows
// wrapped in a DatabaseError.
func (d staffDirectory) IsStudentPerson(ctx context.Context, personID int64) (bool, error) {
	student, err := d.repos.Students.FindByPersonID(ctx, personID)
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (d staffDirectory) FindStaffByPerson(ctx context.Context, personID int64) (identityPorts.StaffMember, bool, error) {
	staff, err := d.repos.Staff.FindByPersonID(ctx, personID)
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.StaffMember{}, false, nil
		}
		return identityPorts.StaffMember{}, false, err
	}
	if staff == nil {
		return identityPorts.StaffMember{}, false, nil
	}
	return staffMember(staff), true, nil
}

// CreateStaff creates the staff row at the school in context, which is the
// school tenantID names.
func (d staffDirectory) CreateStaff(ctx context.Context, _, personID int64) (int64, error) {
	staff, err := d.repos.Membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: personID}})
	if err != nil {
		return 0, err
	}
	return staff.ID, nil
}

func (d staffDirectory) FindCaregiverProfile(ctx context.Context, staffID int64) (identityPorts.CaregiverProfile, bool, error) {
	teacher, err := d.repos.Teachers.FindByStaffID(ctx, staffID)
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.CaregiverProfile{}, false, nil
		}
		return identityPorts.CaregiverProfile{}, false, err
	}
	if teacher == nil {
		return identityPorts.CaregiverProfile{}, false, nil
	}
	return identityPorts.CaregiverProfile{ID: teacher.ID, StaffID: teacher.StaffID, Deleted: teacher.DeletedAt != nil}, true, nil
}

func (d staffDirectory) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return d.caregivers.hasLive(ctx, accountID)
}

// CreateCaregiverProfile creates the caregiver profile at the school in
// context, which is the school tenantID names.
func (d staffDirectory) CreateCaregiverProfile(ctx context.Context, _, staffID int64, position string) (int64, error) {
	teacher, err := d.repos.Membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staffID, Role: position}})
	if err != nil {
		return 0, err
	}
	return teacher.ID, nil
}

// --- credential seams ------------------------------------------------------

// passwordPolicy binds the credential policy Security Runtime owns to the
// module's password seam and reports the module's public sentinel, so every
// flow that accepts a password answers with the same error text.
type passwordPolicy struct{}

func (passwordPolicy) ValidatePasswordStrength(password string) error {
	if securityruntime.PasswordTooWeak(password) {
		return identityaccess.ErrPasswordTooWeak
	}
	return nil
}

func (passwordPolicy) HashPassword(password string) (string, error) {
	return securityruntime.HashPassword(password)
}

// --- preview audit ----------------------------------------------------------

type previewAudit struct {
	events authEventLedger
}

func previewEvent(eventType string, event identityPorts.StaffPreviewEvent) *auditModels.AuthEvent {
	row := auditModels.NewAuthEvent(event.AdminAccountID, eventType, true, event.IPAddress)
	row.SetTenantID(event.TenantID)
	row.UserAgent = event.UserAgent
	row.SetMetadata("target_account_id", event.TargetAccountID)
	row.SetMetadata("preview_id", event.PreviewID)
	return row
}

func (a previewAudit) RecordStaffPreviewStart(ctx context.Context, event identityPorts.StaffPreviewEvent) error {
	return a.events.Create(ctx, previewEvent(auditModels.EventTypeStaffPreviewStarted, event))
}

func (a previewAudit) RecordStaffPreviewEndOnce(ctx context.Context, event identityPorts.StaffPreviewEvent) (bool, error) {
	return a.events.CreateStaffPreviewEndOnce(ctx, previewEvent(auditModels.EventTypeStaffPreviewEnded, event))
}

func (a previewAudit) LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error {
	return a.events.LockStaffPreview(ctx, adminAccountID, previewID)
}

func (a previewAudit) StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error) {
	return a.events.StaffPreviewEnded(ctx, adminAccountID, previewID)
}

// --- guardian directory ----------------------------------------------------

type guardianDirectory struct{ repos LifecycleRepositories }

func guardianProfile(profile *peopledirectory.GuardianProfileSnapshot) identityPorts.GuardianProfile {
	result := identityPorts.GuardianProfile{
		ID: profile.ID, TenantID: profile.TenantID, FirstName: profile.FirstName, LastName: profile.LastName,
		AccountID: profile.AccountID, HasAccount: profile.HasAccount,
	}
	if profile.Email != nil {
		result.Email = *profile.Email
	}
	return result
}

func studentGuardianLink(link *peopledirectory.GuardianRelationshipSnapshot) identityPorts.StudentGuardianLink {
	return identityPorts.StudentGuardianLink{
		ID: link.ID, TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole, IsPrimary: link.IsPrimary,
		IsPayer: link.IsPayer, EmergencyPriority: link.EmergencyPriority,
	}
}

func (d guardianDirectory) findProfile(profile *peopledirectory.GuardianProfileSnapshot, err error) (identityPorts.GuardianProfile, bool, error) {
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.GuardianProfile{}, false, nil
		}
		return identityPorts.GuardianProfile{}, false, err
	}
	if profile == nil {
		return identityPorts.GuardianProfile{}, false, nil
	}
	return guardianProfile(profile), true, nil
}

func (d guardianDirectory) FindGuardianProfileByEmail(ctx context.Context, email string) (identityPorts.GuardianProfile, bool, error) {
	return d.findProfile(d.repos.GuardianProfiles.FindByEmail(ctx, email))
}

func (d guardianDirectory) FindGuardianProfile(ctx context.Context, id int64) (identityPorts.GuardianProfile, bool, error) {
	return d.findProfile(d.repos.GuardianProfiles.FindByID(ctx, id))
}

func (d guardianDirectory) FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]identityPorts.GuardianProfile, error) {
	profiles, err := d.repos.GuardianProfiles.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]identityPorts.GuardianProfile, len(profiles))
	for id, profile := range profiles {
		if profile != nil {
			result[id] = guardianProfile(profile)
		}
	}
	return result, nil
}

// CreateGuardianProfile creates the invited contact at the school in context;
// the invite flow takes record.TenantID from that context.
func (d guardianDirectory) CreateGuardianProfile(ctx context.Context, record identityPorts.GuardianProfile) (int64, error) {
	return d.repos.People.CreateInvitedGuardian(ctx, peopledirectory.InvitedGuardian{
		FirstName: record.FirstName, LastName: record.LastName, Email: record.Email,
	})
}

func (d guardianDirectory) DeleteGuardianProfile(ctx context.Context, id int64) error {
	return d.repos.People.DeleteGuardianProfile(ctx, id)
}

func (d guardianDirectory) LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error {
	return d.repos.People.LinkGuardianProfileToAccount(ctx, profileID, accountID)
}

func (d guardianDirectory) FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (identityPorts.StudentGuardianLink, bool, error) {
	link, err := d.repos.StudentGuardians.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		if peoplePorts.RowMissing(err) {
			return identityPorts.StudentGuardianLink{}, false, nil
		}
		return identityPorts.StudentGuardianLink{}, false, err
	}
	if link == nil {
		return identityPorts.StudentGuardianLink{}, false, nil
	}
	return studentGuardianLink(link), true, nil
}

// LinkStudentGuardianIfAbsent links in the school in context; the invite
// flows take link.TenantID from that same context.
func (d guardianDirectory) LinkStudentGuardianIfAbsent(ctx context.Context, link identityPorts.StudentGuardianLink) (bool, error) {
	return d.repos.People.LinkInvitedGuardian(ctx, peopledirectory.InvitedGuardianLink{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, EmergencyPriority: link.EmergencyPriority,
	})
}

func (d guardianDirectory) PromoteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.repos.People.PromoteGuardianLinkToLegalGuardian(ctx, linkID)
}

func studentGuardianLinks(links []*peopledirectory.GuardianRelationshipSnapshot) []identityPorts.StudentGuardianLink {
	result := make([]identityPorts.StudentGuardianLink, 0, len(links))
	for _, link := range links {
		if link != nil {
			result = append(result, studentGuardianLink(link))
		}
	}
	return result
}

func (d guardianDirectory) ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]identityPorts.StudentGuardianLink, error) {
	links, err := d.repos.StudentGuardians.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return studentGuardianLinks(links), nil
}

func (d guardianDirectory) ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]identityPorts.StudentGuardianLink, error) {
	links, err := d.repos.StudentGuardians.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return studentGuardianLinks(links), nil
}

func (d guardianDirectory) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.repos.People.DeleteStudentGuardianLink(ctx, linkID)
}

func (d guardianDirectory) GuardianRoleClass(role string) identityPorts.GuardianRoleClass {
	switch securityruntime.ClassifyGuardianRole(role) {
	case securityruntime.GuardianRoleClassFull:
		return identityPorts.GuardianRoleFull
	case securityruntime.GuardianRoleClassSocialWorker:
		return identityPorts.GuardianRoleSocialWorker
	default:
		return identityPorts.GuardianRoleRestricted
	}
}

func (d guardianDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]identityPorts.Student, error) {
	students, err := d.repos.Students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]identityPorts.Student, len(students))
	for id, student := range students {
		if student != nil {
			result[id] = identityPorts.Student{ID: student.ID, PersonID: student.PersonID}
		}
	}
	return result, nil
}

func (d guardianDirectory) LockStudent(ctx context.Context, studentID int64) error {
	_, err := d.repos.Students.FindByIDForUpdate(ctx, studentID)
	return err
}

func (d guardianDirectory) FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]identityPorts.PersonName, error) {
	persons, err := d.repos.Persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	return personNames(persons), nil
}

type financialAudit struct{ command auditModels.Command }

func (a financialAudit) RecordPayerRemoved(ctx context.Context, guardianProfileID, studentID, actorAccountID int64) error {
	if a.command == nil {
		return errors.New("audit command is not configured")
	}
	return a.command.Append(ctx, &auditModels.GuardianFinancialChange{
		GuardianProfileID: guardianProfileID,
		StudentID:         &studentID,
		ChangedBy:         actorAccountID,
		FieldName:         auditModels.GuardianPaymentFieldIsPayer,
		OldValue:          "true",
		NewValue:          "false",
		Note:              identityaccess.PayerRemovedAuditNote,
	})
}
