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
)

// The account lifecycle flows (#3225) and the role administration (#3314)
// need facts other owners hold and seams the composition root binds: persons,
// staff and caregiver profiles, guardian profiles and relationships, the
// audit evidence, the password hasher, the retained role storage and
// account management and the retained guardian invitation delivery. The
// seams below are expressed in public values; this package adapts them to the
// consumer-owned ports.

// StaffMember is the users.staff fact of a school.
type StaffMember struct {
	ID       int64
	TenantID int64
	PersonID int64
	Deleted  bool
}

// PersonRecord is the users.persons fact the lifecycle reads and, for the
// school identity chain, creates.
type PersonRecord struct {
	ID        int64
	TenantID  int64
	AccountID *int64
	FirstName string
	LastName  string
	TagID     *string
	Deleted   bool
}

// PersonName is a person's display name.
type PersonName struct {
	FirstName string
	LastName  string
}

// CaregiverProfile is the users.teachers fact of a staff member.
type CaregiverProfile struct {
	ID      int64
	StaffID int64
	Deleted bool
}

// StaffDirectory reads and writes the People Directory and School Membership
// facts of the tenant in context. Missing rows report found=false.
type StaffDirectory interface {
	FindStaff(ctx context.Context, staffID int64) (StaffMember, bool, error)
	FindPerson(ctx context.Context, personID int64) (PersonRecord, bool, error)
	FindPersonByAccount(ctx context.Context, accountID int64) (PersonRecord, bool, error)
	FindPersonByTag(ctx context.Context, tagID string) (PersonRecord, bool, error)
	FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]PersonName, error)
	CreatePerson(ctx context.Context, person PersonRecord) (int64, error)
	LinkPersonToAccount(ctx context.Context, personID, accountID int64) error
	LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error
	IsStudentPerson(ctx context.Context, personID int64) (bool, error)
	FindStaffByPerson(ctx context.Context, personID int64) (StaffMember, bool, error)
	CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error)
	FindCaregiverProfile(ctx context.Context, staffID int64) (CaregiverProfile, bool, error)
	CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error)
	// HasLiveCaregiverProfile reports whether the account's live person at the
	// school carries a live staff record with a live caregiver profile.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
}

// StaffPreviewEvent is the audit evidence of one preview start or end.
type StaffPreviewEvent struct {
	AdminAccountID  int64
	TenantID        int64
	TargetAccountID int64
	PreviewID       string
	IPAddress       string
	UserAgent       string
}

// PreviewAudit records the staff preview evidence on the caller's
// transaction; the lock is transaction-scoped.
type PreviewAudit interface {
	RecordStaffPreviewStart(ctx context.Context, event StaffPreviewEvent) error
	RecordStaffPreviewEndOnce(ctx context.Context, event StaffPreviewEvent) (bool, error)
	LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error
	StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error)
}

// PasswordPolicy validates and hashes parent account passwords.
type PasswordPolicy interface {
	ValidatePasswordStrength(password string) error
	HashPassword(password string) (string, error)
}

// GuardianProfile is the users.guardian_profiles fact.
type GuardianProfile struct {
	ID         int64
	TenantID   int64
	FirstName  string
	LastName   string
	Email      string
	AccountID  *int64
	HasAccount bool
}

// StudentGuardianLink is one users.students_guardians row.
type StudentGuardianLink struct {
	ID                int64
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	RelationshipType  string
	GuardianRole      string
	IsPrimary         bool
	IsPayer           bool
	EmergencyPriority int
}

// Student is the users.students fact the approval queue resolves names through.
type Student struct {
	ID       int64
	PersonID int64
}

// GuardianRoleClass classifies a stored guardian role for the invite flow.
type GuardianRoleClass int

const (
	GuardianRoleRestricted GuardianRoleClass = iota
	GuardianRoleFull
	GuardianRoleSocialWorker
)

