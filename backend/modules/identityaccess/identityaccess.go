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
	ErrAccountNotFound = errors.New("account not found")
	// ErrTenantRequired reports a tenant-scoped command without a tenant.
	ErrTenantRequired = errors.New("tenant is required")
	// ErrGuardianRoleMissing reports a tenant without a guardian base role.
	ErrGuardianRoleMissing = errors.New("guardian role not found")
	// ErrOperatorNotFound reports a lookup or update that matched no operator.
	ErrOperatorNotFound = errors.New("operator not found")
	// ErrOperatorSessionNotFound reports a lookup that matched no operator
	// refresh session.
	ErrOperatorSessionNotFound = errors.New("operator session not found")
	// ErrOperatorSessionRotated reports a rotation hand-off on a session that
	// was already rotated or does not exist.
	ErrOperatorSessionRotated = errors.New("operator session was already rotated or not found")
	// ErrAccountSessionNotFound reports a lookup that matched no account
	// refresh session in the caller's tenant scope.
	ErrAccountSessionNotFound = errors.New("account session not found")
	// ErrAccountSessionRotated reports a rotation hand-off on an account
	// session that was already rotated or does not exist.
	ErrAccountSessionRotated = errors.New("account session was already rotated or not found")
)

// ErrorCode maps a capability error to the stable code recorded in metrics.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrRoleNotFound), errors.Is(err, ErrGuardianRoleMissing),
		errors.Is(err, ErrOperatorNotFound), errors.Is(err, ErrOperatorSessionNotFound),
		errors.Is(err, ErrAccountSessionNotFound), errors.Is(err, ErrSchoolNotFound), errors.Is(err, ErrAccountTenantAccessNotFound),
		errors.Is(err, ErrOperatorMFACredentialNotFound), errors.Is(err, ErrOperatorMFAChallengeNotFound),
		errors.Is(err, ErrOperatorTrustedDeviceNotFound), errors.Is(err, ErrOperatorInvitationNotFound),
		errors.Is(err, ErrOperatorEmailChangeNotFound), errors.Is(err, ErrOperatorPasskeyNotFound),
		errors.Is(err, ErrOperatorPasskeySessionNotFound), errors.Is(err, ErrAccountPasskeyNotFound),
		errors.Is(err, ErrAccountPasskeySessionNotFound):
		return "not_found"
	case errors.Is(err, ErrOperatorSessionRotated), errors.Is(err, ErrAccountSessionRotated), errors.Is(err, ErrAccountTenantAccessExists),
		errors.Is(err, ErrOperatorMFAChallengeStateChanged):
		return "conflict"
	case errors.Is(err, ErrTenantRequired):
		return "tenant_required"
	default:
		return "internal"
	}
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

// OperatorSession is one persisted, revocable operator refresh session.
// Rotation records the hand-off on the predecessor (RotatedAt,
// ReplacementTokenHash, RecoveryProofHash) so a replay can be detected and a
// lost response recovered within the rotation grace.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type OperatorSession struct {
	ID                   int64
	OperatorID           int64
	Token                string
	TokenHash            string
	Expiry               time.Time
	FamilyID             string
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// OperatorQuery resolves operators. Every lookup returns ErrOperatorNotFound
// for an unknown operator; e-mail matching is exact on the stored,
// lower-cased address.
type OperatorQuery interface {
	FindOperator(ctx context.Context, id int64) (Operator, error)
	// FindOperatorForUpdate locks the operator row for the caller's
	// transaction so concurrent credential changes serialize.
	FindOperatorForUpdate(ctx context.Context, id int64) (Operator, error)
	FindOperatorByEmail(ctx context.Context, email string) (Operator, error)
	ListOperators(ctx context.Context) ([]Operator, error)
}

