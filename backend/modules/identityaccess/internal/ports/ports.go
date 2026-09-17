// Package ports declares the persistence and transaction seams the Identity &
// Access application needs. Composition binds them.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Store is the persistence port over the platform account table and the
// tenant-scoped mapping and role tables.
type Store interface {
	ListSchoolRoles(context.Context, int64) ([]*domain.SchoolRole, domain.OperationStats, error)
	FindSchoolRoleByName(context.Context, string, int64) (*domain.SchoolRole, domain.OperationStats, error)
	FindRolePermissions(context.Context, int64, int64) ([]string, domain.OperationStats, error)
	FindInvitedPersonIDs(ctx context.Context, email string, tenantID int64) ([]int64, domain.OperationStats, error)
	CountStudentGuardianInvitations(ctx context.Context, studentID, tenantID int64) (int, domain.OperationStats, error)
	HasActiveAccountTenant(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error)
	// ListAccountRoleIDs returns the roles the account holds at this school.
	ListAccountRoleIDs(ctx context.Context, accountID, tenantID int64) ([]int64, domain.OperationStats, error)
	// ListActiveAccountIDs returns every account with an active mapping to the
	// school, ascending by account id.
	ListActiveAccountIDs(ctx context.Context, tenantID int64) ([]int64, domain.OperationStats, error)
	FindRFIDCard(ctx context.Context, tag string, tenantID int64) (string, bool, domain.OperationStats, error)
	FindAccount(ctx context.Context, id int64) (domain.Account, bool, domain.OperationStats, error)
	FindAccountByEmail(ctx context.Context, email string) (domain.Account, bool, domain.OperationStats, error)
	// EnsureActiveTenantMapping inserts the account's mapping for the tenant or
	// reactivates an existing one, clearing its deactivation.
	EnsureActiveTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
	// FindRoleByName resolves a role visible to the tenant, preferring the
	// tenant's own role over the system role of the same name.
	FindRoleByName(ctx context.Context, name string, tenantID int64) (int64, bool, domain.OperationStats, error)
	// AssignAccountRole assigns the role for the tenant once and reports
	// whether this call created the assignment; an existing assignment is
	// left untouched.
	AssignAccountRole(ctx context.Context, accountID, roleID, tenantID int64) (bool, domain.OperationStats, error)
}

// OperatorStore is the persistence port over the platform operator table
// and the operator refresh-session table. Both are platform-wide: no row
// carries a tenant.
type OperatorStore interface {
	FindOperator(ctx context.Context, id int64, forUpdate bool) (domain.Operator, bool, domain.OperationStats, error)
	FindOperatorByEmail(ctx context.Context, email string) (domain.Operator, bool, domain.OperationStats, error)
	ListOperators(ctx context.Context) ([]domain.Operator, domain.OperationStats, error)
	// InsertOperator stores a validated operator and returns the row with its
	// identity and timestamps.
	InsertOperator(ctx context.Context, operator domain.Operator) (domain.Operator, domain.OperationStats, error)
	// UpdateOperator overwrites every mutable column of the operator's row and
	// reports whether the row existed.
	UpdateOperator(ctx context.Context, operator domain.Operator) (bool, domain.OperationStats, error)
	DeleteOperator(ctx context.Context, id int64) (domain.OperationStats, error)
	RecordOperatorLogin(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error)
	// IncrementOperatorMFAAttempts bumps the counter and applies the lockout
	// window in one statement once the threshold is reached.
	IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperatorMFAAttempts, bool, domain.OperationStats, error)
	ResetOperatorMFAAttempts(ctx context.Context, id int64) (domain.OperationStats, error)

	FindOperatorSessionByToken(ctx context.Context, token string, forUpdate bool) (domain.OperatorSession, bool, domain.OperationStats, error)
	LatestOperatorSessionInFamily(ctx context.Context, familyID string) (domain.OperatorSession, bool, domain.OperationStats, error)
	InsertOperatorSession(ctx context.Context, session domain.OperatorSession) (domain.OperatorSession, domain.OperationStats, error)
	// MarkOperatorSessionRotated records the hand-off on an un-rotated row and
	// reports whether such a row existed.
	MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error)
	// DeleteExpiredRotatedOperatorSessions removes rotated predecessors of the
	// family whose refresh JWTs expired before now.
	DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) (domain.OperationStats, error)
	DeleteOperatorSession(ctx context.Context, id int64) (domain.OperationStats, error)
	DeleteOperatorSessionsByOperator(ctx context.Context, operatorID int64) ([]domain.OperatorSession, domain.OperationStats, error)
	DeleteOperatorSessionsByFamily(ctx context.Context, familyID string) ([]domain.OperatorSession, domain.OperationStats, error)
	DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error)
}

