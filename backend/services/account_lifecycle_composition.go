package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// Identity & Access owns staff PIN verification and lockout, the admin
// staff-view preview, staff offboarding access, the school identity chain,
// parent accounts and guardian relative access (#3225). This file binds the
// seams those flows need to the retained owners the root still composes
// (persons, staff, teachers, students, guardian profiles and relationships,
// the audit ledger, the retained account management and role storage, the
// guardian invitation storage and delivery) and serves the retained auth
// service's consumer-owned port over the public module.

// lifecycleWiring is the retained material the lifecycle seams are bound to.
// The module composition fills repos from the session repositories.
type lifecycleWiring struct {
	repos    lifecycleRepositories
	settings config.SettingsService
	audit    auditModels.Command
	// guardianMail carries the invitation delivery and the enrollment claim
	// the guardian invitation flows leave to the root.
	guardianMail *guardianInvitationWiring
}

// lifecycleRepositories are the retained repositories the lifecycle seams
// read and write.
type lifecycleRepositories struct {
	persons          userModels.PersonRepository
	staff            userModels.StaffRepository
	teachers         userModels.TeacherRepository
	students         userModels.StudentRepository
	guardianProfiles userModels.GuardianProfileRepository
	studentGuardians userModels.StudentGuardianRepository
	authEvents       auditModels.AuthEventRepository
	roles            identityaccessCompose.RoleDirectory
	// rolesErr is why the role directory could not be bound; the lifecycle
	// composition reports it instead of the generic incompleteness.
	rolesErr error
}

func (w lifecycleWiring) complete() bool {
	r := w.repos
	return r.persons != nil && r.staff != nil && r.teachers != nil && r.students != nil &&
		r.guardianProfiles != nil && r.studentGuardians != nil && r.authEvents != nil &&
		r.roles != nil && w.audit != nil
}

func lifecycleDependencies(wiring *lifecycleWiring, logger *slog.Logger) (*identityaccessCompose.LifecycleDependencies, error) {
	if wiring == nil {
		return nil, nil
	}
	if wiring.repos.rolesErr != nil {
		return nil, fmt.Errorf("identity access composition: %w", wiring.repos.rolesErr)
	}
	if !wiring.complete() {
		return nil, errors.New("identity access composition: every lifecycle and role repository and the audit command are required")
	}
	delivery, enrollments, err := guardianInvitationDependencies(wiring.guardianMail)
	if err != nil {
		return nil, err
	}
	return &identityaccessCompose.LifecycleDependencies{
		Staff:       staffDirectory{repos: wiring.repos},
		PINs:        pinHasher{},
		Lockout:     lockoutPolicy{settings: wiring.settings, logger: logger},
		Audit:       previewAudit{events: wiring.repos.authEvents},
		Passwords:   passwordPolicy{},
		Guardians:   guardianDirectory{repos: wiring.repos},
		Delivery:    delivery,
		Enrollments: enrollments,
		Financial:   financialAudit{command: wiring.audit},
		Roles:       wiring.repos.roles,
		Logger:      logger,
	}, nil
}

// --- staff directory --------------------------------------------------------

type staffDirectory struct{ repos lifecycleRepositories }

func personRecord(person *userModels.Person) identityaccessCompose.PersonRecord {
	return identityaccessCompose.PersonRecord{
		ID: person.ID, TenantID: person.TenantID, AccountID: person.AccountID,
		FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID, Deleted: person.DeletedAt != nil,
	}
}

func staffMember(staff *userModels.Staff) identityaccessCompose.StaffMember {
	return identityaccessCompose.StaffMember{ID: staff.ID, TenantID: staff.GetTenantID(), PersonID: staff.PersonID, Deleted: staff.DeletedAt != nil}
}

