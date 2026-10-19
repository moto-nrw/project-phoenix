package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// AccountLifecycle runs the account lifecycle flows (#3225): the admin
// staff-view preview, staff offboarding, the school identity chain a
// personnel role requires, parent accounts and guardian relative access.
// Identity-owned rows are read and written through the module's store; the
// facts of other owners (persons, staff, guardian profiles, relationships,
// audit evidence, the password hasher, the retained role management and
// invitation delivery) arrive through the consumer-owned ports the
// composition binds.
type AccountLifecycle struct {
	sessions    *Service
	auth        *AccountAuthentication
	store       ports.AccountLifecycleStore
	logins      ports.AccountLoginStore
	staff       ports.StaffDirectory
	profiles    ports.CaregiverProfiles
	roles       ports.RolePolicy
	audit       ports.PreviewAudit
	codec       ports.AccessTokenCodec
	admin       ports.AccountAdministration
	passwords   ports.PasswordPolicy
	guardians   ports.GuardianDirectory
	invitations ports.GuardianInvitationStore
	delivery    ports.GuardianInvitationDelivery
	enrollments ports.GuardianEnrollments
	schools     ports.SchoolDirectory
	financial   ports.FinancialAudit
	runtime     ports.Runtime
	rfid        ports.Store
	logger      *slog.Logger
}

// AccountLifecycleDependencies are the ports the lifecycle flows consume.
type AccountLifecycleDependencies struct {
	Store       ports.AccountLifecycleStore
	Logins      ports.AccountLoginStore
	RFID        ports.Store
	Staff       ports.StaffDirectory
	Profiles    ports.CaregiverProfiles
	Roles       ports.RolePolicy
	Audit       ports.PreviewAudit
	Codec       ports.AccessTokenCodec
	Admin       ports.AccountAdministration
	Passwords   ports.PasswordPolicy
	Guardians   ports.GuardianDirectory
	Invitations ports.GuardianInvitationStore
	Delivery    ports.GuardianInvitationDelivery
	Enrollments ports.GuardianEnrollments
	Schools     ports.SchoolDirectory
	Financial   ports.FinancialAudit
	Runtime     ports.Runtime
	Logger      *slog.Logger
}

// NewAccountLifecycle composes the flows over the session service and the
// account-authentication flows the preview and offboarding borrow claims and
// session revocation from.
func NewAccountLifecycle(sessions *Service, auth *AccountAuthentication, deps AccountLifecycleDependencies) (*AccountLifecycle, error) {
	switch {
	case sessions == nil:
		return nil, fmt.Errorf("identity access account lifecycle: session service is required")
	case auth == nil:
		return nil, fmt.Errorf("identity access account lifecycle: account authentication is required")
	case deps.Store == nil, deps.Logins == nil, deps.RFID == nil:
		return nil, fmt.Errorf("identity access account lifecycle: stores are required")
	case deps.Staff == nil, deps.Profiles == nil, deps.Roles == nil:
		return nil, fmt.Errorf("identity access account lifecycle: staff directory, caregiver profiles and role policy are required")
	case deps.Audit == nil, deps.Codec == nil:
		return nil, fmt.Errorf("identity access account lifecycle: preview audit and token codec are required")
	case deps.Admin == nil, deps.Passwords == nil:
		return nil, fmt.Errorf("identity access account lifecycle: account administration and password policy are required")
	case deps.Guardians == nil, deps.Invitations == nil, deps.Delivery == nil, deps.Financial == nil:
		return nil, fmt.Errorf("identity access account lifecycle: guardian directory, invitation store, delivery and financial audit are required")
	case deps.Schools == nil:
		return nil, fmt.Errorf("identity access account lifecycle: school directory is required")
	case deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account lifecycle: tenant runtime is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountLifecycle{
		sessions: sessions, auth: auth, store: deps.Store, logins: deps.Logins, rfid: deps.RFID, staff: deps.Staff,
		profiles: deps.Profiles, roles: deps.Roles, audit: deps.Audit, codec: deps.Codec, admin: deps.Admin,
		passwords: deps.Passwords, guardians: deps.Guardians, invitations: deps.Invitations, delivery: deps.Delivery,
		enrollments: deps.Enrollments, schools: deps.Schools,
		financial: deps.Financial, runtime: deps.Runtime, logger: logger,
	}, nil
}

type StaffDirectory struct{ Source identityPorts.StaffDirectory }

func (d StaffDirectory) FindStaff(ctx context.Context, staffID int64) (domain.StaffMember, bool, error) {
	staff, found, err := d.Source.FindStaff(ctx, staffID)
	return domain.StaffMember(staff), found, err
}

func (d StaffDirectory) FindPerson(ctx context.Context, personID int64) (domain.PersonRecord, bool, error) {
	person, found, err := d.Source.FindPerson(ctx, personID)
	return domain.PersonRecord(person), found, err
}

func (d StaffDirectory) FindPersonByAccount(ctx context.Context, accountID int64) (domain.PersonRecord, bool, error) {
	person, found, err := d.Source.FindPersonByAccount(ctx, accountID)
	return domain.PersonRecord(person), found, err
}

func (d StaffDirectory) FindPersonByTag(ctx context.Context, tagID string) (domain.PersonRecord, bool, error) {
	person, found, err := d.Source.FindPersonByTag(ctx, tagID)
	return domain.PersonRecord(person), found, err
}

func (d StaffDirectory) FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]domain.PersonName, error) {
	names, err := d.Source.FindPersonNames(ctx, accountIDs)
	return personNames(names), err
}