// GuardianDirectory reads and writes guardian profiles, student-guardian
// relationships and students of the tenant in context.
type GuardianDirectory interface {
	FindGuardianProfileByEmail(ctx context.Context, email string) (GuardianProfile, bool, error)
	FindGuardianProfile(ctx context.Context, id int64) (GuardianProfile, bool, error)
	FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]GuardianProfile, error)
	CreateGuardianProfile(ctx context.Context, profile GuardianProfile) (int64, error)
	DeleteGuardianProfile(ctx context.Context, id int64) error
	LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error
	FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (StudentGuardianLink, bool, error)
	LinkStudentGuardianIfAbsent(ctx context.Context, link StudentGuardianLink) (bool, error)
	PromoteStudentGuardianLink(ctx context.Context, linkID int64) error
	ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]StudentGuardianLink, error)
	ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]StudentGuardianLink, error)
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
	GuardianRoleClass(role string) GuardianRoleClass
	FindStudents(ctx context.Context, ids []int64) (map[int64]Student, error)
	LockStudent(ctx context.Context, studentID int64) error
	FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]PersonName, error)
}

// GuardianEnrollments claims the enrollment requests a guardian filed
// before they had an account, on the caller's transaction.
type GuardianEnrollments interface {
	ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error)
}

// GuardianInvitationDelivery mails a guardian invitation: the token expiry
// the tenant configured, the school name for the mail and the outbox
// enqueue the worker dispatches from.
type GuardianInvitationDelivery interface {
	InvitationExpiry(ctx context.Context) time.Duration
	SchoolName(ctx context.Context, tenantID int64) string
	EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile GuardianProfile, schoolName string)
	EnqueueExistingAccountEmail(ctx context.Context, profile GuardianProfile, schoolName string)
}

// FinancialAudit records the removal of a child's payer.
type FinancialAudit interface {
	RecordPayerRemoved(ctx context.Context, guardianProfileID, studentID, actorAccountID int64) error
}

// LifecycleDependencies are the seams the account lifecycle flows need
// beyond the database and the session dependencies.
type LifecycleDependencies struct {
	Staff     StaffDirectory
	Audit     PreviewAudit
	Passwords PasswordPolicy
	Guardians GuardianDirectory
	Delivery  GuardianInvitationDelivery
	// Enrollments is optional: without it an acceptance claims no
	// pre-account enrollment requests.
	Enrollments GuardianEnrollments
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

type staffDirectory struct{ source StaffDirectory }

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

func personNames(names map[int64]PersonName) map[int64]domain.PersonName {
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
	return d.source.CreatePerson(ctx, PersonRecord(person))
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

type previewAudit struct{ source PreviewAudit }

func (a previewAudit) RecordStaffPreviewStart(ctx context.Context, event domain.StaffPreviewEvent) error {
	return a.source.RecordStaffPreviewStart(ctx, StaffPreviewEvent(event))
}

func (a previewAudit) RecordStaffPreviewEndOnce(ctx context.Context, event domain.StaffPreviewEvent) (bool, error) {
	return a.source.RecordStaffPreviewEndOnce(ctx, StaffPreviewEvent(event))
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

type guardianDirectory struct{ source GuardianDirectory }

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
	return d.source.CreateGuardianProfile(ctx, GuardianProfile(profile))
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
	return d.source.LinkStudentGuardianIfAbsent(ctx, StudentGuardianLink(link))
}

func (d guardianDirectory) PromoteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return d.source.PromoteStudentGuardianLink(ctx, linkID)
}

func studentGuardianLinks(links []StudentGuardianLink) []domain.StudentGuardianLink {
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
func guardianEnrollments(source GuardianEnrollments) ports.GuardianEnrollments {
	if source == nil {
		return nil
	}
	return guardianEnrollmentClaims{source: source}
}

type guardianEnrollmentClaims struct{ source GuardianEnrollments }

// ClaimGuardianEnrollments translates the one outcome the acceptance must
// not treat as best-effort back into the flows' vocabulary.
func (c guardianEnrollmentClaims) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed, err := c.source.ClaimGuardianEnrollments(ctx, accountID, email)
	if err != nil && errors.Is(err, identityaccess.ErrTransactionUnusable) {
		return claimed, fmt.Errorf("%w: %w", domain.ErrTransactionUnusable, err)
	}
	return claimed, err
}

type guardianInvitationDelivery struct{ source GuardianInvitationDelivery }

func (d guardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	return d.source.InvitationExpiry(ctx)
}

func (d guardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	return d.source.SchoolName(ctx, tenantID)
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation domain.GuardianInvitation, profile domain.GuardianProfile, schoolName string) {
	d.source.EnqueueInvitationEmail(ctx, identityaccess.GuardianInvitation(invitation), GuardianProfile(profile), schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile domain.GuardianProfile, schoolName string) {
	d.source.EnqueueExistingAccountEmail(ctx, GuardianProfile(profile), schoolName)
}

// --- engine methods -------------------------------------------------------

var errAccountLifecycleUnavailable = identityaccess.ErrAccountLifecycleUnavailable

func roleFacts(role *identityaccess.RoleFacts) *domain.RoleFacts {
	if role == nil {
		return nil
	}
	facts := domain.RoleFacts(*role)
	return &facts
}

func (e engine) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*identityaccess.StaffPreviewSession, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	session, err := e.lifecycle.StartStaffPreview(e.attach(ctx), adminAccountID, tenantID, targetAccountID, previousToken, ipAddress, userAgent)
	if err != nil {
		return nil, lifecycleError(err)
	}
	result := identityaccess.StaffPreviewSession(*session)
	return &result, nil
}

func (e engine) EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error) {
	if e.lifecycle == nil {
		return 0, errAccountLifecycleUnavailable
	}
	accountID, err := e.lifecycle.EndStaffPreview(e.attach(ctx), previewToken, ipAddress, userAgent)
	return accountID, lifecycleError(err)
}