func (d staffDirectory) FindStaff(ctx context.Context, staffID int64) (identityaccessCompose.StaffMember, bool, error) {
	staff, err := d.repos.staff.FindByID(ctx, staffID)
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.StaffMember{}, false, nil
		}
		return identityaccessCompose.StaffMember{}, false, err
	}
	if staff == nil {
		return identityaccessCompose.StaffMember{}, false, nil
	}
	return staffMember(staff), true, nil
}

func (d staffDirectory) findPerson(person *userModels.Person, err error) (identityaccessCompose.PersonRecord, bool, error) {
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.PersonRecord{}, false, nil
		}
		return identityaccessCompose.PersonRecord{}, false, err
	}
	if person == nil {
		return identityaccessCompose.PersonRecord{}, false, nil
	}
	return personRecord(person), true, nil
}

func (d staffDirectory) FindPerson(ctx context.Context, personID int64) (identityaccessCompose.PersonRecord, bool, error) {
	return d.findPerson(d.repos.persons.FindByID(ctx, personID))
}

func (d staffDirectory) FindPersonByAccount(ctx context.Context, accountID int64) (identityaccessCompose.PersonRecord, bool, error) {
	return d.findPerson(d.repos.persons.FindByAccountID(ctx, accountID))
}

func (d staffDirectory) FindPersonByTag(ctx context.Context, tagID string) (identityaccessCompose.PersonRecord, bool, error) {
	return d.findPerson(d.repos.persons.FindByTagID(ctx, tagID))
}

func personNames(persons map[int64]*userModels.Person) map[int64]identityaccessCompose.PersonName {
	names := make(map[int64]identityaccessCompose.PersonName, len(persons))
	for id, person := range persons {
		if person != nil {
			names[id] = identityaccessCompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return names
}

func (d staffDirectory) FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]identityaccessCompose.PersonName, error) {
	persons, err := d.repos.persons.FindByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	return personNames(persons), nil
}

func (d staffDirectory) CreatePerson(ctx context.Context, record identityaccessCompose.PersonRecord) (int64, error) {
	person := &userModels.Person{FirstName: record.FirstName, LastName: record.LastName, TagID: record.TagID}
	person.SetTenantID(record.TenantID)
	if err := d.repos.persons.Create(ctx, person); err != nil {
		return 0, err
	}
	return person.ID, nil
}

func (d staffDirectory) LinkPersonToAccount(ctx context.Context, personID, accountID int64) error {
	return d.repos.persons.LinkToAccount(ctx, personID, accountID)
}

func (d staffDirectory) LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return d.repos.persons.LinkToRFIDCard(ctx, personID, tagID)
}

// IsStudentPerson: the repository reports "not a student" as sql.ErrNoRows
// wrapped in a DatabaseError.
func (d staffDirectory) IsStudentPerson(ctx context.Context, personID int64) (bool, error) {
	student, err := d.repos.students.FindByPersonID(ctx, personID)
	if err != nil {
		if userModels.RowMissing(err) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (d staffDirectory) FindStaffByPerson(ctx context.Context, personID int64) (identityaccessCompose.StaffMember, bool, error) {
	staff, err := d.repos.staff.FindByPersonID(ctx, personID)
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.StaffMember{}, false, nil
		}
		return identityaccessCompose.StaffMember{}, false, err
	}
	if staff == nil {
		return identityaccessCompose.StaffMember{}, false, nil
	}
	return staffMember(staff), true, nil
}

func (d staffDirectory) CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error) {
	staff := &userModels.Staff{PersonID: personID}
	staff.SetTenantID(tenantID)
	if err := d.repos.staff.Create(ctx, staff); err != nil {
		return 0, err
	}
	return staff.ID, nil
}

func (d staffDirectory) FindCaregiverProfile(ctx context.Context, staffID int64) (identityaccessCompose.CaregiverProfile, bool, error) {
	teacher, err := d.repos.teachers.FindByStaffID(ctx, staffID)
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.CaregiverProfile{}, false, nil
		}
		return identityaccessCompose.CaregiverProfile{}, false, err
	}
	if teacher == nil {
		return identityaccessCompose.CaregiverProfile{}, false, nil
	}
	return identityaccessCompose.CaregiverProfile{ID: teacher.ID, StaffID: teacher.StaffID, Deleted: teacher.DeletedAt != nil}, true, nil
}