func (d StaffDirectory) CreatePerson(ctx context.Context, person domain.PersonRecord) (int64, error) {
	return d.Source.CreatePerson(ctx, identityPorts.PersonRecord(person))
}

func (d StaffDirectory) LinkPersonToAccount(ctx context.Context, personID, accountID int64) error {
	return d.Source.LinkPersonToAccount(ctx, personID, accountID)
}

func (d StaffDirectory) MoveAccountLink(ctx context.Context, fromPersonID, toPersonID, accountID int64) error {
	if err := d.Source.UnlinkPersonFromAccount(ctx, fromPersonID); err != nil {
		return err
	}
	return d.Source.LinkPersonToAccount(ctx, toPersonID, accountID)
}

func (d StaffDirectory) LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return d.Source.LinkPersonToRFIDCard(ctx, personID, tagID)
}

func (d StaffDirectory) IsStudentPerson(ctx context.Context, personID int64) (bool, error) {
	return d.Source.IsStudentPerson(ctx, personID)
}

func (d StaffDirectory) FindStaffByPerson(ctx context.Context, personID int64) (domain.StaffMember, bool, error) {
	staff, found, err := d.Source.FindStaffByPerson(ctx, personID)
	return domain.StaffMember(staff), found, err
}

func (d StaffDirectory) HasRetiredStaffMembership(ctx context.Context, personID int64) (bool, error) {
	return d.Source.HasRetiredStaffMembership(ctx, personID)
}

func (d StaffDirectory) CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error) {
	return d.Source.CreateStaff(ctx, tenantID, personID)
}

func (d StaffDirectory) FindCaregiverProfile(ctx context.Context, staffID int64) (domain.CaregiverProfile, bool, error) {
	profile, found, err := d.Source.FindCaregiverProfile(ctx, staffID)
	return domain.CaregiverProfile(profile), found, err
}

func (d StaffDirectory) CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error) {
	return d.Source.CreateCaregiverProfile(ctx, tenantID, staffID, position)
}

// rolePolicy binds the public role classification to the application port.
type RolePolicy struct{}

func (RolePolicy) RoleNeedsStaffRecord(role *domain.RoleFacts) bool {
	return identityaccess.RoleNeedsStaffRecord(publicRoleFacts(role))
}

func (RolePolicy) RoleNeedsCaregiverProfile(role *domain.RoleFacts) bool {
	return identityaccess.RoleNeedsCaregiverProfile(publicRoleFacts(role))
}

type PreviewAudit struct{ Source identityPorts.PreviewAudit }

func (a PreviewAudit) RecordStaffPreviewStart(ctx context.Context, event domain.StaffPreviewEvent) error {
	return a.Source.RecordStaffPreviewStart(ctx, identityPorts.StaffPreviewEvent(event))
}

func (a PreviewAudit) RecordStaffPreviewEndOnce(ctx context.Context, event domain.StaffPreviewEvent) (bool, error) {
	return a.Source.RecordStaffPreviewEndOnce(ctx, identityPorts.StaffPreviewEvent(event))
}

func (a PreviewAudit) LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error {
	return a.Source.LockStaffPreview(ctx, adminAccountID, previewID)
}

func (a PreviewAudit) StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error) {
	return a.Source.StaffPreviewEnded(ctx, adminAccountID, previewID)
}

func (c TokenCodec) IssueAccessToken(claims domain.SessionClaims) (string, error) {
	return c.Source.IssueAccessToken(identityaccess.SessionClaims(claims))
}

func (c TokenCodec) ParseAccessTokenAllowExpired(token string) (domain.SessionClaims, error) {
	claims, err := c.Source.ParseAccessTokenAllowExpired(token)
	return domain.SessionClaims(claims), err
}

func (c TokenCodec) AccessExpiry() time.Duration { return c.Source.AccessExpiry() }

type GuardianDirectory struct {
	Source identityPorts.GuardianDirectory
}

func (d GuardianDirectory) FindGuardianProfileByEmail(ctx context.Context, email string) (domain.GuardianProfile, bool, error) {
	profile, found, err := d.Source.FindGuardianProfileByEmail(ctx, email)
	return domain.GuardianProfile(profile), found, err
}

func (d GuardianDirectory) FindGuardianProfile(ctx context.Context, id int64) (domain.GuardianProfile, bool, error) {
	profile, found, err := d.Source.FindGuardianProfile(ctx, id)
	return domain.GuardianProfile(profile), found, err
}