// OperatorMFAStore is the persistence port over the operator MFA enrollment,
// e-mail challenge and trusted-device tables. All three are platform-wide:
// no row carries a tenant. "Active" compares against the now the caller
// passes.
type OperatorMFAStore interface {
	FindOperatorMFACredential(ctx context.Context, operatorID int64) (domain.OperatorMFACredential, bool, domain.OperationStats, error)
	InsertOperatorMFACredential(ctx context.Context, credential domain.OperatorMFACredential) (domain.OperatorMFACredential, domain.OperationStats, error)
	TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) (domain.OperationStats, error)
	DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) (domain.OperationStats, error)

	InsertOperatorMFAChallenge(ctx context.Context, challenge domain.OperatorMFAChallenge) (domain.OperatorMFAChallenge, domain.OperationStats, error)
	// FindActiveOperatorMFAChallenge returns the unconsumed, unexpired code
	// with the latest expiry.
	FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64, now time.Time) (domain.OperatorMFAChallenge, bool, domain.OperationStats, error)
	CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, domain.OperationStats, error)
	// ActivateOperatorMFAChallenge clears consumed_at on a consumed row and
	// reports whether such a row existed.
	ActivateOperatorMFAChallenge(ctx context.Context, id int64) (bool, domain.OperationStats, error)
	// ConsumeOperatorMFAChallenge stamps consumed_at on an unconsumed row and
	// reports whether such a row existed.
	ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) (bool, domain.OperationStats, error)

	InsertOperatorTrustedDevice(ctx context.Context, device domain.OperatorTrustedDevice) (domain.OperatorTrustedDevice, domain.OperationStats, error)
	FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string, now time.Time) (domain.OperatorTrustedDevice, bool, domain.OperationStats, error)
	// ListActiveOperatorTrustedDevices orders by last use, most recent first,
	// then by creation.
	ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64, now time.Time) ([]domain.OperatorTrustedDevice, domain.OperationStats, error)
	TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) (domain.OperationStats, error)
	// RevokeOperatorTrustedDevice stamps revoked_at on an unrevoked row and
	// reports whether such a row existed.
	RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) (bool, domain.OperationStats, error)
	RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) (domain.OperationStats, error)
}

