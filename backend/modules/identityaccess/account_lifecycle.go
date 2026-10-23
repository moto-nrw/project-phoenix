package identityaccess

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Account lifecycle (#3225): the admin staff-view preview, staff offboarding,
// the school identity chain a personnel role requires, parent accounts and
// guardian relative access. The error messages are the wire contract the
// retained auth service established; the HTTP layers switch on the sentinels.
var (
	ErrPreviewSelf           = errors.New("cannot preview your own account")
	ErrPreviewTargetNotStaff = errors.New("account is not a staff member at this school")
	ErrPreviewTokenInvalid   = errors.New("not a preview token of this session")

	ErrStaffOffboardingConflict = errors.New("account access changed since offboarding preview")

	ErrSchoolIdentityNamesRequired   = errors.New("vor- und Nachname sind erforderlich, um ein Konto als Personal anzulegen")
	ErrSchoolIdentityPersonIsStudent = errors.New("dieses Konto ist mit dem Datensatz eines Kindes verknüpft und kann nicht als Personal angelegt werden")
	ErrSchoolIdentityTagUnknown      = errors.New("der angegebene Transponder ist an dieser Schule nicht bekannt")
	ErrSchoolIdentityTagConflict     = errors.New("diese Person trägt an dieser Schule bereits einen anderen Transponder; dieser wird über die Personalverwaltung geändert")
	ErrSchoolIdentityTagTaken        = errors.New("dieser Transponder ist an dieser Schule bereits einer anderen Person zugeordnet")

	ErrEmailAlreadyExists    = errors.New("diese E-Mail-Adresse ist bereits registriert")
	ErrUsernameAlreadyExists = errors.New("dieser Benutzername ist bereits vergeben")

	// ErrPasswordTooWeak reports a credential that does not meet the
	// complexity requirements Security Runtime owns. The module reports it
	// wherever it accepts a password: registration, the invitation
	// acceptances, the password reset and the self-service change.
	ErrPasswordTooWeak = errors.New("password doesn't meet complexity requirements")

	ErrCannotRemovePrimaryGuardian      = errors.New("the primary guardian cannot be removed by a parent")
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	ErrCannotRemoveOwnAccess            = errors.New("a parent cannot remove their own access to a child")
	ErrCannotRemovePayerGuardian        = errors.New("diese Person ist als Zahler für das Kind eingetragen und kann nicht entfernt werden. Bitte wenden Sie sich an die Schule")
	ErrInviteSocialWorkerManaged        = errors.New("a social-worker contact is managed by the school and cannot be invited to the parents portal")
	ErrGuardianInvitationNotFound       = errors.New("invitation not found")
	ErrGuardianInvitationExpired        = errors.New("invitation has expired")

	// ErrAccountLifecycleUnavailable reports a module composed without the
	// account-lifecycle dependencies (repository fixtures, CLI roots that
	// only read).
	ErrAccountLifecycleUnavailable = errors.New("account lifecycle is not composed")
)

// GuardianInvitationValidationError marks input rejected by the guardian
// invitation flows while retaining the original caller-facing explanation.
type GuardianInvitationValidationError struct{ Err error }

func (e *GuardianInvitationValidationError) Error() string { return e.Err.Error() }
func (e *GuardianInvitationValidationError) Unwrap() error { return e.Err }

// IsSchoolIdentityRequestError reports whether the error is the caller's
// fault rather than the server's: a missing name, a child's record, an
// unknown or conflicting transponder. Handlers render these as 400.
func IsSchoolIdentityRequestError(err error) bool {
	return errors.Is(err, ErrSchoolIdentityNamesRequired) ||
		errors.Is(err, ErrSchoolIdentityPersonIsStudent) ||
		errors.Is(err, ErrSchoolIdentityTagUnknown) ||
		errors.Is(err, ErrSchoolIdentityTagConflict) ||
		errors.Is(err, ErrSchoolIdentityTagTaken)
}

// StaffPreviewSession is the result of starting an admin staff-view preview
// (#2893): a short-lived, access-only JWT carrying the TARGET account's
// identity plus the read_only/acting_admin_id claims.
type StaffPreviewSession struct {
	AccessToken     string
	ExpiresIn       int64
	TargetAccountID int64
	TargetName      string
}

// StaffPreviewCandidate is one selectable staff member for the preview
// picker. AccountID travels as a JSON string: an int64 ID must never pass
// through a JavaScript number.
type StaffPreviewCandidate struct {
	AccountID int64    `json:"account_id,string"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
}

// StaffPreview is the capability the admin staff-view routes consume. The
// route layer restricts callers to effective admins.
type StaffPreview interface {
	StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*StaffPreviewSession, error)
	EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error)
	ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error)
}

// StaffOffboardingPreview contains no credential material. The revision is
// an exact encoding of the access decisions, not an authorization credential.
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

// StaffOffboardingAccess is the access step of the staff offboarding
// workflow: the preview snapshots roles, permissions, sessions and membership
// at the school in context; the execution revokes them against that
// snapshot on the caller's transaction.
type StaffOffboardingAccess interface {
	PreviewStaffOffboarding(ctx context.Context, accountID int64) (StaffOffboardingPreview, error)
	ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (StaffOffboardingResult, error)
}

// RoleFacts are the classification facts of a role: its tier (base_role)
// and, for system roles seeded before the column existed, its name.
type RoleFacts struct {
	ID       int64
	TenantID *int64
	Name     string
	IsSystem bool
	BaseRole *string
}

// Role tiers and the system role names the classification falls back to.
const (
	BaseRoleAdmin    = "admin"
	BaseRoleUser     = "user"
	BaseRoleGuardian = "guardian"

	// GuestRoleName is the system role „Gast“: the access level „Nur
	// ansehen“ (ADR 0085), children, rooms and AGs without parent data.
	GuestRoleName = "guest"
	// AGLeitungRoleName is the system role „AG-Leitung“ (migration
	// 1.15.475, ADR 0085): base role user, only schedules:read, runs the
	// own blocks. The role does not make anyone external.
	AGLeitungRoleName = "ag-leitung"

	lehrkraftRoleName     = "lehrkraft"
	legacyTeacherRoleName = "teacher"
)

// EffectiveBaseRole returns the privilege tier a role hands out. Custom roles
// carry base_role explicitly; system roles predate the column and are
// identified by name. Returns "" when the tier cannot be determined.
func EffectiveBaseRole(role *RoleFacts) string {
	if role == nil {
		return ""
	}
	if role.BaseRole != nil {
		if base := strings.ToLower(strings.TrimSpace(*role.BaseRole)); base != "" {
			return base
		}
	}
	if role.IsSystem {
		switch name := strings.ToLower(strings.TrimSpace(role.Name)); name {
		case BaseRoleAdmin, BaseRoleUser, BaseRoleGuardian:
			return name
		}
	}
	return ""
}

// IsLehrkraftSystemRole reports whether the role is the platform Lehrkraft
// role (#1772): class_day-read-only by design, never a caregiver.
func IsLehrkraftSystemRole(role *RoleFacts) bool {
	return role != nil && role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), lehrkraftRoleName)
}

// RoleNeedsStaffRecord reports whether an account holding this role at a
// school must carry a users.staff row. Everything that is not a guardian is
// personnel; an unknown tier counts as personnel on purpose. This decision
// fails OPEN: a staff row grants no permission, it is a directory entry, and
// withholding it is what breaks the account.
func RoleNeedsStaffRecord(role *RoleFacts) bool {
	if role == nil {
		return false
	}
	return !strings.EqualFold(EffectiveBaseRole(role), BaseRoleGuardian)
}

// RoleNeedsCaregiverProfile reports whether a role only works on an account
// that also carries a caregiver profile (users.teachers). Decided by tier, so
// a school's own role with base_role 'user' gets the same profile the
// platform user role gets (#2222). The Lehrkraft role never does (#1772); the
// retired teacher role stays a name match narrowed to system roles.
func RoleNeedsCaregiverProfile(role *RoleFacts) bool {
	if role == nil {
		return false
	}
	if IsLehrkraftSystemRole(role) {
		return false
	}
	if strings.EqualFold(EffectiveBaseRole(role), BaseRoleUser) {
		return true
	}
	return role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), legacyTeacherRoleName)
}

// IsPlatformCaregiverRole reports whether the role IS the platform caregiver
// role (or its retired predecessor), as opposed to merely being of that
// tier: base_role classifies, it does not grant.
func IsPlatformCaregiverRole(role *RoleFacts) bool {
	if role == nil || !role.IsSystem {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(role.Name)) {
	case BaseRoleUser, legacyTeacherRoleName:
		return true
	default:
		return false
	}
}

// SchoolIdentityInput describes the identity to provision for an account at
// the school in context.
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

	// Position fills the caregiver profile's role when one is created.
	Position string

	// CaregiverUpgrade provisions the caregiver profile for a role that
	// would not get one on its own.
	CaregiverUpgrade bool

	// CreatePerson allows the caller to refuse inventing an identity for an
	// account that has none at this school yet.
	CreatePerson bool
}

// OperatorProvisioningCreatesPerson is the CreatePerson of operator
// provisioning: an operator sets up a school's accounts before anyone
// entered their people there, so the school identity creates the missing
// person.
const OperatorProvisioningCreatesPerson = true

// SchoolIdentity is what the account carries at the school afterwards.
// TeacherID is zero for roles that run without a caregiver profile.
type SchoolIdentity struct {
	PersonID  int64
	StaffID   int64
	TeacherID int64
}

// SchoolIdentityProvisioning maintains the users.persons -> users.staff ->
// users.teachers chain a personnel role requires (#2222). The role
// classification it applies is RoleNeedsStaffRecord and
// RoleNeedsCaregiverProfile. EnsureSchoolIdentity runs on the caller's tenant
// transaction and returns nil for roles that need no staff record and for
// an account without a person when CreatePerson is false.
type SchoolIdentityProvisioning interface {
	EnsureSchoolIdentity(ctx context.Context, input SchoolIdentityInput) (*SchoolIdentity, error)
	// HasLiveCaregiverProfile reports whether the account's identity at the
	// tenant in context carries a live caregiver profile, the fact every path
	// that guards the Lehrkraft role reads (#1772). Offboarded records do not
	// count.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
}

// InviteToStudentRequest is the unified "invite an email to a child" input,
// used by both the staff "Erziehungsberechtigte" tab and the parents portal.
// The caller resolves RequireApproval from the tenant setting and performs
// the caller-type authorization beforehand.
type InviteToStudentRequest struct {
	StudentID        int64
	Email            string
	FirstName        string
	LastName         string
	RelationshipType string
	CreatedBy        int64
	// RequestedByParentAccountID is set when a parent initiated the invite
	// via the parents portal; nil for staff-initiated invites.
	RequestedByParentAccountID *int64
	// RequireApproval queues the invite for staff approval instead of
	// acting immediately.
	RequireApproval bool
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to legal_guardian (#2172). A PARENT-initiated confirmed upgrade
	// is always queued for staff approval.
	ConfirmRoleUpgrade bool
}

// StaffInviteRequiresApproval is the RequireApproval of an invite staff send
// from the "Erziehungsberechtigte" tab: staff are the approvers, so their
// invite acts immediately.
const StaffInviteRequiresApproval = false

// InviteToStudentOutcome describes what the resolve logic did.
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
	InvitationID      *int64
	ExistingRole      string
}

// RevokeAccessRequest removes one account's access to one child by deleting
// the relationship. The account, profile, and sibling links are untouched.
type RevokeAccessRequest struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
	// ByParent is true for parents-portal removals. Parents may not remove
	// the primary guardian; staff may remove anyone.
	ByParent bool
	// MayClearPayer says whether the actor holds guardians:financial (#2608).
	MayClearPayer bool
}

// ParentRevokeAccessRequest is the parents-portal removal of a relative's
// access: a parent never removes the primary guardian, and never holds
// guardians:financial, so the payer guard stays closed.
func ParentRevokeAccessRequest(studentID, guardianProfileID, actorAccountID int64) RevokeAccessRequest {
	return RevokeAccessRequest{
		StudentID: studentID, GuardianProfileID: guardianProfileID, ActorAccountID: actorAccountID,
		ByParent: true,
	}
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
	RoleUpgrade       bool
}

// GuardianRelativeAccess invites further guardians to a child, decides
// parent-initiated requests and revokes an account's access to one child.
// Every write runs on the caller's tenant transaction except RevokeAccess,
// which opens one for the tenant in context and locks the child's row.
type GuardianRelativeAccess interface {
	InviteToStudent(ctx context.Context, req InviteToStudentRequest) (*InviteToStudentResult, error)
	BulkInviteToStudents(ctx context.Context, req BulkInviteRequest) (*BulkInviteResult, error)
	ApproveInvitation(ctx context.Context, invitationID, approverAccountID int64) error
	RejectInvitation(ctx context.Context, invitationID, approverAccountID int64) error
	PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error)
	ListPendingApprovalsDetailed(ctx context.Context) ([]PendingApprovalView, error)
	RevokeAccess(ctx context.Context, req RevokeAccessRequest) error
}

// AccountLifecycle is the capability the retained auth service, the staff
// membership runtime and the guardian directory consume (#3225).
type AccountLifecycle interface {
	StaffPreview
	StaffOffboardingAccess
	SchoolIdentityProvisioning
	GuardianRelativeAccess
	GuardianInvitations
}