func (d GuardianDirectory) LockGuardianProfile(ctx context.Context, id int64) (domain.GuardianProfile, bool, error) {
	locked, err := d.Source.LockGuardianProfile(ctx, id)
	if err != nil {
		return domain.GuardianProfile{}, false, fmt.Errorf("lock guardian profile: %w", err)
	}
	if !locked {
		return domain.GuardianProfile{}, false, nil
	}
	profile, found, err := d.Source.FindGuardianProfile(ctx, id)
	if err != nil {
		return domain.GuardianProfile{}, false, fmt.Errorf("read locked guardian profile: %w", err)
	}
	return domain.GuardianProfile(profile), found, nil
}

func (d GuardianDirectory) FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]domain.GuardianProfile, error) {
	profiles, err := d.Source.FindGuardianProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.GuardianProfile, len(profiles))
	for id, profile := range profiles {
		result[id] = domain.GuardianProfile(profile)
	}
	return result, nil
}

func (d GuardianDirectory) CreateGuardianProfile(ctx context.Context, profile domain.GuardianProfile) (int64, error) {
	return d.Source.CreateGuardianProfile(ctx, identityPorts.GuardianProfile(profile))
}

func (d GuardianDirectory) DeleteGuardianProfile(ctx context.Context, id int64) error {
	return d.Source.DeleteGuardianProfile(ctx, id)
}

func (d GuardianDirectory) LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error {
	return d.Source.LinkGuardianProfileToAccount(ctx, profileID, accountID)
}

func (d GuardianDirectory) FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (domain.StudentGuardianLink, bool, error) {
	link, found, err := d.Source.FindStudentGuardianLinkForUpdate(ctx, studentID, guardianProfileID)
	return domain.StudentGuardianLink(link), found, err
}

func (d GuardianDirectory) LinkStudentGuardianIfAbsent(ctx context.Context, link domain.StudentGuardianLink) (bool, error) {
	return d.Source.LinkStudentGuardianIfAbsent(ctx, identityPorts.StudentGuardianLink(link))
}

func (d GuardianDirectory) PromoteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.Source.PromoteStudentGuardianLink(ctx, linkID)
}

func (d GuardianDirectory) ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]domain.StudentGuardianLink, error) {
	links, err := d.Source.ListStudentGuardianLinksByStudents(ctx, studentIDs)
	return studentGuardianLinks(links), err
}

func (d GuardianDirectory) ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]domain.StudentGuardianLink, error) {
	links, err := d.Source.ListStudentGuardianLinksByProfile(ctx, guardianProfileID)
	return studentGuardianLinks(links), err
}

func (d GuardianDirectory) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.Source.DeleteStudentGuardianLink(ctx, linkID)
}

func (d GuardianDirectory) GuardianRoleClass(role string) domain.GuardianRoleClass {
	return domain.GuardianRoleClass(d.Source.GuardianRoleClass(role))
}

func (d GuardianDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]domain.Student, error) {
	students, err := d.Source.FindStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.Student, len(students))
	for id, student := range students {
		result[id] = domain.Student(student)
	}
	return result, nil
}

func (d GuardianDirectory) LockStudent(ctx context.Context, studentID int64) error {
	return d.Source.LockStudent(ctx, studentID)
}

func (d GuardianDirectory) FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]domain.PersonName, error) {
	names, err := d.Source.FindPersonNamesByIDs(ctx, personIDs)
	return personNames(names), err
}

type GuardianEnrollmentClaims struct {
	Source identityPorts.GuardianEnrollments
}

// ClaimGuardianEnrollments translates the one outcome the acceptance must
// not treat as best-effort back into the flows' vocabulary.
func (c GuardianEnrollmentClaims) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed, err := c.Source.ClaimGuardianEnrollments(ctx, accountID, email)
	if err != nil && errors.Is(err, identityaccess.ErrTransactionUnusable) {
		return claimed, fmt.Errorf("%w: %w", domain.ErrTransactionUnusable, err)
	}
	return claimed, err
}

type GuardianInvitationDelivery struct {
	Source identityPorts.GuardianInvitationDelivery
}

func (d GuardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	return d.Source.InvitationExpiry(ctx)
}

func (d GuardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	return d.Source.SchoolName(ctx, tenantID)
}

func (d GuardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, schoolName string) (int64, bool) {
	return d.Source.EnqueueInvitationEmail(ctx, identityaccess.GuardianInvitation(invitation), identityPorts.GuardianProfile(profile), schoolName)
}

func (d GuardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile domain.GuardianProfile, schoolName string) (int64, bool) {
	return d.Source.EnqueueExistingAccountEmail(ctx, identityPorts.GuardianProfile(profile), schoolName)
}

func personNames(names map[int64]identityPorts.PersonName) map[int64]domain.PersonName {
	if names == nil {
		return nil
	}
	result := make(map[int64]domain.PersonName, len(names))
	for id, name := range names {
		result[id] = domain.PersonName(name)
	}
	return result
}

func publicRoleFacts(role *domain.RoleFacts) *identityaccess.RoleFacts {
	if role == nil {
		return nil
	}
	facts := identityaccess.RoleFacts(*role)
	return &facts
}

func studentGuardianLinks(links []identityPorts.StudentGuardianLink) []domain.StudentGuardianLink {
	if links == nil {
		return nil
	}
	result := make([]domain.StudentGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, domain.StudentGuardianLink(link))
	}
	return result
}