// OperatorTokenStore is the persistence port over the operator invitation
// and e-mail change links. Both tables are platform-wide: no row carries a
// tenant. "Redeemable" means unused and not expired at the now the caller
// passes. The bool results report whether a row in the expected state
// existed.
type OperatorTokenStore interface {
	InsertOperatorInvitation(ctx context.Context, invitation domain.OperatorInvitation) (domain.OperatorInvitation, domain.OperationStats, error)
	FindOperatorInvitation(ctx context.Context, id int64) (domain.OperatorInvitation, bool, domain.OperationStats, error)
	FindRedeemableOperatorInvitation(ctx context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error)
	// ListRedeemableOperatorInvitations orders by creation, newest first.
	ListRedeemableOperatorInvitations(ctx context.Context, now time.Time) ([]domain.OperatorInvitation, domain.OperationStats, error)
	CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, domain.OperationStats, error)
	// RedeemOperatorInvitation stamps used_at on the redeemable row and
	// returns it; concurrent redemptions see exactly one winner.
	RedeemOperatorInvitation(ctx context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error)
	RevokeOperatorInvitation(ctx context.Context, id int64, now time.Time) (bool, domain.OperationStats, error)
	RevokeOperatorInvitationsForEmail(ctx context.Context, email string, now time.Time) (int, domain.OperationStats, error)
	ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt, now time.Time) (bool, domain.OperationStats, error)
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error)
	DeleteExpiredOperatorInvitations(ctx context.Context, now time.Time) (int, domain.OperationStats, error)

	InsertOperatorEmailChange(ctx context.Context, change domain.OperatorEmailChange) (domain.OperatorEmailChange, domain.OperationStats, error)
	CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, domain.OperationStats, error)
	// RedeemOperatorEmailChange marks the redeemable row used and returns it;
	// concurrent redemptions see exactly one winner.
	RedeemOperatorEmailChange(ctx context.Context, token string, now time.Time) (domain.OperatorEmailChange, bool, domain.OperationStats, error)
	RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) (domain.OperationStats, error)
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error)
	// RevokeExpiredOperatorEmailChanges marks expired, unused rows used so
	// they stop occupying the one-active-link-per-operator index.
	RevokeExpiredOperatorEmailChanges(ctx context.Context, now time.Time) (int, domain.OperationStats, error)
	// DeleteStaleOperatorEmailChanges deletes expired or used rows created
	// before createdBefore, so the rate-limit window keeps its rows.
	DeleteStaleOperatorEmailChanges(ctx context.Context, createdBefore, now time.Time) (int, domain.OperationStats, error)
}

// OperatorPasskeyStore is the persistence port over the operator passkey
// credential and ceremony-session tables. Both are platform-wide: no row
// carries a tenant. "Active" means unrevoked.
type OperatorPasskeyStore interface {
	InsertOperatorPasskey(ctx context.Context, credential domain.OperatorPasskeyCredential) (domain.OperatorPasskeyCredential, domain.OperationStats, error)
	// ListActiveOperatorPasskeys orders by creation, oldest first.
	ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]domain.OperatorPasskeyCredential, domain.OperationStats, error)
	FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (domain.OperatorPasskeyCredential, bool, domain.OperationStats, error)
	// UpdateOperatorPasskeyAfterUse stores the credential state on an active
	// row and reports whether such a row existed.
	UpdateOperatorPasskeyAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) (bool, domain.OperationStats, error)
	// RevokeOperatorPasskey stamps revoked_at on an active row of that
	// operator and reports whether such a row existed.
	RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) (bool, domain.OperationStats, error)

	InsertOperatorPasskeySession(ctx context.Context, session domain.OperatorPasskeySession) (domain.OperatorPasskeySession, domain.OperationStats, error)
	// ConsumeOperatorPasskeySession stamps consumed_at on an unconsumed row
	// with that purpose that expires after consumedAt and returns it.
	ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (domain.OperatorPasskeySession, bool, domain.OperationStats, error)
}

// AccountPasskeyStore is the persistence port over the school-portal
// passkey credential and ceremony-session tables. Credentials belong to the
// account and carry no tenant; a ceremony records the school whose portal
// started it. "Active" means unrevoked.
type AccountPasskeyStore interface {
	InsertAccountPasskey(ctx context.Context, credential domain.AccountPasskeyCredential) (domain.AccountPasskeyCredential, domain.OperationStats, error)
	// ListActiveAccountPasskeys orders by creation, oldest first.
	ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]domain.AccountPasskeyCredential, domain.OperationStats, error)
	FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (domain.AccountPasskeyCredential, bool, domain.OperationStats, error)
	// UpdateAccountPasskeyAfterUse stores the credential state on an active
	// row and reports whether such a row existed.
	UpdateAccountPasskeyAfterUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) (bool, domain.OperationStats, error)
	// RevokeAccountPasskey stamps revoked_at on an active row of that account
	// and reports whether such a row existed.
	RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) (bool, domain.OperationStats, error)

	InsertAccountPasskeySession(ctx context.Context, session domain.AccountPasskeySession) (domain.AccountPasskeySession, domain.OperationStats, error)
	// ConsumeAccountPasskeySession stamps consumed_at on an unconsumed row
	// with that purpose that expires after consumedAt and returns it.
	ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (domain.AccountPasskeySession, bool, domain.OperationStats, error)
}