func (d staffDirectory) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return hasLiveCaregiverProfile(ctx, d.repos.persons, d.repos.staff, d.repos.teachers, accountID)
}

func (d staffDirectory) CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error) {
	teacher := &userModels.Teacher{StaffID: staffID, Role: position}
	teacher.SetTenantID(tenantID)
	if err := d.repos.teachers.Create(ctx, teacher); err != nil {
		return 0, err
	}
	return teacher.ID, nil
}

// --- credential seams ------------------------------------------------------

// pinHasher reuses the Argon2id helpers accounts store their credentials with.
type pinHasher struct{}

func (pinHasher) HashPIN(pin string) (string, error) { return securityruntime.HashPassword(pin) }

func (pinHasher) VerifyPIN(pin, hash string) bool {
	valid, err := securityruntime.VerifyPassword(pin, hash)
	return err == nil && valid
}

// lockoutPolicy resolves the security.account_lockout_* settings for the
// tenant in context. The PIN lockout and the MFA lockout share one
// threshold and one window, so the identity module's constants are the
// fallback here too (#586: one source of truth for the 5-attempt /
// 15-minute policy).
type lockoutPolicy struct {
	settings config.SettingsService
	logger   *slog.Logger
}

func (p lockoutPolicy) PINLockout(ctx context.Context) (int, time.Duration) {
	threshold := config.ResolveIntOrDefault(ctx, p.settings, configModels.KeyAccountLockoutThreshold, identityaccess.MFALockoutThreshold, p.logger)
	minutes := config.ResolveIntOrDefault(ctx, p.settings, configModels.KeyAccountLockoutDurationMinutes, int(identityaccess.MFALockoutDuration/time.Minute), p.logger)
	return threshold, time.Duration(minutes) * time.Minute
}

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
	events auditModels.AuthEventRepository
}

func previewEvent(eventType string, event identityaccessCompose.StaffPreviewEvent) *auditModels.AuthEvent {
	row := auditModels.NewAuthEvent(event.AdminAccountID, eventType, true, event.IPAddress)
	row.SetTenantID(event.TenantID)
	row.UserAgent = event.UserAgent
	row.SetMetadata("target_account_id", event.TargetAccountID)
	row.SetMetadata("preview_id", event.PreviewID)
	return row
}

func (a previewAudit) RecordStaffPreviewStart(ctx context.Context, event identityaccessCompose.StaffPreviewEvent) error {
	return a.events.Create(ctx, previewEvent(auditModels.EventTypeStaffPreviewStarted, event))
}

func (a previewAudit) RecordStaffPreviewEndOnce(ctx context.Context, event identityaccessCompose.StaffPreviewEvent) (bool, error) {
	return a.events.CreateStaffPreviewEndOnce(ctx, previewEvent(auditModels.EventTypeStaffPreviewEnded, event))
}

func (a previewAudit) LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error {
	return a.events.LockStaffPreview(ctx, adminAccountID, previewID)
}

func (a previewAudit) StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error) {
	return a.events.StaffPreviewEnded(ctx, adminAccountID, previewID)
}

// --- guardian directory ----------------------------------------------------

type guardianDirectory struct{ repos lifecycleRepositories }

func guardianProfile(profile *userModels.GuardianProfile) identityaccessCompose.GuardianProfile {
	result := identityaccessCompose.GuardianProfile{
		ID: profile.ID, TenantID: profile.TenantID, FirstName: profile.FirstName, LastName: profile.LastName,
		AccountID: profile.AccountID, HasAccount: profile.HasAccount,
	}
	if profile.Email != nil {
		result.Email = *profile.Email
	}
	return result
}