func (e engine) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]identityaccess.StaffPreviewCandidate, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	candidates, err := e.lifecycle.ListStaffPreviewCandidates(e.attach(ctx), tenantID, excludeAccountID)
	if err != nil {
		return nil, lifecycleError(err)
	}
	result := make([]identityaccess.StaffPreviewCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, identityaccess.StaffPreviewCandidate(candidate))
	}
	return result, nil
}

func (e engine) PreviewStaffOffboarding(ctx context.Context, accountID int64) (identityaccess.StaffOffboardingPreview, error) {
	if e.lifecycle == nil {
		return identityaccess.StaffOffboardingPreview{}, errAccountLifecycleUnavailable
	}
	preview, err := e.lifecycle.PreviewStaffOffboarding(e.attach(ctx), accountID)
	return identityaccess.StaffOffboardingPreview(preview), lifecycleError(err)
}

func (e engine) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (identityaccess.StaffOffboardingResult, error) {
	if e.lifecycle == nil {
		return identityaccess.StaffOffboardingResult{}, errAccountLifecycleUnavailable
	}
	result, err := e.lifecycle.ExecuteStaffOffboarding(e.attach(ctx), accountID, revision)
	return identityaccess.StaffOffboardingResult(result), lifecycleError(err)
}

func (e engine) EnsureSchoolIdentity(ctx context.Context, input identityaccess.SchoolIdentityInput) (*identityaccess.SchoolIdentity, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	identity, err := e.lifecycle.EnsureSchoolIdentity(e.attach(ctx), domain.SchoolIdentityInput{
		AccountID: input.AccountID, TenantID: input.TenantID, Role: roleFacts(input.Role),
		FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID, PersonID: input.PersonID,
		Position: input.Position, CaregiverUpgrade: input.CaregiverUpgrade, CreatePerson: input.CreatePerson,
	})
	if err != nil {
		return nil, lifecycleError(err)
	}
	if identity == nil {
		return nil, nil
	}
	result := identityaccess.SchoolIdentity(*identity)
	return &result, nil
}

func (e engine) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	if e.lifecycle == nil {
		return false, errAccountLifecycleUnavailable
	}
	hasProfile, err := e.lifecycle.HasLiveCaregiverProfile(e.attach(ctx), accountID)
	return hasProfile, lifecycleError(err)
}