// AccountSessionStore is the persistence port over auth.tokens, the
// tenant-scoped refresh sessions of platform accounts. Reads and deletes that
// name no explicit tenant apply the scope the composition resolves from the
// caller's context, exactly as the retained repository did; the cap and
// retirement writes skip that filter inside an administrative transaction.
type AccountSessionStore interface {
	FindAccountSessionByToken(ctx context.Context, token string, forUpdate bool) (domain.AccountSession, bool, domain.OperationStats, error)
	LatestAccountSessionInFamily(ctx context.Context, familyID string) (domain.AccountSession, bool, domain.OperationStats, error)
	ListAccountSessions(ctx context.Context, filter domain.AccountSessionFilter, now time.Time) ([]domain.AccountSession, domain.OperationStats, error)
	// CountExpiredAccountSessions counts across every tenant; the cleanup
	// preview reports the whole sweep.
	CountExpiredAccountSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error)
	ListInactiveAccountIDsWithLiveSessions(ctx context.Context, now time.Time) ([]int64, domain.OperationStats, error)
	HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since, now time.Time) (bool, domain.OperationStats, error)

	// InsertAccountSession stores a validated session and returns the row
	// with its identity and timestamps.
	InsertAccountSession(ctx context.Context, session domain.AccountSession) (domain.AccountSession, domain.OperationStats, error)
	// MarkAccountSessionRotated records the hand-off on an un-rotated row and
	// reports whether such a row existed.
	MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error)
	// DeleteExpiredRotatedAccountSessions removes rotated predecessors of the
	// account whose refresh JWTs expired before now.
	DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) (domain.OperationStats, error)
	// RetireAccountSessionFamily caps the expiry of the family's live sessions.
	RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) (domain.OperationStats, error)
	// ListLiveAccountSessionsForCap returns the account's live, un-rotated
	// sessions in the cap's portal group, newest expiry first.
	ListLiveAccountSessionsForCap(ctx context.Context, accountID int64, portalScopes []string, now time.Time) ([]domain.AccountSession, domain.OperationStats, error)
	DeleteAccountSessionsByID(ctx context.Context, ids []int64) ([]domain.AccountSession, domain.OperationStats, error)
	DeleteAccountSession(ctx context.Context, id int64) (domain.OperationStats, error)
	DeleteAccountSessionsByFamily(ctx context.Context, familyID string) ([]domain.AccountSession, domain.OperationStats, error)
	// DeleteAccountSessionsByAccount deletes the account's sessions: only the
	// caller's tenant when tenantScoped, every tenant otherwise. A non-zero
	// cutoff keeps sessions that only started after it.
	DeleteAccountSessionsByAccount(ctx context.Context, accountID int64, tenantScoped bool, cutoff time.Time) ([]domain.AccountSession, domain.OperationStats, error)
	DeleteAccountSessionsByTenant(ctx context.Context, tenantID int64) ([]domain.AccountSession, domain.OperationStats, error)
	DeleteExpiredAccountSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error)
}

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction, else an admin transaction for tenantless readers.
	RunRead(context.Context, func(context.Context) error) error
	// RunPlatform joins the caller's transaction when one is active and
	// otherwise runs on the root connection. Operator rows are platform-wide
	// and carry no tenant, so no tenant transaction is opened for them; the
	// operator flows open their own administrative transaction where several
	// writes must commit together.
	RunPlatform(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)

