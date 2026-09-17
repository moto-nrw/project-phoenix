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
	"github.com/moto-nrw/project-phoenix/services/auth"
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
// The module composition fills repos from the session repositories. admin
// and delivery are read at call time because the auth service and the
// guardian invitation service are composed after the module.
type lifecycleWiring struct {
	repos    lifecycleRepositories
	settings config.SettingsService
	audit    auditModels.Command
	admin    func() *auth.Service
	delivery func() auth.GuardianInvitationDelivery
}

// lifecycleRepositories are the retained repositories the lifecycle seams
// read and write.
type lifecycleRepositories struct {
	persons             userModels.PersonRepository
	staff               userModels.StaffRepository
	teachers            userModels.TeacherRepository
	students            userModels.StudentRepository
	guardianProfiles    userModels.GuardianProfileRepository
	studentGuardians    userModels.StudentGuardianRepository
	guardianInvitations auth.GuardianInvitationStore
	authEvents          auditModels.AuthEventRepository
	roles               identityaccessCompose.RoleDirectory
	// rolesErr is why the role directory could not be bound; the lifecycle
	// composition reports it instead of the generic incompleteness.
	rolesErr error
}

func (w lifecycleWiring) complete() bool {
	r := w.repos
	return r.persons != nil && r.staff != nil && r.teachers != nil && r.students != nil &&
		r.guardianProfiles != nil && r.studentGuardians != nil && r.guardianInvitations != nil && r.authEvents != nil &&
		r.roles != nil && w.audit != nil && w.admin != nil && w.delivery != nil
}

