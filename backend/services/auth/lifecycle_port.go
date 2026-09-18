package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// ErrAccountLifecycleUnavailable reports a service composed without the
// Identity & Access account-lifecycle port.
var ErrAccountLifecycleUnavailable = errors.New("account lifecycle is not composed")

// IsRowMissing reports the "no row" outcomes the retained repositories use:
// a bare or wrapped sql.ErrNoRows, the translated not-found sentinel and the
// guardian sentinels. The composition root classifies repository results
// with it when it binds the Identity & Access seams.
func IsRowMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, modelBase.ErrNotFound) ||
		errors.Is(err, userModels.ErrGuardianProfileNotFound) || errors.Is(err, userModels.ErrStudentGuardianNotFound) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}

// RoleFacts are the classification facts of a role the Identity & Access
// school identity decisions read: its tier (base_role) and, for system roles
// seeded before the column existed, its name.
type RoleFacts struct {
	ID       int64
	TenantID *int64
	Name     string
	IsSystem bool
	BaseRole *string
}

// StaffPreviewSession is the result of starting an admin staff-view preview
// (#2893): a short-lived, access-only JWT carrying the TARGET account's
// identity, roles, and permissions plus the read_only/acting_admin_id claims.
type StaffPreviewSession struct {
	AccessToken     string
	ExpiresIn       int64 // seconds until the access token expires
	TargetAccountID int64
	TargetName      string
}