// AccountLoginStore is the persistence port over the identity-owned facts
// tenant, parent and school login, refresh, switching and session validation
// read: the account row with its credential, the school mappings and the
// tenant-scoped roles and permissions. Every statement runs on the
// connection the caller's context carries; the locking variants require an
// ambient transaction.
type AccountLoginStore interface {
	// HasActiveAccountTenant reports an active mapping of the account at the
	// school.
	HasActiveAccountTenant(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error)
	// FindLoginAccountByEmail matches case-insensitively.
	FindLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	FindLoginAccount(ctx context.Context, id int64, forUpdate bool) (domain.LoginAccount, bool, domain.OperationStats, error)
	// RecordAccountLogin stamps the last login; on the login path it also
	// takes the account row lock the session cap serializes on.
	RecordAccountLogin(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error)
	// ListActiveTenantIDs returns the schools the account is actively mapped
	// to in mapping-creation order.
	ListActiveTenantIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error)
	// LockActiveTenantMappingShared is HasActiveAccountTenant with a FOR
	// SHARE lock so a revocation cannot commit under a mint.
	LockActiveTenantMappingShared(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error)
	// ListAccountRolesAtTenant returns the roles the account holds at the
	// school, with their role facts; forShare pins the assignment rows.
	ListAccountRolesAtTenant(ctx context.Context, accountID, tenantID int64, forShare bool) ([]domain.RoleAssignment, domain.OperationStats, error)
	// ListAccountPermissionsAtTenant returns the distinct effective
	// permission names (resource:action) granted directly or through roles.
	ListAccountPermissionsAtTenant(ctx context.Context, accountID, tenantID int64) ([]string, domain.OperationStats, error)
	// LockAccountPermissionSources pins the direct grants and the role
	// permissions the effective set derives from.
	LockAccountPermissionSources(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
}

// SchoolDirectory is the consumer-owned port over the Organisation & Tenancy
// school facts login resolves. Missing schools report found=false; every
// other failure is an error.
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (domain.School, bool, error)
	FindSchoolBySubdomain(ctx context.Context, subdomain string) (domain.School, bool, error)
	// LockSchoolShared reads the school under a FOR SHARE lock inside the
	// caller's transaction.
	LockSchoolShared(ctx context.Context, id int64) (domain.School, bool, error)
	// ListActiveSchoolsOfAccount returns the live, active schools the
	// account is actively mapped to.
	ListActiveSchoolsOfAccount(ctx context.Context, accountID int64) ([]domain.School, error)
}

// PersonDirectory is the consumer-owned port over the People Directory name
// of an account's person row in the tenant the context carries.
type PersonDirectory interface {
	FindPersonName(ctx context.Context, accountID int64) (firstName, lastName string, found bool, err error)
}

// PasswordVerifier checks a password against the stored hash.
type PasswordVerifier interface {
	VerifyPassword(password, hash string) (bool, error)
}

// TokenCodec signs and parses the session JWTs.
type TokenCodec interface {
	IssueTokenPair(access domain.SessionClaims, refresh domain.RefreshClaims) (accessToken, refreshToken string, err error)
	IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error)
	// ParseAccessToken verifies the signature and expiry of an access JWT.
	ParseAccessToken(token string) (domain.SessionClaims, error)
	// ParseRefreshToken verifies the signature of a refresh JWT and refuses
	// challenge, enrollment and preview tokens.
	ParseRefreshToken(token string) (domain.RefreshClaims, error)
	RefreshExpiry() time.Duration
}

// MFAPolicy is a resolved MFA verdict waiting for the role set it applies to.
type MFAPolicy interface {
	RequiredFor(roleNames []string) bool
}