func lifecycleDependencies(wiring *lifecycleWiring, logger *slog.Logger) (*identityaccessCompose.LifecycleDependencies, error) {
	if wiring == nil {
		return nil, nil
	}
	if wiring.repos.rolesErr != nil {
		return nil, fmt.Errorf("identity access composition: %w", wiring.repos.rolesErr)
	}
	if !wiring.complete() {
		return nil, errors.New("identity access composition: every lifecycle and role repository, the audit command, the retained auth service and the guardian invitation delivery are required")
	}
	return &identityaccessCompose.LifecycleDependencies{
		Staff:       staffDirectory{repos: wiring.repos},
		PINs:        pinHasher{},
		Lockout:     lockoutPolicy{settings: wiring.settings, logger: logger},
		Audit:       previewAudit{events: wiring.repos.authEvents},
		Admin:       accountAdministration{current: wiring.admin},
		Passwords:   passwordPolicy{},
		Guardians:   guardianDirectory{repos: wiring.repos},
		Invitations: guardianInvitationStore{store: wiring.repos.guardianInvitations},
		Delivery:    guardianInvitationDelivery{current: wiring.delivery},
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
		if auth.IsRowMissing(err) {
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
		if auth.IsRowMissing(err) {
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
		if auth.IsRowMissing(err) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (d staffDirectory) FindStaffByPerson(ctx context.Context, personID int64) (identityaccessCompose.StaffMember, bool, error) {
	staff, err := d.repos.staff.FindByPersonID(ctx, personID)
	if err != nil {
		if auth.IsRowMissing(err) {
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
		if auth.IsRowMissing(err) {
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

func (pinHasher) HashPIN(pin string) (string, error) { return auth.HashPassword(pin) }

func (pinHasher) VerifyPIN(pin, hash string) bool {
	valid, err := auth.VerifyPassword(pin, hash)
	return err == nil && valid
}

// lockoutPolicy resolves the security.account_lockout_* settings for the
// tenant in context; the MFA lockout constants are the shared fallback.
type lockoutPolicy struct {
	settings config.SettingsService
	logger   *slog.Logger
}

func (p lockoutPolicy) PINLockout(ctx context.Context) (int, time.Duration) {
	threshold := config.ResolveIntOrDefault(ctx, p.settings, configModels.KeyAccountLockoutThreshold, auth.MFALockoutThreshold, p.logger)
	minutes := config.ResolveIntOrDefault(ctx, p.settings, configModels.KeyAccountLockoutDurationMinutes, int(auth.MFALockoutDuration/time.Minute), p.logger)
	return threshold, time.Duration(minutes) * time.Minute
}

type passwordPolicy struct{}

func (passwordPolicy) ValidatePasswordStrength(password string) error {
	return auth.ValidatePasswordStrength(password)
}

func (passwordPolicy) HashPassword(password string) (string, error) {
	return auth.HashPassword(password)
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

// --- retained account management -------------------------------------------

type accountAdministration struct{ current func() *auth.Service }

func (a accountAdministration) DeactivateAccount(ctx context.Context, accountID int64) error {
	service := a.current()
	if service == nil {
		return errors.New("auth service is not composed")
	}
	return service.DeactivateAccount(ctx, int(accountID))
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
		if auth.IsRowMissing(err) {
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
		if auth.IsRowMissing(err) {
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

func (d guardianDirectory) ListStudentGuardianLinksByStudent(ctx context.Context, studentID int64) ([]identityaccessCompose.StudentGuardianLink, error) {
	links, err := d.repos.studentGuardians.FindByStudentID(ctx, studentID)
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

// --- guardian invitations ---------------------------------------------------

type guardianInvitationStore struct{ store auth.GuardianInvitationStore }

func guardianInvitationFact(record auth.GuardianInvitationRecord) identityaccessCompose.GuardianInvitation {
	return identityaccessCompose.GuardianInvitation(record)
}

func guardianInvitationFacts(records []auth.GuardianInvitationRecord) []identityaccessCompose.GuardianInvitation {
	result := make([]identityaccessCompose.GuardianInvitation, 0, len(records))
	for _, record := range records {
		result = append(result, guardianInvitationFact(record))
	}
	return result
}

func (s guardianInvitationStore) FindGuardianInvitation(ctx context.Context, id int64) (identityaccessCompose.GuardianInvitation, bool, error) {
	record, found, err := s.store.FindGuardianInvitation(ctx, id)
	return guardianInvitationFact(record), found, err
}

func (s guardianInvitationStore) ListGuardianInvitationsByProfile(ctx context.Context, guardianProfileID int64) ([]identityaccessCompose.GuardianInvitation, error) {
	records, err := s.store.ListGuardianInvitationsByProfile(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return guardianInvitationFacts(records), nil
}

func (s guardianInvitationStore) ListPendingGuardianApprovals(ctx context.Context) ([]identityaccessCompose.GuardianInvitation, error) {
	records, err := s.store.ListPendingGuardianApprovals(ctx)
	if err != nil {
		return nil, err
	}
	return guardianInvitationFacts(records), nil
}

func (s guardianInvitationStore) InsertGuardianInvitation(ctx context.Context, fact identityaccessCompose.GuardianInvitation) (identityaccessCompose.GuardianInvitation, error) {
	record, err := s.store.InsertGuardianInvitation(ctx, auth.GuardianInvitationRecord(fact))
	return guardianInvitationFact(record), err
}

func (s guardianInvitationStore) UpdateGuardianInvitation(ctx context.Context, fact identityaccessCompose.GuardianInvitation) error {
	return s.store.UpdateGuardianInvitation(ctx, auth.GuardianInvitationRecord(fact))
}

type guardianInvitationDelivery struct {
	current func() auth.GuardianInvitationDelivery
}

func (d guardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	if delivery := d.current(); delivery != nil {
		return delivery.InvitationExpiry(ctx)
	}
	return auth.GuardianTokenExpiryFallback
}

func (d guardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	if delivery := d.current(); delivery != nil {
		return delivery.SchoolName(ctx, tenantID)
	}
	return ""
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation identityaccessCompose.GuardianInvitation, profile identityaccessCompose.GuardianProfile, schoolName string) {
	delivery := d.current()
	if delivery == nil {
		return
	}
	delivery.EnqueueInvitationEmail(ctx, auth.GuardianInvitationRecord(invitation), auth.GuardianInvitationRecipient{
		FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email,
	}, schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile identityaccessCompose.GuardianProfile, schoolName string) {
	delivery := d.current()
	if delivery == nil {
		return
	}
	delivery.EnqueueExistingAccountEmail(ctx, auth.GuardianInvitationRecipient{
		FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email,
	}, schoolName)
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

// --- the retained auth service's consumer-owned lifecycle port --------------

func roleFacts(role *auth.RoleFacts) *identityaccess.RoleFacts {
	if role == nil {
		return nil
	}
	facts := identityaccess.RoleFacts(*role)
	return &facts
}

func (a *accountSessions) EnsureSchoolIdentity(ctx context.Context, input auth.SchoolIdentityInput) (*auth.SchoolIdentity, error) {
	identity, err := a.module.EnsureSchoolIdentity(ctx, identityaccess.SchoolIdentityInput{
		AccountID: input.AccountID, TenantID: input.TenantID, Role: roleFacts(input.Role),
		FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID, PersonID: input.PersonID,
		Position: input.Position, CaregiverUpgrade: input.CaregiverUpgrade, CreatePerson: input.CreatePerson,
	})
	if err != nil {
		return nil, authServiceError(err)
	}
	if identity == nil {
		return nil, nil
	}
	result := auth.SchoolIdentity(*identity)
	return &result, nil
}

func (a *accountSessions) RoleNeedsStaffRecord(role *auth.RoleFacts) bool {
	return identityaccess.RoleNeedsStaffRecord(roleFacts(role))
}

func (a *accountSessions) RoleNeedsCaregiverProfile(role *auth.RoleFacts) bool {
	return identityaccess.RoleNeedsCaregiverProfile(roleFacts(role))
}

func (a *accountSessions) IsPlatformCaregiverRole(role *auth.RoleFacts) bool {
	return identityaccess.IsPlatformCaregiverRole(roleFacts(role))
}

func (a *accountSessions) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*auth.StaffPreviewSession, error) {
	session, err := a.module.StartStaffPreview(ctx, adminAccountID, tenantID, targetAccountID, previousToken, ipAddress, userAgent)
	if err != nil {
		return nil, authServiceError(err)
	}
	result := auth.StaffPreviewSession(*session)
	return &result, nil
}

func (a *accountSessions) EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error) {
	accountID, err := a.module.EndStaffPreview(ctx, previewToken, ipAddress, userAgent)
	return accountID, authServiceError(err)
}

func (a *accountSessions) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]auth.StaffPreviewCandidate, error) {
	candidates, err := a.module.ListStaffPreviewCandidates(ctx, tenantID, excludeAccountID)
	if err != nil {
		return nil, authServiceError(err)
	}
	result := make([]auth.StaffPreviewCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, auth.StaffPreviewCandidate(candidate))
	}
	return result, nil
}

func (a *accountSessions) PreviewStaffOffboarding(ctx context.Context, accountID int64) (auth.StaffOffboardingPreview, error) {
	preview, err := a.module.PreviewStaffOffboarding(ctx, accountID)
	return auth.StaffOffboardingPreview(preview), authServiceError(err)
}

func (a *accountSessions) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (auth.StaffOffboardingResult, error) {
	result, err := a.module.ExecuteStaffOffboarding(ctx, accountID, revision)
	return auth.StaffOffboardingResult(result), authServiceError(err)
}

func (a *accountSessions) CreateParentAccount(ctx context.Context, email, username, password string) (auth.ParentAccountRecord, error) {
	account, err := a.module.CreateParentAccount(ctx, email, username, password)
	return auth.ParentAccountRecord(account), authServiceError(err)
}

func (a *accountSessions) GetParentAccountByID(ctx context.Context, id int64) (auth.ParentAccountRecord, error) {
	account, err := a.module.GetParentAccountByID(ctx, id)
	return auth.ParentAccountRecord(account), authServiceError(err)
}

func (a *accountSessions) GetParentAccountByEmail(ctx context.Context, email string) (auth.ParentAccountRecord, error) {
	account, err := a.module.GetParentAccountByEmail(ctx, email)
	return auth.ParentAccountRecord(account), authServiceError(err)
}

func (a *accountSessions) UpdateParentAccount(ctx context.Context, account auth.ParentAccountRecord) error {
	return authServiceError(a.module.UpdateParentAccount(ctx, identityaccess.ParentAccount(account)))
}

func (a *accountSessions) ActivateParentAccount(ctx context.Context, id int64) error {
	return authServiceError(a.module.ActivateParentAccount(ctx, id))
}

func (a *accountSessions) DeactivateParentAccount(ctx context.Context, id int64) error {
	return authServiceError(a.module.DeactivateParentAccount(ctx, id))
}

func (a *accountSessions) ListParentAccounts(ctx context.Context, email string, active *bool) ([]auth.ParentAccountRecord, error) {
	accounts, err := a.module.ListParentAccounts(ctx, identityaccess.ParentAccountFilter{Email: email, Active: active})
	if err != nil {
		return nil, authServiceError(err)
	}
	result := make([]auth.ParentAccountRecord, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, auth.ParentAccountRecord(account))
	}
	return result, nil
}

func (a *accountSessions) InviteToStudent(ctx context.Context, req auth.InviteToStudentRequest) (*auth.InviteToStudentResult, error) {
	result, err := a.module.InviteToStudent(ctx, identityaccess.InviteToStudentRequest(req))
	if err != nil {
		return nil, authServiceError(err)
	}
	return &auth.InviteToStudentResult{
		Outcome: auth.InviteToStudentOutcome(result.Outcome), GuardianProfileID: result.GuardianProfileID,
		InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
	}, nil
}

func (a *accountSessions) ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	return authServiceError(a.module.ApproveInvitation(ctx, invitationID, approverAccountID))
}

func (a *accountSessions) RejectInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	return authServiceError(a.module.RejectInvitation(ctx, invitationID, approverAccountID))
}

func (a *accountSessions) PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error) {
	studentID, err := a.module.PendingInvitationStudentID(ctx, invitationID)
	return studentID, authServiceError(err)
}

func (a *accountSessions) ListPendingApprovalsDetailed(ctx context.Context) ([]*auth.PendingApprovalView, error) {
	views, err := a.module.ListPendingApprovalsDetailed(ctx)
	if err != nil {
		return nil, authServiceError(err)
	}
	result := make([]*auth.PendingApprovalView, 0, len(views))
	for _, view := range views {
		retained := auth.PendingApprovalView(view)
		result = append(result, &retained)
	}
	return result, nil
}

func (a *accountSessions) RevokeAccess(ctx context.Context, req auth.RevokeAccessRequest) error {
	return authServiceError(a.module.RevokeAccess(ctx, identityaccess.RevokeAccessRequest(req)))
}

var _ auth.AccountLifecycle = (*accountSessions)(nil)

// lifecycleRetainedSentinels extend the session translation with the
// lifecycle outcomes the retained consumers switch on.
var lifecycleRetainedSentinels = []retainedSentinel{
	{identityaccess.ErrPreviewSelf, auth.ErrPreviewSelf},
	{identityaccess.ErrPreviewTargetNotStaff, auth.ErrPreviewTargetNotStaff},
	{identityaccess.ErrPreviewTokenInvalid, auth.ErrPreviewTokenInvalid},
	{identityaccess.ErrStaffOffboardingConflict, auth.ErrStaffOffboardingConflict},
	{identityaccess.ErrSchoolIdentityNamesRequired, auth.ErrSchoolIdentityNamesRequired},
	{identityaccess.ErrSchoolIdentityPersonIsStudent, auth.ErrSchoolIdentityPersonIsStudent},
	{identityaccess.ErrSchoolIdentityTagUnknown, auth.ErrSchoolIdentityTagUnknown},
	{identityaccess.ErrSchoolIdentityTagConflict, auth.ErrSchoolIdentityTagConflict},
	{identityaccess.ErrSchoolIdentityTagTaken, auth.ErrSchoolIdentityTagTaken},
	{identityaccess.ErrParentAccountNotFound, auth.ErrParentAccountNotFound},
	{identityaccess.ErrEmailAlreadyExists, auth.ErrEmailAlreadyExists},
	{identityaccess.ErrUsernameAlreadyExists, auth.ErrUsernameAlreadyExists},
	{identityaccess.ErrCannotRemovePrimaryGuardian, auth.ErrCannotRemovePrimaryGuardian},
	{identityaccess.ErrCannotRemoveStaffManagedGuardian, auth.ErrCannotRemoveStaffManagedGuardian},
	{identityaccess.ErrCannotRemoveOwnAccess, auth.ErrCannotRemoveOwnAccess},
	{identityaccess.ErrCannotRemovePayerGuardian, auth.ErrCannotRemovePayerGuardian},
	{identityaccess.ErrInviteSocialWorkerManaged, auth.ErrInviteSocialWorkerManaged},
	{identityaccess.ErrGuardianInvitationNotFound, auth.ErrInvitationNotFound},
	{identityaccess.ErrGuardianInvitationExpired, auth.ErrInvitationExpired},
	{identityaccess.ErrAccountLifecycleUnavailable, auth.ErrAccountLifecycleUnavailable},
}