func studentGuardianLink(link *userModels.StudentGuardian) identityaccessCompose.StudentGuardianLink {
	return identityaccessCompose.StudentGuardianLink{
		ID: link.ID, TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole, IsPrimary: link.IsPrimary,
		IsPayer: link.IsPayer, EmergencyPriority: link.EmergencyPriority,
	}
}

func (d guardianDirectory) findProfile(profile *userModels.GuardianProfile, err error) (identityaccessCompose.GuardianProfile, bool, error) {
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.GuardianProfile{}, false, nil
		}
		return identityaccessCompose.GuardianProfile{}, false, err
	}
	if profile == nil {
		return identityaccessCompose.GuardianProfile{}, false, nil
	}
	return guardianProfile(profile), true, nil
}

func (d guardianDirectory) FindGuardianProfileByEmail(ctx context.Context, email string) (identityaccessCompose.GuardianProfile, bool, error) {
	return d.findProfile(d.repos.guardianProfiles.FindByEmail(ctx, email))
}

func (d guardianDirectory) FindGuardianProfile(ctx context.Context, id int64) (identityaccessCompose.GuardianProfile, bool, error) {
	return d.findProfile(d.repos.guardianProfiles.FindByID(ctx, id))
}

func (d guardianDirectory) FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]identityaccessCompose.GuardianProfile, error) {
	profiles, err := d.repos.guardianProfiles.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]identityaccessCompose.GuardianProfile, len(profiles))
	for id, profile := range profiles {
		if profile != nil {
			result[id] = guardianProfile(profile)
		}
	}
	return result, nil
}

func (d guardianDirectory) CreateGuardianProfile(ctx context.Context, record identityaccessCompose.GuardianProfile) (int64, error) {
	email := record.Email
	profile := &userModels.GuardianProfile{
		FirstName: record.FirstName, LastName: record.LastName, Email: &email,
		PreferredContactMethod: "email", LanguagePreference: "de",
	}
	if record.TenantID > 0 {
		profile.SetTenantID(record.TenantID)
	}
	if err := profile.Validate(); err != nil {
		return 0, err
	}
	if err := d.repos.guardianProfiles.Create(ctx, profile); err != nil {
		return 0, err
	}
	return profile.ID, nil
}

func (d guardianDirectory) DeleteGuardianProfile(ctx context.Context, id int64) error {
	return d.repos.guardianProfiles.Delete(ctx, id)
}

func (d guardianDirectory) LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error {
	return d.repos.guardianProfiles.LinkAccount(ctx, profileID, accountID)
}

func (d guardianDirectory) FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (identityaccessCompose.StudentGuardianLink, bool, error) {
	link, err := d.repos.studentGuardians.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		if userModels.RowMissing(err) {
			return identityaccessCompose.StudentGuardianLink{}, false, nil
		}
		return identityaccessCompose.StudentGuardianLink{}, false, err
	}
	if link == nil {
		return identityaccessCompose.StudentGuardianLink{}, false, nil
	}
	return studentGuardianLink(link), true, nil
}

func (d guardianDirectory) LinkStudentGuardianIfAbsent(ctx context.Context, link identityaccessCompose.StudentGuardianLink) (bool, error) {
	rel := &userModels.StudentGuardian{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, EmergencyPriority: link.EmergencyPriority,
	}
	if link.TenantID > 0 {
		rel.SetTenantID(link.TenantID)
	}
	return d.repos.studentGuardians.LinkIfNotExists(ctx, rel)
}

func (d guardianDirectory) PromoteStudentGuardianLink(ctx context.Context, linkID int64) error {
	link, err := d.repos.studentGuardians.FindByID(ctx, linkID)
	if err != nil {
		return err
	}
	if link == nil {
		return userModels.ErrStudentGuardianNotFound
	}
	users.PromoteStudentGuardianLink(link)
	return d.repos.studentGuardians.Update(ctx, link)
}

func studentGuardianLinks(links []*userModels.StudentGuardian) []identityaccessCompose.StudentGuardianLink {
	result := make([]identityaccessCompose.StudentGuardianLink, 0, len(links))
	for _, link := range links {
		if link != nil {
			result = append(result, studentGuardianLink(link))
		}
	}
	return result
}