// OperatorCommand changes operator identity rows. Create and Update validate
// and normalize the address and display name; Update writes every mutable
// column and returns ErrOperatorNotFound when the row is gone.
type OperatorCommand interface {
	CreateOperator(ctx context.Context, operator Operator) (Operator, error)
	UpdateOperator(ctx context.Context, operator Operator) (Operator, error)
	DeleteOperator(ctx context.Context, id int64) error
	RecordOperatorLogin(ctx context.Context, id int64) error
	// IncrementOperatorMFAAttempts bumps the failed-attempt counter atomically
	// and applies the lockout window once threshold is reached.
	IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (OperatorMFAAttempts, error)
	ResetOperatorMFAAttempts(ctx context.Context, id int64) error
}

// OperatorSessionQuery resolves refresh sessions. Both lookups return
// ErrOperatorSessionNotFound when nothing matches.
type OperatorSessionQuery interface {
	// FindOperatorSessionForUpdate locks the session row for the caller's
	// transaction; refresh serializes on it before rotating.
	FindOperatorSessionForUpdate(ctx context.Context, token string) (OperatorSession, error)
	LatestOperatorSessionInFamily(ctx context.Context, familyID string) (OperatorSession, error)
}

// OperatorSessionCommand mints, rotates and revokes refresh sessions. The
// revocations return the deleted sessions so the caller can record its audit
// evidence in the same transaction.
type OperatorSessionCommand interface {
	CreateOperatorSession(ctx context.Context, session OperatorSession) (OperatorSession, error)
	// MarkOperatorSessionRotated records the hand-off exactly once and returns
	// ErrOperatorSessionRotated when the session was rotated before.
	MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	// DeleteExpiredRotatedOperatorSessions drops rotated predecessors of the
	// family whose refresh JWTs expired before now; earlier ones stay as
	// replay evidence.
	DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error
	DeleteOperatorSession(ctx context.Context, id int64) error
	RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]OperatorSession, error)
	RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]OperatorSession, error)
	DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error)
}

// OperatorAccess is the capability operator login, refresh, MFA lockout and
// session revocation consume (#2720).
type OperatorAccess interface {
	OperatorQuery
	OperatorCommand
	OperatorSessionQuery
	OperatorSessionCommand
}

// AccountSession is one persisted, revocable refresh session of a platform
// account at one school. A family groups the generations one login produced
// through rotation; the hand-off (RotatedAt, ReplacementTokenHash,
// RecoveryProofHash) lets a lost rotation response be recovered within the
// rotation grace and a replay be detected. FamilyExpiryCap bounds every
// successor of a family a tenant switch retired.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type AccountSession struct {
	ID       int64
	TenantID int64
	// OrganizationID is the Träger of a Träger-Portal session (portal scope
	// org); such a session has no school (ADR 0084).
	OrganizationID       int64
	AccountID            int64
	Token                string
	TokenHash            string
	Expiry               time.Time
	Mobile               bool
	Identifier           *string
	PortalScope          string
	MFAVerified          bool
	FamilyID             string
	FamilyExpiryCap      *time.Time
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AccountSessionLiveness narrows a listing by expiry.
type AccountSessionLiveness int

const (
	// AccountSessionsAny lists sessions regardless of expiry.
	AccountSessionsAny AccountSessionLiveness = iota
	// AccountSessionsLive lists sessions whose expiry is in the future.
	AccountSessionsLive
	// AccountSessionsExpired lists sessions whose expiry has passed.
	AccountSessionsExpired
)

// AccountSessionFilter narrows ListAccountSessions. Zero values do not
// filter; the caller's tenant scope always applies.
type AccountSessionFilter struct {
	AccountID int64
	FamilyID  string
	Mobile    *bool
	Liveness  AccountSessionLiveness
}

