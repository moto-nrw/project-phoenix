package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The account lifecycle flows (#3225) and the role administration (#3314)
// need facts other owners hold and seams the composition root binds: persons,
// staff and caregiver profiles, guardian profiles and relationships, the
// audit evidence, the password hasher, the retained role storage and
// account management and the retained guardian invitation delivery. The
// seams below are expressed in public values; this package adapts them to the
// consumer-owned ports.

// PasswordPolicy validates and hashes parent account passwords.
type PasswordPolicy interface {
	ValidatePasswordStrength(password string) error
	HashPassword(password string) (string, error)
}

// FinancialAudit records the removal of a child's payer.
type FinancialAudit interface {
	RecordPayerRemoved(ctx context.Context, guardianProfileID, studentID, actorAccountID int64) error
}

// LifecycleDependencies are the seams the account lifecycle flows need
// beyond the database and the session dependencies.
type LifecycleDependencies struct {
	Staff     identityPorts.StaffDirectory
	Audit     identityPorts.PreviewAudit
	Passwords PasswordPolicy
	Guardians identityPorts.GuardianDirectory
	Delivery  identityPorts.GuardianInvitationDelivery
	// Enrollments is optional: without it an acceptance claims no
	// pre-account enrollment requests.
	Enrollments identityPorts.GuardianEnrollments
	Financial   FinancialAudit
	Logger      *slog.Logger
}

// newAccountLifecycle composes the lifecycle flows and the role
// administration: offboarding removes roles through the administration, and
// an assignment completes the school identity through the lifecycle.
func newAccountLifecycle(service *application.Service, auth *application.AccountAuthentication, store *postgres.Store, sessions *SessionDependencies, deps *LifecycleDependencies, administration *application.AccountAdministration) (*application.AccountLifecycle, *application.RoleAdministration, error) {
	if deps == nil {
		return nil, nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, nil, errors.New("identity access compose: the lifecycle flows require the session dependencies")
	}
	switch {
	case deps.Staff == nil, deps.Audit == nil,
		deps.Passwords == nil, deps.Guardians == nil, deps.Delivery == nil, deps.Financial == nil:
		return nil, nil, errors.New("identity access compose: every lifecycle dependency is required")
	case administration == nil:
		return nil, nil, errors.New("identity access compose: the account administration is required")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	runtime := tenantRuntime{attach: attach, runner: newTransactionRunner()}
	var lifecycle *application.AccountLifecycle
	roles, err := newRoleAdministration(auth, runtime, postgres.NewRoleStore(store), administration, deps, func() *application.AccountLifecycle { return lifecycle })
	if err != nil {
		return nil, nil, err
	}
	lifecycle, err = application.NewAccountLifecycle(service, auth, application.AccountLifecycleDependencies{
		Store: store, Logins: store, RFID: canonicalTagStore{store},
		Staff:       staffDirectory{deps.Staff},
		Profiles:    staffDirectory{deps.Staff},
		Roles:       rolePolicy{},
		Audit:       previewAudit{deps.Audit},
		Codec:       tokenCodec{sessions.Codec},
		Admin:       accountAdministration{roles: roles, accounts: administration},
		Passwords:   deps.Passwords,
		Guardians:   guardianDirectory{deps.Guardians},
		Invitations: store,
		Delivery:    guardianInvitationDelivery{deps.Delivery},
		Enrollments: guardianEnrollments(deps.Enrollments),
		Schools:     schoolDirectory{sessions.Schools},
		Financial:   deps.Financial,
		Runtime:     runtime,
		Logger:      deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	return lifecycle, roles, nil
}

// --- port adapters ---------------------------------------------------------

type staffDirectory struct{ source identityPorts.StaffDirectory }

func (d staffDirectory) FindStaff(ctx context.Context, staffID int64) (domain.StaffMember, bool, error) {
	staff, found, err := d.source.FindStaff(ctx, staffID)
	return domain.StaffMember(staff), found, err
}

func (d staffDirectory) FindPerson(ctx context.Context, personID int64) (domain.PersonRecord, bool, error) {
	person, found, err := d.source.FindPerson(ctx, personID)
	return domain.PersonRecord(person), found, err
}

func (d staffDirectory) FindPersonByAccount(ctx context.Context, accountID int64) (domain.PersonRecord, bool, error) {
	person, found, err := d.source.FindPersonByAccount(ctx, accountID)
	return domain.PersonRecord(person), found, err
}

func (d staffDirectory) FindPersonByTag(ctx context.Context, tagID string) (domain.PersonRecord, bool, error) {
	person, found, err := d.source.FindPersonByTag(ctx, tagID)
	return domain.PersonRecord(person), found, err
}

func (d staffDirectory) FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]domain.PersonName, error) {
	names, err := d.source.FindPersonNames(ctx, accountIDs)
	return personNames(names), err
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

func (d staffDirectory) CreatePerson(ctx context.Context, person domain.PersonRecord) (int64, error) {
	return d.source.CreatePerson(ctx, identityPorts.PersonRecord(person))
}

func (d staffDirectory) LinkPersonToAccount(ctx context.Context, personID, accountID int64) error {
	return d.source.LinkPersonToAccount(ctx, personID, accountID)
}

func (d staffDirectory) LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return d.source.LinkPersonToRFIDCard(ctx, personID, tagID)
}

func (d staffDirectory) IsStudentPerson(ctx context.Context, personID int64) (bool, error) {
	return d.source.IsStudentPerson(ctx, personID)
}

func (d staffDirectory) FindStaffByPerson(ctx context.Context, personID int64) (domain.StaffMember, bool, error) {
	staff, found, err := d.source.FindStaffByPerson(ctx, personID)
	return domain.StaffMember(staff), found, err
}

func (d staffDirectory) CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error) {
	return d.source.CreateStaff(ctx, tenantID, personID)
}

func (d staffDirectory) FindCaregiverProfile(ctx context.Context, staffID int64) (domain.CaregiverProfile, bool, error) {
	profile, found, err := d.source.FindCaregiverProfile(ctx, staffID)
	return domain.CaregiverProfile(profile), found, err
}

func (d staffDirectory) CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error) {
	return d.source.CreateCaregiverProfile(ctx, tenantID, staffID, position)
}

