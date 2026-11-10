// Package identityaccess is the public Identity & Access capability other
// owners consume. The full Identity & Access migration comes late in #2580;
// this package exposes the narrow facts and commands earlier cutovers need
// just in time, starting with guardian portal access for enrollment
// acceptance (#2699), the platform operator identity and refresh sessions
// behind operator login, refresh and revocation, and the account refresh
// sessions behind tenant, parent and school login, refresh, switching and
// revocation (#2720). The role and permission administration behind the RBAC
// routes, staff membership, operator provisioning and staff offboarding
// followed with #3314.
//
// Accounts are platform-wide rows without a tenant. The school mapping
// (`auth.account_tenants`), the guardian base role assignment
// (`auth.account_roles`) and the account refresh sessions (`auth.tokens`)
// belong to the tenant in context. Operators and their refresh sessions are
// platform-wide as well and never carry a tenant.
package identityaccess

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrAccountNotFound reports a lookup that matched no platform account.
	ErrAccountNotFound = notFoundError("account not found")
	// ErrTenantRequired reports a tenant-scoped command without a tenant.
	ErrTenantRequired = tenantRequiredError("tenant is required")
	// ErrGuardianRoleMissing reports a tenant without a guardian base role.
	ErrGuardianRoleMissing = notFoundError("guardian role not found")
	// ErrOperatorNotFound reports a lookup or update that matched no operator.
	ErrOperatorNotFound = notFoundError("operator not found")
	// ErrOperatorSessionNotFound reports a lookup that matched no operator
	// refresh session.
	ErrOperatorSessionNotFound = notFoundError("operator session not found")
	// ErrOperatorSessionRotated reports a rotation hand-off on a session that
	// was already rotated or does not exist.
	ErrOperatorSessionRotated = conflictError("operator session was already rotated or not found")
	// ErrAccountSessionNotFound reports a lookup that matched no account
	// refresh session in the caller's tenant scope.
	ErrAccountSessionNotFound = notFoundError("account session not found")
	// ErrAccountSessionRotated reports a rotation hand-off on an account
	// session that was already rotated or does not exist.
	ErrAccountSessionRotated = conflictError("account session was already rotated or not found")
)

// ErrorCode is the stable code metrics record for a capability error: the
// class its sentinel declares, else "internal".
func ErrorCode(err error) string {
	if err == nil {
		return "none"
	}
	var classed *classedError
	if errors.As(err, &classed) {
		return classed.class
	}
	return "internal"
}

// classedError is a sentinel that names its metrics class where it is
// declared, so no list of sentinels has to be kept beside them.
type classedError struct{ text, class string }

func (e *classedError) Error() string { return e.text }

func notFoundError(text string) error { return &classedError{text: text, class: "not_found"} }
func conflictError(text string) error { return &classedError{text: text, class: "conflict"} }
func tenantRequiredError(text string) error {
	return &classedError{text: text, class: "tenant_required"}
}

// Account is the platform login fact enrollment acceptance needs: which
// address owns the account.
type Account struct {
	ID    int64
	Email string
}

// GuardianTenantAccess is the result of granting an account guardian access
// to the tenant in context. RoleAssigned is false when the guardian role was
// already assigned for this school.
type GuardianTenantAccess struct {
	AccountID    int64
	TenantID     int64
	RoleID       int64
	RoleAssigned bool
}

// GuardianAccessQuery resolves platform accounts. Both lookups return
// ErrAccountNotFound for an unknown account; e-mail matching is
// case-insensitive.
// AccountBatchQuery resolves current, active account addresses in one read.
type AccountBatchQuery func(context.Context, []int64) (map[int64]Account, error)

type GuardianAccessQuery interface {
	FindAccount(ctx context.Context, id int64) (Account, error)
	FindAccountByEmail(ctx context.Context, email string) (Account, error)
}