func parentAccount(value domain.ParentAccount) identityaccess.ParentAccount {
	return identityaccess.ParentAccount{
		ID: value.ID, TenantID: value.TenantID, Email: value.Email, Username: value.Username, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (e engine) CreateParentAccount(ctx context.Context, email, username, password string) (identityaccess.ParentAccount, error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	account, err := e.lifecycle.CreateParentAccount(e.attach(ctx), email, username, password)
	return parentAccount(account), lifecycleError(err)
}

func (e engine) GetParentAccountByID(ctx context.Context, id int64) (identityaccess.ParentAccount, error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	account, err := e.lifecycle.GetParentAccountByID(e.attach(ctx), id)
	return parentAccount(account), lifecycleError(err)
}

func (e engine) GetParentAccountByEmail(ctx context.Context, email string) (identityaccess.ParentAccount, error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	account, err := e.lifecycle.GetParentAccountByEmail(e.attach(ctx), email)
	return parentAccount(account), lifecycleError(err)
}

func (e engine) UpdateParentAccount(ctx context.Context, account identityaccess.ParentAccount) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.UpdateParentAccount(e.attach(ctx), domain.ParentAccount{
		ID: account.ID, TenantID: account.TenantID, Email: account.Email, Username: account.Username, Active: account.Active,
	}))
}

func (e engine) ActivateParentAccount(ctx context.Context, id int64) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.ActivateParentAccount(e.attach(ctx), id))
}

func (e engine) DeactivateParentAccount(ctx context.Context, id int64) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.DeactivateParentAccount(e.attach(ctx), id))
}

func (e engine) ListParentAccounts(ctx context.Context, filter identityaccess.ParentAccountFilter) ([]identityaccess.ParentAccount, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	accounts, err := e.lifecycle.ListParentAccounts(e.attach(ctx), domain.ParentAccountFilter(filter))
	if err != nil {
		return nil, lifecycleError(err)
	}
	result := make([]identityaccess.ParentAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, parentAccount(account))
	}
	return result, nil
}

func (e engine) InviteToStudent(ctx context.Context, req identityaccess.InviteToStudentRequest) (*identityaccess.InviteToStudentResult, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	result, err := e.lifecycle.InviteToStudent(e.attach(ctx), domain.InviteToStudentRequest(req))
	if err != nil {
		return nil, lifecycleError(err)
	}
	public := identityaccess.InviteToStudentResult{
		Outcome: identityaccess.InviteToStudentOutcome(result.Outcome), GuardianProfileID: result.GuardianProfileID,
		InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
	}
	return &public, nil
}

func (e engine) ApproveInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.ApproveInvitation(e.attach(ctx), invitationID, approverAccountID))
}

func (e engine) RejectInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.RejectInvitation(e.attach(ctx), invitationID, approverAccountID))
}

// The guardian invitation lifecycle answers with the invitation sentinels
// the school invitation flows share; invitationError falls through to the
// lifecycle mapping for the rest.

func (e engine) CreateGuardianInvitation(ctx context.Context, request identityaccess.GuardianInvitationRequest) (identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return identityaccess.GuardianInvitation{}, errAccountLifecycleUnavailable
	}
	invitation, err := e.lifecycle.CreateGuardianInvitation(e.attach(ctx), domain.GuardianInvitationRequest(request))
	if err != nil {
		return identityaccess.GuardianInvitation{}, invitationError(err)
	}
	return identityaccess.GuardianInvitation(invitation), nil
}

func (e engine) ValidateGuardianInvitation(ctx context.Context, token string) (identityaccess.GuardianInvitationPreview, error) {
	if e.lifecycle == nil {
		return identityaccess.GuardianInvitationPreview{}, errAccountLifecycleUnavailable
	}
	preview, err := e.lifecycle.ValidateGuardianInvitation(e.attach(ctx), token)
	if err != nil {
		return identityaccess.GuardianInvitationPreview{}, invitationError(err)
	}
	return identityaccess.GuardianInvitationPreview(preview), nil
}

func (e engine) AcceptGuardianInvitation(ctx context.Context, token string, registration identityaccess.GuardianRegistration) (identityaccess.Account, error) {
	if e.lifecycle == nil {
		return identityaccess.Account{}, errAccountLifecycleUnavailable
	}
	account, err := e.lifecycle.AcceptGuardianInvitation(e.attach(ctx), token, domain.GuardianRegistration(registration))
	if err != nil {
		return identityaccess.Account{}, invitationError(err)
	}
	return identityaccess.Account{ID: account.ID, Email: account.Email}, nil
}

func (e engine) ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return invitationError(e.lifecycle.ResendGuardianInvitation(e.attach(ctx), invitationID, actorAccountID))
}