// rolePolicy binds the public role classification to the application port.
type rolePolicy struct{}

func publicRoleFacts(role *domain.RoleFacts) *identityaccess.RoleFacts {
	if role == nil {
		return nil
	}
	facts := identityaccess.RoleFacts(*role)
	return &facts
}

func (rolePolicy) RoleNeedsStaffRecord(role *domain.RoleFacts) bool {
	return identityaccess.RoleNeedsStaffRecord(publicRoleFacts(role))
}

func (rolePolicy) RoleNeedsCaregiverProfile(role *domain.RoleFacts) bool {
	return identityaccess.RoleNeedsCaregiverProfile(publicRoleFacts(role))
}

type previewAudit struct{ source identityPorts.PreviewAudit }

func (a previewAudit) RecordStaffPreviewStart(ctx context.Context, event domain.StaffPreviewEvent) error {
	return a.source.RecordStaffPreviewStart(ctx, identityPorts.StaffPreviewEvent(event))
}

func (a previewAudit) RecordStaffPreviewEndOnce(ctx context.Context, event domain.StaffPreviewEvent) (bool, error) {
	return a.source.RecordStaffPreviewEndOnce(ctx, identityPorts.StaffPreviewEvent(event))
}

func (a previewAudit) LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error {
	return a.source.LockStaffPreview(ctx, adminAccountID, previewID)
}

func (a previewAudit) StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error) {
	return a.source.StaffPreviewEnded(ctx, adminAccountID, previewID)
}

func (c tokenCodec) IssueAccessToken(claims domain.SessionClaims) (string, error) {
	return c.source.IssueAccessToken(identityaccess.SessionClaims(claims))
}

func (c tokenCodec) ParseAccessTokenAllowExpired(token string) (domain.SessionClaims, error) {
	claims, err := c.source.ParseAccessTokenAllowExpired(token)
	return domain.SessionClaims(claims), err
}

func (c tokenCodec) AccessExpiry() time.Duration { return c.source.AccessExpiry() }

type guardianDirectory struct {
	source identityPorts.GuardianDirectory
}

func (d guardianDirectory) FindGuardianProfileByEmail(ctx context.Context, email string) (domain.GuardianProfile, bool, error) {
	profile, found, err := d.source.FindGuardianProfileByEmail(ctx, email)
	return domain.GuardianProfile(profile), found, err
}

func (d guardianDirectory) FindGuardianProfile(ctx context.Context, id int64) (domain.GuardianProfile, bool, error) {
	profile, found, err := d.source.FindGuardianProfile(ctx, id)
	return domain.GuardianProfile(profile), found, err
}