// MFAGate is the consumer-owned port over the retained MFA service. An
// unconfigured gate (Configured false) means "not required / not enrolled".
type MFAGate interface {
	Configured() bool
	IsRequired(ctx context.Context, accountID int64, email string, roleNames []string, tenantID int64) (bool, error)
	ResolvePolicy(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	// ResolvePolicyInTx re-reads the policy on the caller's transaction, past
	// every request-scoped cache.
	ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	HasEnrollment(ctx context.Context, accountID int64) (bool, error)
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error)
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error)
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool
	TrustedDeviceDays(ctx context.Context, tenantID int64) int
}

// MFAPolicyLock pins a school's MFA mode for the rest of the caller's
// transaction, in shared mode.
type MFAPolicyLock interface {
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
}

// AuthAudit is the consumer-owned port over the Audit platform's
// authentication ledger.
type AuthAudit interface {
	// RecordAuthEvent appends the event on the caller's transaction.
	RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error
	ListPendingAccountWideWipes(ctx context.Context) ([]domain.PendingAccountWideWipe, error)
	ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]domain.PendingAccountWideWipe, error)
}

// PushSubscriptionCleanup is the consumer-owned port over the Delivery
// platform's push subscription rows a session revocation orphans.
type PushSubscriptionCleanup interface {
	DeleteStaffByAccount(ctx context.Context, accountID int64) error
	DeleteSchoolByAccount(ctx context.Context, accountID int64) error
	DeleteParentByAccount(ctx context.Context, accountID int64) error
	DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error
	DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error
	DeleteOrphaned(ctx context.Context) error
}

// Runtime is the tenant transaction runtime the account flows run under:
// administrative transactions for the pre-authentication flows, tenant
// transactions for audit evidence, and the after-commit hooks a revocation
// defers its cleanup to.
type Runtime interface {
	WithAdminTx(ctx context.Context, fn func(context.Context) error) error
	WithTenantTx(ctx context.Context, tenantID int64, fn func(context.Context) error) error
	// RunInTx joins the caller's transaction, runs tenantless scoped callers
	// administratively and otherwise opens the tenant's transaction.
	RunInTx(ctx context.Context, fn func(context.Context) error) error
	IsAdminTx(ctx context.Context) bool
	HasTransaction(ctx context.Context) bool
	HasAfterCommitHooks(ctx context.Context) bool
	RegisterAfterCommit(ctx context.Context, fn func())
	TenantID(ctx context.Context) int64
	Scope(ctx context.Context) string
	OrgID(ctx context.Context) int64
	WithTenantID(ctx context.Context, tenantID int64) context.Context
	// Detach strips the transaction, the tenant and the after-commit hooks
	// so independent cleanup cannot join the caller's outcome.
	Detach(ctx context.Context) context.Context
	WithoutTransaction(ctx context.Context) context.Context
}