func (e engine) ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	invitations, err := e.lifecycle.ListGuardianInvitations(e.attach(ctx), guardianProfileID)
	return publicGuardianInvitations(invitations), invitationError(err)
}

func (e engine) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	invitations, err := e.lifecycle.ListOpenGuardianInvitations(e.attach(ctx), guardianProfileIDs)
	return publicGuardianInvitations(invitations), invitationError(err)
}

func (e engine) ListRedeemableGuardianInvitations(ctx context.Context) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	invitations, err := e.lifecycle.ListRedeemableGuardianInvitations(e.attach(ctx))
	return publicGuardianInvitations(invitations), invitationError(err)
}

func publicGuardianInvitations(invitations []domain.GuardianInvitation) []identityaccess.GuardianInvitation {
	if invitations == nil {
		return nil
	}
	result := make([]identityaccess.GuardianInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, identityaccess.GuardianInvitation(invitation))
	}
	return result
}

func (e engine) GuardianInvitationSchoolSlug(ctx context.Context, token string) string {
	if e.lifecycle == nil {
		return ""
	}
	return e.lifecycle.GuardianInvitationSchoolSlug(e.attach(ctx), token)
}

func (e engine) PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error) {
	if e.lifecycle == nil {
		return 0, errAccountLifecycleUnavailable
	}
	studentID, err := e.lifecycle.PendingInvitationStudentID(e.attach(ctx), invitationID)
	return studentID, lifecycleError(err)
}

func (e engine) ListPendingApprovalsDetailed(ctx context.Context) ([]identityaccess.PendingApprovalView, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	views, err := e.lifecycle.ListPendingApprovalsDetailed(e.attach(ctx))
	if err != nil {
		return nil, lifecycleError(err)
	}
	result := make([]identityaccess.PendingApprovalView, 0, len(views))
	for _, view := range views {
		result = append(result, identityaccess.PendingApprovalView(view))
	}
	return result, nil
}

func (e engine) RevokeAccess(ctx context.Context, req identityaccess.RevokeAccessRequest) error {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	return lifecycleError(e.lifecycle.RevokeAccess(e.attach(ctx), domain.RevokeAccessRequest(req)))
}

var lifecycleSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrPreviewSelf, identityaccess.ErrPreviewSelf},
	{domain.ErrPreviewTargetNotStaff, identityaccess.ErrPreviewTargetNotStaff},
	{domain.ErrPreviewTokenInvalid, identityaccess.ErrPreviewTokenInvalid},
	{domain.ErrStaffOffboardingConflict, identityaccess.ErrStaffOffboardingConflict},
	{domain.ErrSchoolIdentityNamesRequired, identityaccess.ErrSchoolIdentityNamesRequired},
	{domain.ErrSchoolIdentityPersonIsStudent, identityaccess.ErrSchoolIdentityPersonIsStudent},
	{domain.ErrSchoolIdentityTagUnknown, identityaccess.ErrSchoolIdentityTagUnknown},
	{domain.ErrSchoolIdentityTagConflict, identityaccess.ErrSchoolIdentityTagConflict},
	{domain.ErrSchoolIdentityTagTaken, identityaccess.ErrSchoolIdentityTagTaken},
	{domain.ErrParentAccountNotFound, identityaccess.ErrParentAccountNotFound},
	{domain.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{domain.ErrUsernameAlreadyExists, identityaccess.ErrUsernameAlreadyExists},
	{domain.ErrCannotRemovePrimaryGuardian, identityaccess.ErrCannotRemovePrimaryGuardian},
	{domain.ErrCannotRemoveStaffManagedGuardian, identityaccess.ErrCannotRemoveStaffManagedGuardian},
	{domain.ErrCannotRemoveOwnAccess, identityaccess.ErrCannotRemoveOwnAccess},
	{domain.ErrCannotRemovePayerGuardian, identityaccess.ErrCannotRemovePayerGuardian},
	{domain.ErrInviteSocialWorkerManaged, identityaccess.ErrInviteSocialWorkerManaged},
	{domain.ErrGuardianInvitationNotFound, identityaccess.ErrGuardianInvitationNotFound},
	{domain.ErrGuardianInvitationExpired, identityaccess.ErrGuardianInvitationExpired},
}