// StaffPreviewCandidate is one selectable staff member for the preview
// picker. AccountID travels as a JSON string: an int64 ID must never pass
// through a JavaScript number (see api/auth.StaffPreviewStartRequest).
type StaffPreviewCandidate struct {
	AccountID int64    `json:"account_id,string"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
}

// ErrStaffOffboardingConflict reports an offboarding execution whose
// preview revision no longer matches the account's access.
var ErrStaffOffboardingConflict = errors.New("account access changed since offboarding preview")

// StaffOffboardingPreview contains no credential material. The revision is an
// exact encoding of the access decisions, not an authorization credential.
type StaffOffboardingPreview struct {
	AccountID         int64
	ActiveMembership  bool
	RoleIDs           []int64
	Permissions       int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
	Revision          string
}

type StaffOffboardingResult struct {
	RolesRevoked            int64
	PermissionsRevoked      int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

// SchoolIdentityInput describes the identity to provision: the person, staff
// and (for caregiver roles) teacher rows an account needs to be usable at a
// school (#2222).
type SchoolIdentityInput struct {
	AccountID int64
	TenantID  int64
	Role      *RoleFacts

	// FirstName/LastName are consulted only when no person exists yet.
	FirstName string
	LastName  string
	TagID     *string

	// PersonID names an existing, account-less person at this school the
	// account should be linked to before a new one would be created (#2600).
	PersonID *int64

	// Position fills users.teachers.role when a caregiver profile is created.
	Position string

	// CaregiverUpgrade provisions the caregiver profile for a role that would
	// not get one on its own.
	CaregiverUpgrade bool

	// CreatePerson allows the caller to refuse inventing an identity for an
	// account that has none at this school yet.
	CreatePerson bool
}

// SchoolIdentity is what the account carries at the school afterwards.
// TeacherID is zero for roles that run without a caregiver profile.
type SchoolIdentity struct {
	PersonID  int64
	StaffID   int64
	TeacherID int64
}

// SchoolIdentityProvisioning is the consumer-owned port over the Identity &
// Access school identity capability and school-role policy the retained
// registration, linking, invitation and operator provisioning flows call
// inside their tenant transactions (#3225, #3314).
type SchoolIdentityProvisioning interface {
	EnsureSchoolIdentity(ctx context.Context, input SchoolIdentityInput) (*SchoolIdentity, error)
	RoleNeedsStaffRecord(role *RoleFacts) bool
	RoleNeedsCaregiverProfile(role *RoleFacts) bool
	IsPlatformCaregiverRole(role *RoleFacts) bool
	// IsLehrkraftSystemRole reports whether the role is the platform
	// Lehrkraft role (#1772).
	IsLehrkraftSystemRole(role *RoleFacts) bool
	// ValidateAssignableSchoolRole returns the policy sentinel of this
	// package that refuses the role for tenantID, or nil.
	ValidateAssignableSchoolRole(role *RoleFacts, tenantID int64) error
	// HasLiveCaregiverProfile reports whether the account's identity at the
	// tenant in context carries a live caregiver profile.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
}

// ParentAccountRecord is one parent authentication account as the port
// reports it; the AuthService contract projects it onto the retained model.
type ParentAccountRecord struct {
	ID        int64
	TenantID  int64
	Email     string
	Username  string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func parentAccountModel(record ParentAccountRecord) *authModels.AccountParent {
	row := &authModels.AccountParent{Email: record.Email, Active: record.Active}
	row.ID = record.ID
	row.CreatedAt = record.CreatedAt
	row.UpdatedAt = record.UpdatedAt
	row.SetTenantID(record.TenantID)
	if record.Username != "" {
		username := record.Username
		row.Username = &username
	}
	return row
}

// InviteToStudentRequest is the unified "invite an email to a child" input,
// used by both the staff "Erziehungsberechtigte" tab and the parents portal.
// The handler resolves RequireApproval from the tenant setting and performs
// the caller-type authorization before calling this method.
type InviteToStudentRequest struct {
	StudentID        int64
	Email            string
	FirstName        string // optional, only used when creating a brand-new profile
	LastName         string // optional
	RelationshipType string // defaults to "guardian"
	CreatedBy        int64  // inviting account (staff or parent)

	// RequestedByParentAccountID is set (non-nil) when a parent initiated the
	// invite via the parents portal; nil for staff-initiated invites.
	RequestedByParentAccountID *int64
	// RequireApproval queues the invite for staff approval instead of acting
	// immediately. The handler decides this from guardians.parent_invite_mode.
	RequireApproval bool
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link (emergency_contact/pickup_only/custom) to legal_guardian. Without
	// it, such an invite returns InviteOutcomeExistingContactRestricted with
	// no side effects so the UI can ask first (#2172).
	ConfirmRoleUpgrade bool
}

// InviteToStudentOutcome describes what the resolve logic did, so the UI can
// show the right confirmation.
type InviteToStudentOutcome string

const (
	InviteOutcomeLinkedExistingAccount     InviteToStudentOutcome = "linked_existing_account"
	InviteOutcomeAlreadyLinked             InviteToStudentOutcome = "already_linked"
	InviteOutcomeInvited                   InviteToStudentOutcome = "invited"
	InviteOutcomePendingApproval           InviteToStudentOutcome = "pending_approval"
	InviteOutcomeExistingContactRestricted InviteToStudentOutcome = "existing_contact_restricted"
)

// InviteToStudentResult is returned by InviteToStudent.
type InviteToStudentResult struct {
	Outcome           InviteToStudentOutcome
	GuardianProfileID int64
	InvitationID      *int64 // nil for auto-link / already-linked outcomes
	// ExistingRole carries the current guardian role for the
	// existing_contact_restricted outcome; empty otherwise.
	ExistingRole string
}

// RevokeAccessRequest removes one account's access to one child by deleting
// the students_guardians link. The account, profile, and sibling links are
// left untouched.
type RevokeAccessRequest struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
	// ByParent is true for parents-portal removals. Parents may not remove the
	// primary guardian; staff may remove anyone.
	ByParent bool
	// MayClearPayer says whether the actor holds guardians:financial (#2608).
	MayClearPayer bool
}

// PendingApprovalView is the staff-facing, name-resolved projection of a
// parent-initiated invitation awaiting approval.
type PendingApprovalView struct {
	InvitationID      int64
	GuardianProfileID int64
	GuardianName      string
	GuardianEmail     string
	StudentID         int64
	StudentName       string
	RequestedByEmail  string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	// RoleUpgrade marks a request whose approval also upgrades an existing
	// restrictive contact link to full portal access (#2172).
	RoleUpgrade bool
}

// GuardianRelativeAccess is the consumer-owned port over the Identity &
// Access guardian relative access capability: invite further guardians to a
// child, decide parent-initiated requests, revoke an account's access.
type GuardianRelativeAccess interface {
	InviteToStudent(ctx context.Context, req InviteToStudentRequest) (*InviteToStudentResult, error)
	ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error
	RejectInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error
	PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error)
	ListPendingApprovalsDetailed(ctx context.Context) ([]*PendingApprovalView, error)
	RevokeAccess(ctx context.Context, req RevokeAccessRequest) error
}

// AccountLifecycle is the consumer-owned port over the Identity & Access
// account-lifecycle capability (#3225). The composition root binds it;
// every error already carries the AuthError envelope and the sentinels of
// this package.
type AccountLifecycle interface {
	SchoolIdentityProvisioning
	GuardianRelativeAccess

	StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*StaffPreviewSession, error)
	EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error)
	ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error)

	PreviewStaffOffboarding(ctx context.Context, accountID int64) (StaffOffboardingPreview, error)
	ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (StaffOffboardingResult, error)

	CreateParentAccount(ctx context.Context, email, username, password string) (ParentAccountRecord, error)
	GetParentAccountByID(ctx context.Context, id int64) (ParentAccountRecord, error)
	GetParentAccountByEmail(ctx context.Context, email string) (ParentAccountRecord, error)
	UpdateParentAccount(ctx context.Context, account ParentAccountRecord) error
	ActivateParentAccount(ctx context.Context, id int64) error
	DeactivateParentAccount(ctx context.Context, id int64) error
	ListParentAccounts(ctx context.Context, email string, active *bool) ([]ParentAccountRecord, error)
}

func (s *Service) accountLifecycle(op string) (AccountLifecycle, error) {
	if s.lifecycle == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountLifecycleUnavailable}
	}
	return s.lifecycle, nil
}

// The AuthService methods below delegate to the Identity & Access port so
// the retained consumers (auth routes, staff offboarding workflow, operator
// provisioning) keep their contract while the flows live in the owner module.

func (s *Service) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*StaffPreviewSession, error) {
	lifecycle, err := s.accountLifecycle("start staff preview")
	if err != nil {
		return nil, err
	}
	return lifecycle.StartStaffPreview(ctx, adminAccountID, tenantID, targetAccountID, previousToken, ipAddress, userAgent)
}

func (s *Service) EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error) {
	lifecycle, err := s.accountLifecycle("end staff preview")
	if err != nil {
		return 0, err
	}
	return lifecycle.EndStaffPreview(ctx, previewToken, ipAddress, userAgent)
}

func (s *Service) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error) {
	lifecycle, err := s.accountLifecycle("list staff preview candidates")
	if err != nil {
		return nil, err
	}
	return lifecycle.ListStaffPreviewCandidates(ctx, tenantID, excludeAccountID)
}

// PreviewStaffOffboarding snapshots the account's access at the school in
// context; ExecuteStaffOffboarding revokes it against that snapshot on the
// workflow's transaction.
func (s *Service) PreviewStaffOffboarding(ctx context.Context, accountID int64) (StaffOffboardingPreview, error) {
	lifecycle, err := s.accountLifecycle("staff offboarding preview")
	if err != nil {
		return StaffOffboardingPreview{}, err
	}
	return lifecycle.PreviewStaffOffboarding(ctx, accountID)
}

func (s *Service) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (StaffOffboardingResult, error) {
	lifecycle, err := s.accountLifecycle("staff offboarding")
	if err != nil {
		return StaffOffboardingResult{}, err
	}
	return lifecycle.ExecuteStaffOffboarding(ctx, accountID, revision)
}

func (s *Service) CreateParentAccount(ctx context.Context, email, username, password string) (*authModels.AccountParent, error) {
	lifecycle, err := s.accountLifecycle(opCreateParentAccount)
	if err != nil {
		return nil, err
	}
	record, err := lifecycle.CreateParentAccount(ctx, email, username, password)
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *Service) GetParentAccountByID(ctx context.Context, id int) (*authModels.AccountParent, error) {
	lifecycle, err := s.accountLifecycle("get parent account")
	if err != nil {
		return nil, err
	}
	record, err := lifecycle.GetParentAccountByID(ctx, int64(id))
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *Service) GetParentAccountByEmail(ctx context.Context, email string) (*authModels.AccountParent, error) {
	lifecycle, err := s.accountLifecycle("get parent account by email")
	if err != nil {
		return nil, err
	}
	record, err := lifecycle.GetParentAccountByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return parentAccountModel(record), nil
}

func (s *Service) UpdateParentAccount(ctx context.Context, account *authModels.AccountParent) error {
	lifecycle, err := s.accountLifecycle("update parent account")
	if err != nil {
		return err
	}
	if account == nil {
		return &AuthError{Op: "update parent account", Err: ErrParentAccountNotFound}
	}
	record := ParentAccountRecord{ID: account.ID, TenantID: account.TenantID, Email: account.Email, Active: account.Active}
	if account.Username != nil {
		record.Username = *account.Username
	}
	return lifecycle.UpdateParentAccount(ctx, record)
}

func (s *Service) ActivateParentAccount(ctx context.Context, accountID int) error {
	lifecycle, err := s.accountLifecycle("activate parent account")
	if err != nil {
		return err
	}
	return lifecycle.ActivateParentAccount(ctx, int64(accountID))
}

func (s *Service) DeactivateParentAccount(ctx context.Context, accountID int) error {
	lifecycle, err := s.accountLifecycle("deactivate parent account")
	if err != nil {
		return err
	}
	return lifecycle.DeactivateParentAccount(ctx, int64(accountID))
}

func (s *Service) ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*authModels.AccountParent, error) {
	lifecycle, err := s.accountLifecycle("list parent accounts")
	if err != nil {
		return nil, err
	}
	var email string
	if value, ok := filters["email"].(string); ok {
		email = value
	}
	var active *bool
	if value, ok := filters["active"].(bool); ok {
		active = &value
	}
	records, err := lifecycle.ListParentAccounts(ctx, email, active)
	if err != nil {
		return nil, err
	}
	accounts := make([]*authModels.AccountParent, 0, len(records))
	for _, record := range records {
		accounts = append(accounts, parentAccountModel(record))
	}
	return accounts, nil
}

// Account lifecycle outcomes the retained handlers switch on; the
// Identity & Access module reports them and the composition translates
// them back (#3225).
var (
	// ErrParentAccountNotFound returned when parent account doesn't exist
	ErrParentAccountNotFound = errors.New("parent account not found")

	// Admin staff-view preview (#2893)
	// ErrPreviewSelf: previewing your own account is pointless and refused.
	ErrPreviewSelf = errors.New("cannot preview your own account")
	// ErrPreviewTargetNotStaff: the target has no tenant-portal surface at
	// this school (guardian-only, or no role at all).
	ErrPreviewTargetNotStaff = errors.New("account is not a staff member at this school")
	// ErrPreviewTokenInvalid: the token presented when ending a preview is
	// not a valid preview token of the calling admin at this school. The end
	// call proves which preview it closes with that token, so a client cannot
	// name an arbitrary account in the audit trail.
	ErrPreviewTokenInvalid = errors.New("not a preview token of this session")

	// School identity (#2222): the request errors the Identity & Access
	// provisioning reports and the handlers render as 400. German because the
	// operator and admin screens show them verbatim.
	ErrSchoolIdentityNamesRequired   = errors.New("Vor- und Nachname sind erforderlich, um ein Konto als Personal anzulegen")                                                //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityPersonIsStudent = errors.New("Dieses Konto ist mit dem Datensatz eines Kindes verknüpft und kann nicht als Personal angelegt werden")                   //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagUnknown      = errors.New("Der angegebene Transponder ist an dieser Schule nicht bekannt")                                                           //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagConflict     = errors.New("Diese Person trägt an dieser Schule bereits einen anderen Transponder; dieser wird über die Personalverwaltung geändert") //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagTaken        = errors.New("Dieser Transponder ist an dieser Schule bereits einer anderen Person zugeordnet")                                         //nolint:staticcheck // ST1005: user-facing German message

	// Related-accounts errors
	ErrCannotRemovePrimaryGuardian      = errors.New("the primary guardian cannot be removed by a parent")
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	ErrCannotRemoveOwnAccess            = errors.New("a parent cannot remove their own access to a child")
	// ErrCannotRemovePayerGuardian: the link carries the child's payer mark
	// (#2608). Clearing it is a financial decision that needs
	// guardians:financial; the parents portal never holds that, so the payer
	// stays until the school reassigns the payment account. The sentence is
	// German because the parents portal shows it verbatim.
	ErrCannotRemovePayerGuardian = errors.New("Diese Person ist als Zahler für das Kind eingetragen und kann nicht entfernt werden. Bitte wenden Sie sich an die Schule.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrInviteSocialWorkerManaged: the invited email belongs to a contact
	// linked to this child as a social worker. That is a school-managed
	// professional contact — the invite flow must never turn it into a legal
	// guardian, so the invite is refused entirely (mirrors
	// ErrGuardianSocialWorkerManaged on the parent edit paths).
	ErrInviteSocialWorkerManaged = errors.New("a social-worker contact is managed by the school and cannot be invited to the parents portal")
)

// StaffPreviewOperations delegate the admin staff-view preview (#2893) to
// Identity & Access.
type StaffPreviewOperations interface {
	// Admin staff-view preview (#2893): a read-only, access-only token that
	// sees the tenant portal exactly as the target staff member. Start mints
	// (and re-mints) the token, End records the audit trail, the candidate
	// list feeds the picker. The route layer restricts all three to
	// effective admins. End is given the preview token it closes and reads
	// the previewed account from it, so the audit trail cannot be stamped
	// with a preview that never happened. Start takes the token the client
	// currently holds so a re-mint continues the running preview instead of
	// opening a second one in the audit trail.
	StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*StaffPreviewSession, error)
	EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error)
	ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error)
}

// ParentAccountOperations delegate parent account management to Identity &
// Access.
type ParentAccountOperations interface {
	// Parent Account Management
	CreateParentAccount(ctx context.Context, email, username, password string) (*authModels.AccountParent, error)
	GetParentAccountByID(ctx context.Context, id int) (*authModels.AccountParent, error)
	GetParentAccountByEmail(ctx context.Context, email string) (*authModels.AccountParent, error)
	UpdateParentAccount(ctx context.Context, account *authModels.AccountParent) error
	ActivateParentAccount(ctx context.Context, accountID int) error
	DeactivateParentAccount(ctx context.Context, accountID int) error
	ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*authModels.AccountParent, error)
}