func (d guardianDirectory) FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]domain.GuardianProfile, error) {
	profiles, err := d.source.FindGuardianProfiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.GuardianProfile, len(profiles))
	for id, profile := range profiles {
		result[id] = domain.GuardianProfile(profile)
	}
	return result, nil
}

func (d guardianDirectory) CreateGuardianProfile(ctx context.Context, profile domain.GuardianProfile) (int64, error) {
	return d.source.CreateGuardianProfile(ctx, identityPorts.GuardianProfile(profile))
}

func (d guardianDirectory) DeleteGuardianProfile(ctx context.Context, id int64) error {
	return d.source.DeleteGuardianProfile(ctx, id)
}

func (d guardianDirectory) LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error {
	return d.source.LinkGuardianProfileToAccount(ctx, profileID, accountID)
}

func (d guardianDirectory) FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (domain.StudentGuardianLink, bool, error) {
	link, found, err := d.source.FindStudentGuardianLinkForUpdate(ctx, studentID, guardianProfileID)
	return domain.StudentGuardianLink(link), found, err
}

func (d guardianDirectory) LinkStudentGuardianIfAbsent(ctx context.Context, link domain.StudentGuardianLink) (bool, error) {
	return d.source.LinkStudentGuardianIfAbsent(ctx, identityPorts.StudentGuardianLink(link))
}

func (d guardianDirectory) PromoteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.source.PromoteStudentGuardianLink(ctx, linkID)
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

func (d guardianDirectory) ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]domain.StudentGuardianLink, error) {
	links, err := d.source.ListStudentGuardianLinksByStudents(ctx, studentIDs)
	return studentGuardianLinks(links), err
}

func (d guardianDirectory) ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]domain.StudentGuardianLink, error) {
	links, err := d.source.ListStudentGuardianLinksByProfile(ctx, guardianProfileID)
	return studentGuardianLinks(links), err
}

func (d guardianDirectory) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.source.DeleteStudentGuardianLink(ctx, linkID)
}

func (d guardianDirectory) GuardianRoleClass(role string) domain.GuardianRoleClass {
	return domain.GuardianRoleClass(d.source.GuardianRoleClass(role))
}

func (d guardianDirectory) FindStudents(ctx context.Context, ids []int64) (map[int64]domain.Student, error) {
	students, err := d.source.FindStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.Student, len(students))
	for id, student := range students {
		result[id] = domain.Student(student)
	}
	return result, nil
}

func (d guardianDirectory) LockStudent(ctx context.Context, studentID int64) error {
	return d.source.LockStudent(ctx, studentID)
}

func (d guardianDirectory) FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]domain.PersonName, error) {
	names, err := d.source.FindPersonNamesByIDs(ctx, personIDs)
	return personNames(names), err
}

// guardianEnrollments keeps the optional enrollment claim out of the flows:
// a composition without it reports nothing to claim.
func guardianEnrollments(source identityPorts.GuardianEnrollments) ports.GuardianEnrollments {
	if source == nil {
		return nil
	}
	return guardianEnrollmentClaims{source: source}
}

type guardianEnrollmentClaims struct {
	source identityPorts.GuardianEnrollments
}

// ClaimGuardianEnrollments translates the one outcome the acceptance must
// not treat as best-effort back into the flows' vocabulary.
func (c guardianEnrollmentClaims) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed, err := c.source.ClaimGuardianEnrollments(ctx, accountID, email)
	if err != nil && errors.Is(err, identityaccess.ErrTransactionUnusable) {
		return claimed, fmt.Errorf("%w: %w", domain.ErrTransactionUnusable, err)
	}
	return claimed, err
}

type guardianInvitationDelivery struct {
	source identityPorts.GuardianInvitationDelivery
}

func (d guardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	return d.source.InvitationExpiry(ctx)
}

func (d guardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	return d.source.SchoolName(ctx, tenantID)
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, schoolName string) (int64, bool) {
	return d.source.EnqueueInvitationEmail(ctx, identityaccess.GuardianInvitation(invitation), identityPorts.GuardianProfile(profile), schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile domain.GuardianProfile, schoolName string) (int64, bool) {
	return d.source.EnqueueExistingAccountEmail(ctx, identityPorts.GuardianProfile(profile), schoolName)
}

// --- engine methods -------------------------------------------------------

// The guardian invitation lifecycle answers with the invitation sentinels
// the school invitation flows share; invitationError falls through to the
// lifecycle mapping for the rest.