// AccountSessionQuery resolves account refresh sessions. Reads that name no
// tenant apply the caller's tenant scope: a tenantless caller (login,
// refresh and logout run before a tenant is known) sees every school's rows.
// Lookups return ErrAccountSessionNotFound when nothing matches.
type AccountSessionQuery interface {
	FindAccountSession(ctx context.Context, token string) (AccountSession, error)
	// FindAccountSessionForUpdate locks the session row for the caller's
	// transaction; refresh serializes on it before rotating.
	FindAccountSessionForUpdate(ctx context.Context, token string) (AccountSession, error)
	LatestAccountSessionInFamily(ctx context.Context, familyID string) (AccountSession, error)
	ListAccountSessions(ctx context.Context, filter AccountSessionFilter) ([]AccountSession, error)
	// CountExpiredAccountSessions counts across every school; the cleanup
	// preview reports the whole sweep.
	CountExpiredAccountSessions(ctx context.Context) (int, error)
	// ListInactiveAccountIDsWithLiveSessions names deactivated accounts that
	// still hold an un-rotated, unexpired session, so a failed account-wide
	// wipe can be recovered.
	ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error)
	HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error)
}

// AccountSessionCommand mints, rotates, caps and revokes account refresh
// sessions. Every write joins the caller's transaction; the revocations
// return the deleted sessions so the caller can record its audit evidence in
// the same transaction.
type AccountSessionCommand interface {
	// CreateAccountSession validates the session and pins it to the caller's
	// tenant when the session names none.
	CreateAccountSession(ctx context.Context, session AccountSession) (AccountSession, error)
	// MarkAccountSessionRotated records the hand-off exactly once and returns
	// ErrAccountSessionRotated when the session was rotated before.
	MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	// DeleteExpiredRotatedAccountSessions drops rotated predecessors of the
	// account whose refresh JWTs expired before now; earlier ones stay as
	// replay evidence.
	DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error
	// RetireAccountSessionFamily caps the expiry of the family's live sessions
	// so a tenant switch hands the replaced session a bounded grace.
	RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error
	// EnforceAccountSessionCap keeps at most keep live sessions in the portal
	// group of portalScope and returns the evicted sessions.
	EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) ([]AccountSession, error)
	DeleteAccountSession(ctx context.Context, id int64) error
	RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]AccountSession, error)
	// RevokeAccountSessionsInTenant deletes the account's sessions at the
	// caller's school only and returns ErrTenantRequired without one.
	RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]AccountSession, error)
	// RevokeAllAccountSessions deletes the account's sessions at every school.
	RevokeAllAccountSessions(ctx context.Context, accountID int64) ([]AccountSession, error)
	// RevokeAccountSessionsCreatedAtOrBefore deletes the sessions that already
	// existed at cutoff, including their later refresh successors, and keeps
	// logins that only started afterwards.
	RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]AccountSession, error)
	RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]AccountSession, error)
	DeleteExpiredAccountSessions(ctx context.Context) (int, error)
}

// AccountSessionAccess is the capability tenant, parent and school login,
// refresh, switching, logout, session validation and revocation consume
// (#2720).
type AccountSessionAccess interface {
	AccountSessionQuery
	AccountSessionCommand
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
	OperatorAccess
	OperatorMFARecords
	OperatorMFAFlows
	OperatorTokens
	OperatorPasskeyRecords
	OperatorPasskeyFlows
	AccountPasskeyRecords
	AccountPasskeyFlows
	AccountMFA
	PasswordResets
	SchoolInvitations
	SignInDataRetention
	AccountRetention
	AccountSessionAccess
	RFIDQuery
	AccountProfiles
	AccountAnalyticsConsent
	SchoolAccountQuery
	InvitedPersonQuery
	StudentGuardianInvitationQuery
	SchoolRoleQuery
	RolePermissionQuery
	SchoolMembershipQuery
	AccountAuthentication
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
	// AccountHoldsRole reports whether the account holds the role of that
	// exact name. A failed lookup counts as not holding it: the answer only
	// ever withholds.
	AccountHoldsRole(ctx context.Context, accountID int64, roleName string) bool
	// DemoAccess returns the demo-only capability when the owner was
	// composed in the demo environment, and nil otherwise.
	DemoAccess() *DemoAccess
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