// AccountAccessStore is the persistence port over the identity-owned rows
// the operator-led school access flows read and write (#3252): the account
// row, its school mappings, its role assignments and direct permissions,
// and the roles that may be handed out. Every statement runs on the
// administrative transaction the flows open; the mapping and role reads
// name their tenant explicitly and apply no context scope.
type AccountAccessStore interface {
	FindManagedAccount(ctx context.Context, id int64, forUpdate bool) (domain.ManagedAccount, bool, domain.OperationStats, error)
	SetAccountActive(ctx context.Context, id int64, active bool) (domain.OperationStats, error)
	// ListTenantMappings returns every school mapping of the account,
	// ascending by school.
	ListTenantMappings(ctx context.Context, accountID int64) ([]domain.TenantMapping, domain.OperationStats, error)
	// DeactivateTenantMapping marks the mapping inactive and drops the
	// staff calendar feed token it carried.
	DeactivateTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
	// ListAccountRoleAssignments returns the account's role assignments at
	// every school with their role facts.
	ListAccountRoleAssignments(ctx context.Context, accountID int64) ([]domain.AccountRoleAssignment, domain.OperationStats, error)
	RemoveAccountRole(ctx context.Context, accountID, roleID, tenantID int64) (domain.OperationStats, error)
	DeleteAccountPermissionsAtTenant(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
	FindRole(ctx context.Context, roleID int64) (domain.RoleFact, bool, domain.OperationStats, error)
	ListRoles(ctx context.Context) ([]domain.RoleFact, domain.OperationStats, error)
}

// OrganizationDirectory is the consumer-owned port over the Organisation &
// Tenancy facts the access listing shows: the schools behind the mappings
// and the organisation names, in the owner's name order.
type OrganizationDirectory interface {
	ListSchools(ctx context.Context, ids []int64) ([]domain.School, error)
	ListOrganizationNames(ctx context.Context, ids []int64) ([]domain.OrganizationName, error)
}

// SchoolIdentityProvisioner is the consumer-owned port over the People
// Directory and School Membership identity chain a school access requires
// (persons, staff, teachers) and the facts the listing shows about it. The
// tenant-scoped reads take the tenant from the context; the platform-wide
// reads run administratively.
type SchoolIdentityProvisioner interface {
	// HasLivePersonAtSchool reports a live person carrying the account at
	// the school in context.
	HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error)
	// ListAccountPersons returns every person row carrying the account at
	// any school.
	ListAccountPersons(ctx context.Context, accountID int64) ([]domain.AccountPersonIdentity, error)
	// HasLiveCaregiverProfile reports a live users.teachers row behind the
	// account's identity at the school in context.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
	// EnsureSchoolIdentity creates the person, staff and (for caregiver
	// roles) teacher rows idempotently on the caller's tenant transaction.
	// A request the caller can correct is reported as
	// *domain.InvalidInputError.
	EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error
	// ListAccountIdentityFacts reports, per school, whether a person and a
	// staff record back the account.
	ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]domain.AccountIdentityFact, error)
}

// SchoolRolePolicy is the consumer-owned port over the retained role
// assignment rules every school-access path shares (#1772, #2222): which
// roles may be handed out at a school, which role is the lehrkraft system
// role, which roles need a staff record, and the message a lehrkraft
// account's role change is refused with.
type SchoolRolePolicy interface {
	ValidateAssignableSchoolRole(role domain.RoleFact, tenantID int64) error
	IsLehrkraftSystemRole(role domain.RoleFact) bool
	RoleNeedsStaffRecord(role domain.RoleFact) bool
	LehrkraftRoleImmutable() error
}

// OperatorMFAGate is the consumer-owned port over the retained operator MFA
// service. Operator MFA is mandatory: an unconfigured gate (Configured
// false) issues token pairs directly, as the early phases did.
type OperatorMFAGate interface {
	Configured() bool
	HasEnrollment(ctx context.Context, operatorID int64) (bool, error)
	VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error)
	StartChallenge(ctx context.Context, operatorID int64, ipAddress string) (string, error)
	TrustedDeviceDays() int
}

// OperatorAudit is the consumer-owned port over the Audit platform's
// operator action ledger. Entries append on the caller's transaction.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error
}

// OperatorCredentialCleanup invalidates the bearer-style controls a password
// rotation must not leave alive besides the refresh sessions: the pending
// e-mail change links. The composition binds it to the module's own link
// operations (#2722), so the revocation joins the password change.
type OperatorCredentialCleanup interface {
	InvalidateEmailChangeTokens(ctx context.Context, operatorID int64) error
}

// PasswordHasher hashes a new password and applies the strength policy.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ValidatePasswordStrength(password string) error
}

// Rotation is the refresh-rotation recovery policy shared with the operator
// portal.
type Rotation interface {
	RecoveryGrace() time.Duration
	MaxRecoveryHops() int
	RecoveryProofHash(ctx context.Context) []byte
	MatchesRecoveryProof(ctx context.Context, expected []byte) bool
	FamilyFingerprint(familyID string) string
}