// GuardianAccessCommand makes an account's guardian membership in the tenant
// in context usable: the school mapping is created or reactivated and the
// guardian base role is assigned once. The command joins the caller's
// tenant transaction.
type GuardianAccessCommand interface {
	GrantGuardianTenantAccess(ctx context.Context, accountID int64) (GuardianTenantAccess, error)
	// SeedGuardianAccount is for the authorized local-development seeder. It
	// creates or reuses an account and grants guardian access at the context's
	// school in the caller's transaction. Existing credentials and activation
	// stay untouched; passwordHash is an already-hashed development credential.
	SeedGuardianAccount(ctx context.Context, email, passwordHash string) (int64, bool, error)
}

// GuardianAccess is the capability enrollment acceptance consumes.
type GuardianAccess interface {
	GuardianAccessQuery
	GuardianAccessCommand
}

// OperatorMFAAttempts is the counter snapshot after one failed MFA
// verification: the new attempt count and the lockout, if it was applied.
type OperatorMFAAttempts struct {
	Attempts    int
	LockedUntil *time.Time
}

// Owner is the Identity & Access capability the owner's composition serves:
// every public capability above, the role check the staff membership binds
// and the demo-only capability.
type Owner interface {
	StaffCalendarFeeds
	ParentCalendarFeeds
	SchoolAccountListings
	GuardianPortalQuery
	GuardianPortalLoginQuery
	RFIDCards
	AccountRoleQueries
	StaffAccountQueries
	GuardianSchools
	GuardianAccess
	OperatorLinkDeliveryRecorder
	OperatorRecordCleanup
	OperatorDirectory
	OperatorSessionLiveness
	OperatorMFAFlows
	OperatorPasskeyFlows
	AccountPasskeyFlows
	AccountMFA
	PasswordResets
	SchoolInvitations
	SchoolInvitationMailStates
	SignInDataRetention
	AccountRetention
	RFIDQuery
	AccountProfiles
	AccountAnalyticsConsent
	SchoolAccountQuery
	InvitedPersonQuery
	StudentGuardianInvitationQuery
	SchoolRoleQuery
	RolePermissionQuery
	SchoolMembershipQuery
	SchoolMembershipLock
	AccountAuthentication
	AccessSessions
	AccountSessionMaintenance
	AccountClaimsQuery
	OperatorAuthentication
	OperatorAccountAccess
	AccountLifecycle
	RoleAdministration
	AccountProvisioning
	AccountAdministration
	OperatorProvisioning
	OperatorStatus
	TraegerRoles
	TraegerAccountManagement
	TraegerOperatorInvitation
	// AccountHoldsRole reports whether the account holds the role of that
	// exact name. A failed lookup counts as not holding it: the answer only
	// ever withholds.
	AccountHoldsRole(ctx context.Context, accountID int64, roleName string) bool
	// DemoAccess returns the demo-only capability when the owner was
	// composed in the demo environment, and nil otherwise.
	DemoAccess() *DemoAccess
	// Caterers returns the caterer flows (ADR 0077) when the owner was
	// composed with their dependencies, and nil otherwise.
	Caterers() *CatererAccess
}

// InvitedPersonQuery retains the person identities of unused invitations in
// the current tenant. Expired invitations remain identity links until used or
// revoked; this query does not authorize accepting an expired invitation.
type InvitedPersonQuery interface {
	FindInvitedPersonIDs(context.Context, string) ([]int64, error)
}

// StudentGuardianInvitationQuery counts the guardian invitations that name a
// child. A permanent child deletion reports them as removed communications;
// the rows cascade with the student, so Identity owns only the count.
type StudentGuardianInvitationQuery interface {
	CountStudentGuardianInvitations(context.Context, int64) (int, error)
}

// SchoolAccountQuery resolves login identities with active membership in the
// current tenant. Unknown accounts and inactive/foreign mappings are not found.
type SchoolAccountQuery interface {
	FindSchoolAccountByEmail(context.Context, string) (Account, error)
}

// RFIDQuery resolves the canonical ID of a card belonging to the current
// tenant. Missing cards return found=false. A tenant is always required.
type RFIDQuery interface {
	FindRFIDCard(ctx context.Context, tag string) (id string, found bool, err error)
}