func (d guardianDirectory) ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]identityaccessCompose.StudentGuardianLink, error) {
	links, err := d.repos.studentGuardians.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return studentGuardianLinks(links), nil
}

func (d guardianDirectory) ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]identityaccessCompose.StudentGuardianLink, error) {
	links, err := d.repos.studentGuardians.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return studentGuardianLinks(links), nil
}

func (d guardianDirectory) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.repos.studentGuardians.Delete(ctx, linkID)
}

func (d guardianDirectory) GuardianRoleClass(role string) identityaccessCompose.GuardianRoleClass {
	switch {
	case users.IsFullGuardianRole(role):
		return identityaccessCompose.GuardianRoleFull
	case users.IsSocialWorkerGuardianRole(role):
		return identityaccessCompose.GuardianRoleSocialWorker
	default:
		return identityaccessCompose.GuardianRoleRestricted
	}
}

func (d guardianDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]identityaccessCompose.Student, error) {
	students, err := d.repos.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]identityaccessCompose.Student, len(students))
	for id, student := range students {
		if student != nil {
			result[id] = identityaccessCompose.Student{ID: student.ID, PersonID: student.PersonID}
		}
	}
	return result, nil
}

func (d guardianDirectory) LockStudent(ctx context.Context, studentID int64) error {
	_, err := d.repos.students.FindByIDForUpdate(ctx, studentID)
	return err
}

func (d guardianDirectory) FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]identityaccessCompose.PersonName, error) {
	persons, err := d.repos.persons.FindByIDs(ctx, personIDs)
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
		Note:              "Erziehungsberechtigte Person vom Kind entfernt",
	})
}

// --- access token codec seams -----------------------------------------------

func (c sessionTokenCodec) IssueAccessToken(claims identityaccess.SessionClaims) (string, error) {
	return c.tokenAuth.CreateJWT(appClaims(claims))
}

func (c sessionTokenCodec) ParseAccessTokenAllowExpired(token string) (identityaccess.SessionClaims, error) {
	claims, err := c.tokenAuth.ParseExpiredAccessJWT(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	return sessionClaims(claims), nil
}

func (c sessionTokenCodec) AccessExpiry() time.Duration { return c.tokenAuth.JwtExpiry }

var _ identityaccessCompose.TokenCodec = sessionTokenCodec{}

// --- device staff PIN -------------------------------------------------------

// StaffPINPrincipal is the verified staff member the device middleware binds
// a kiosk action to. Its accessors satisfy the device authentication adapter
// without a people-directory row leaving this root.
type StaffPINPrincipal struct {
	ID       int64
	PersonID int64
	TenantID int64
}

func (p *StaffPINPrincipal) GetID() any         { return p.ID }
func (p *StaffPINPrincipal) GetTenantID() int64 { return p.TenantID }

// StaffPINAuthenticator verifies a staff-specific PIN inside the staff
// member's tenant boundary. Device middleware uses this narrow contract to
// bind kiosk attribution to a person.
type StaffPINAuthenticator interface {
	AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (*StaffPINPrincipal, error)
}

type staffPINAuthenticator struct{ module *identityaccess.Module }

// NewStaffPINAuthenticator serves the device port from the public module.
func NewStaffPINAuthenticator(module *identityaccess.Module) StaffPINAuthenticator {
	return staffPINAuthenticator{module: module}
}

func (a staffPINAuthenticator) AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (*StaffPINPrincipal, error) {
	if a.module == nil {
		return nil, errors.New("staff PIN authentication is not composed")
	}
	staff, err := a.module.AuthenticateStaffPIN(ctx, tenantID, staffID, pin)
	if err != nil {
		return nil, err
	}
	return &StaffPINPrincipal{ID: staff.ID, PersonID: staff.PersonID, TenantID: staff.TenantID}, nil
}
