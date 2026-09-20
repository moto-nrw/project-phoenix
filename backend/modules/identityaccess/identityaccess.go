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
	"fmt"
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
}

// GuardianAccess is the capability enrollment acceptance consumes.
type GuardianAccess interface {
	GuardianAccessQuery
	GuardianAccessCommand
}

// Operator is the platform login identity of a moto operator.
type Operator struct {
	ID             int64
	Email          string
	DisplayName    string
	PasswordHash   string
	Active         bool
	LastLogin      *time.Time
	MFAAttempts    int
	MFALockedUntil *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// OperatorMFAAttempts is the counter snapshot after one failed MFA
// verification: the new attempt count and the lockout, if it was applied.
type OperatorMFAAttempts struct {
	Attempts    int
	LockedUntil *time.Time
}

// OperatorSession is one persisted, revocable operator refresh session.
// Rotation records the hand-off on the predecessor (RotatedAt,
// ReplacementToken, RecoveryProofHash) so a replay can be detected and a
// lost response recovered within the rotation grace.
type OperatorSession struct {
	ID                int64
	OperatorID        int64
	Token             string
	Expiry            time.Time
	FamilyID          string
	Generation        int
	RotatedAt         *time.Time
	ReplacementToken  *string
	RecoveryProofHash []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
// through rotation; the hand-off (RotatedAt, ReplacementToken,
// RecoveryProofHash) lets a lost rotation response be recovered within the
// rotation grace and a replay be detected. FamilyExpiryCap bounds every
// successor of a family a tenant switch retired.
type AccountSession struct {
	ID                int64
	TenantID          int64
	AccountID         int64
	Token             string
	Expiry            time.Time
	Mobile            bool
	Identifier        *string
	PortalScope       string
	FamilyID          string
	FamilyExpiryCap   *time.Time
	Generation        int
	RotatedAt         *time.Time
	ReplacementToken  *string
	RecoveryProofHash []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
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

// Engine is the composed implementation behind the public module.
type Engine interface {
	StaffCalendarFeeds
	SchoolAccountListings
	GuardianPortalQuery
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
	AccountSessionAccess
	RFIDQuery
	AccountProfiles
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
}

// InvitedPersonQuery retains the person identities of unused invitations in
// the current tenant. Expired invitations remain identity links until used or
// revoked; this query does not authorize accepting an expired invitation.
type InvitedPersonQuery interface {
	FindInvitedPersonIDs(context.Context, string) ([]int64, error)
}

func (m *Module) FindInvitedPersonIDs(ctx context.Context, email string) ([]int64, error) {
	ids, err := m.engine.FindInvitedPersonIDs(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("identity access: find invited people: %w", err)
	}
	return ids, nil
}

// StudentGuardianInvitationQuery counts the guardian invitations that name a
// child. A permanent child deletion reports them as removed communications;
// the rows cascade with the student, so Identity owns only the count.
type StudentGuardianInvitationQuery interface {
	CountStudentGuardianInvitations(context.Context, int64) (int, error)
}

func (m *Module) CountStudentGuardianInvitations(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("identity access: count student guardian invitations: student ID is required")
	}
	count, err := m.engine.CountStudentGuardianInvitations(ctx, studentID)
	if err != nil {
		return 0, fmt.Errorf("identity access: count student guardian invitations: %w", err)
	}
	return count, nil
}

// SchoolAccountQuery resolves login identities with active membership in the
// current tenant. Unknown accounts and inactive/foreign mappings are not found.
type SchoolAccountQuery interface {
	FindSchoolAccountByEmail(context.Context, string) (Account, error)
}

func (m *Module) FindSchoolAccountByEmail(ctx context.Context, email string) (Account, error) {
	account, err := m.engine.FindSchoolAccountByEmail(ctx, email)
	if err != nil {
		return Account{}, fmt.Errorf("identity access: find school account: %w", err)
	}
	return account, nil
}

// RFIDQuery resolves the canonical ID of a card belonging to the current
// tenant. Missing cards return found=false. A tenant is always required.
type RFIDQuery interface {
	FindRFIDCard(ctx context.Context, tag string) (id string, found bool, err error)
}

func (m *Module) FindRFIDCard(ctx context.Context, tag string) (string, bool, error) {
	id, found, err := m.engine.FindRFIDCard(ctx, tag)
	if err != nil {
		return "", false, fmt.Errorf("identity access: find RFID card: %w", err)
	}
	return id, found, nil
}

// TenantRuntimeBinding is the seam the composition root binds the unit of
// work through that the module opens its own session, lifecycle and operator
// transactions under. modules/identityaccess/compose supplies the reference
// and the root supplies the runtime, each naming the runtime package it
// already depends on; the facade names only its own types, so the module's
// contract stays free of the transaction infrastructure (#3364).
type TenantRuntimeBinding interface {
	// BindRuntime binds the unit of work. A value the reference cannot use
	// is a composition mistake and is reported.
	BindRuntime(runtime any) error
}

// Module is the public Identity & Access facade.
type Module struct {
	engine Engine
	// runtime is the binding for the unit of work the module opens its own
	// transactions under. The root composes the module before it has built
	// the runtime, so it is bound afterwards through SetTenantRuntime.
	runtime TenantRuntimeBinding
}

// NewModule wraps the composed engine and the runtime binding the
// composition captured for its transactions. Composition supplies both;
// consumers depend on the interfaces above.
func NewModule(engine Engine, runtime TenantRuntimeBinding) *Module {
	if engine == nil {
		panic("identity access: engine is required")
	}
	return &Module{engine: engine, runtime: runtime}
}

// SetTenantRuntime binds the unit of work the module opens its session,
// lifecycle and operator transactions under. The composition root wires it
// once the runtime exists and passes the unit of work it owns; a module
// composed without a binding accepts the call and keeps its own fallback.
func (m *Module) SetTenantRuntime(runtime any) error {
	if m.runtime == nil {
		return nil
	}
	return m.runtime.BindRuntime(runtime)
}

func (m *Module) FindAccount(ctx context.Context, id int64) (Account, error) {
	account, err := m.engine.FindAccount(ctx, id)
	if err != nil {
		return Account{}, fmt.Errorf("identity access: find account: %w", err)
	}
	return account, nil
}

func (m *Module) FindAccountByEmail(ctx context.Context, email string) (Account, error) {
	account, err := m.engine.FindAccountByEmail(ctx, email)
	if err != nil {
		return Account{}, fmt.Errorf("identity access: find account by email: %w", err)
	}
	return account, nil
}

func (m *Module) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (GuardianTenantAccess, error) {
	access, err := m.engine.GrantGuardianTenantAccess(ctx, accountID)
	if err != nil {
		return GuardianTenantAccess{}, fmt.Errorf("identity access: grant guardian tenant access: %w", err)
	}
	return access, nil
}

func (m *Module) FindOperator(ctx context.Context, id int64) (Operator, error) {
	operator, err := m.engine.FindOperator(ctx, id)
	if err != nil {
		return Operator{}, fmt.Errorf("identity access: find operator: %w", err)
	}
	return operator, nil
}

func (m *Module) FindOperatorForUpdate(ctx context.Context, id int64) (Operator, error) {
	operator, err := m.engine.FindOperatorForUpdate(ctx, id)
	if err != nil {
		return Operator{}, fmt.Errorf("identity access: find operator for update: %w", err)
	}
	return operator, nil
}

func (m *Module) FindOperatorByEmail(ctx context.Context, email string) (Operator, error) {
	operator, err := m.engine.FindOperatorByEmail(ctx, email)
	if err != nil {
		return Operator{}, fmt.Errorf("identity access: find operator by email: %w", err)
	}
	return operator, nil
}

func (m *Module) ListOperators(ctx context.Context) ([]Operator, error) {
	operators, err := m.engine.ListOperators(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access: list operators: %w", err)
	}
	return operators, nil
}

func (m *Module) CreateOperator(ctx context.Context, operator Operator) (Operator, error) {
	stored, err := m.engine.CreateOperator(ctx, operator)
	if err != nil {
		return Operator{}, fmt.Errorf("identity access: create operator: %w", err)
	}
	return stored, nil
}

func (m *Module) UpdateOperator(ctx context.Context, operator Operator) (Operator, error) {
	stored, err := m.engine.UpdateOperator(ctx, operator)
	if err != nil {
		return Operator{}, fmt.Errorf("identity access: update operator: %w", err)
	}
	return stored, nil
}

func (m *Module) DeleteOperator(ctx context.Context, id int64) error {
	if err := m.engine.DeleteOperator(ctx, id); err != nil {
		return fmt.Errorf("identity access: delete operator: %w", err)
	}
	return nil
}

func (m *Module) RecordOperatorLogin(ctx context.Context, id int64) error {
	if err := m.engine.RecordOperatorLogin(ctx, id); err != nil {
		return fmt.Errorf("identity access: record operator login: %w", err)
	}
	return nil
}

func (m *Module) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (OperatorMFAAttempts, error) {
	attempts, err := m.engine.IncrementOperatorMFAAttempts(ctx, id, threshold, lockout)
	if err != nil {
		return OperatorMFAAttempts{}, fmt.Errorf("identity access: increment operator mfa attempts: %w", err)
	}
	return attempts, nil
}

func (m *Module) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	if err := m.engine.ResetOperatorMFAAttempts(ctx, id); err != nil {
		return fmt.Errorf("identity access: reset operator mfa attempts: %w", err)
	}
	return nil
}

func (m *Module) FindOperatorSessionForUpdate(ctx context.Context, token string) (OperatorSession, error) {
	session, err := m.engine.FindOperatorSessionForUpdate(ctx, token)
	if err != nil {
		return OperatorSession{}, fmt.Errorf("identity access: find operator session for update: %w", err)
	}
	return session, nil
}

func (m *Module) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (OperatorSession, error) {
	session, err := m.engine.LatestOperatorSessionInFamily(ctx, familyID)
	if err != nil {
		return OperatorSession{}, fmt.Errorf("identity access: latest operator session in family: %w", err)
	}
	return session, nil
}

func (m *Module) CreateOperatorSession(ctx context.Context, session OperatorSession) (OperatorSession, error) {
	stored, err := m.engine.CreateOperatorSession(ctx, session)
	if err != nil {
		return OperatorSession{}, fmt.Errorf("identity access: create operator session: %w", err)
	}
	return stored, nil
}

func (m *Module) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	if err := m.engine.MarkOperatorSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt); err != nil {
		return fmt.Errorf("identity access: mark operator session rotated: %w", err)
	}
	return nil
}

func (m *Module) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	if err := m.engine.DeleteExpiredRotatedOperatorSessions(ctx, familyID, now); err != nil {
		return fmt.Errorf("identity access: delete expired rotated operator sessions: %w", err)
	}
	return nil
}

func (m *Module) DeleteOperatorSession(ctx context.Context, id int64) error {
	if err := m.engine.DeleteOperatorSession(ctx, id); err != nil {
		return fmt.Errorf("identity access: delete operator session: %w", err)
	}
	return nil
}

func (m *Module) RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]OperatorSession, error) {
	sessions, err := m.engine.RevokeOperatorSessions(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke operator sessions: %w", err)
	}
	return sessions, nil
}

func (m *Module) RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]OperatorSession, error) {
	sessions, err := m.engine.RevokeOperatorSessionFamily(ctx, familyID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke operator session family: %w", err)
	}
	return sessions, nil
}

func (m *Module) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error) {
	deleted, err := m.engine.DeleteExpiredOperatorSessions(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired operator sessions: %w", err)
	}
	return deleted, nil
}

func (m *Module) FindAccountSession(ctx context.Context, token string) (AccountSession, error) {
	session, err := m.engine.FindAccountSession(ctx, token)
	if err != nil {
		return AccountSession{}, fmt.Errorf("identity access: find account session: %w", err)
	}
	return session, nil
}

func (m *Module) FindAccountSessionForUpdate(ctx context.Context, token string) (AccountSession, error) {
	session, err := m.engine.FindAccountSessionForUpdate(ctx, token)
	if err != nil {
		return AccountSession{}, fmt.Errorf("identity access: find account session for update: %w", err)
	}
	return session, nil
}

func (m *Module) LatestAccountSessionInFamily(ctx context.Context, familyID string) (AccountSession, error) {
	session, err := m.engine.LatestAccountSessionInFamily(ctx, familyID)
	if err != nil {
		return AccountSession{}, fmt.Errorf("identity access: latest account session in family: %w", err)
	}
	return session, nil
}

func (m *Module) ListAccountSessions(ctx context.Context, filter AccountSessionFilter) ([]AccountSession, error) {
	sessions, err := m.engine.ListAccountSessions(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("identity access: list account sessions: %w", err)
	}
	return sessions, nil
}

func (m *Module) CountExpiredAccountSessions(ctx context.Context) (int, error) {
	count, err := m.engine.CountExpiredAccountSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: count expired account sessions: %w", err)
	}
	return count, nil
}

func (m *Module) ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error) {
	ids, err := m.engine.ListInactiveAccountIDsWithLiveSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access: list inactive accounts with live sessions: %w", err)
	}
	return ids, nil
}

func (m *Module) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error) {
	exists, err := m.engine.HasLiveAccountSessionsCreatedAfter(ctx, accountID, since)
	if err != nil {
		return false, fmt.Errorf("identity access: has live account sessions created after: %w", err)
	}
	return exists, nil
}

func (m *Module) CreateAccountSession(ctx context.Context, session AccountSession) (AccountSession, error) {
	stored, err := m.engine.CreateAccountSession(ctx, session)
	if err != nil {
		return AccountSession{}, fmt.Errorf("identity access: create account session: %w", err)
	}
	return stored, nil
}

func (m *Module) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	if err := m.engine.MarkAccountSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt); err != nil {
		return fmt.Errorf("identity access: mark account session rotated: %w", err)
	}
	return nil
}

func (m *Module) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error {
	if err := m.engine.DeleteExpiredRotatedAccountSessions(ctx, accountID, now); err != nil {
		return fmt.Errorf("identity access: delete expired rotated account sessions: %w", err)
	}
	return nil
}

func (m *Module) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error {
	if err := m.engine.RetireAccountSessionFamily(ctx, accountID, familyID, expiry); err != nil {
		return fmt.Errorf("identity access: retire account session family: %w", err)
	}
	return nil
}

func (m *Module) EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int) ([]AccountSession, error) {
	evicted, err := m.engine.EnforceAccountSessionCap(ctx, accountID, portalScope, keep)
	if err != nil {
		return nil, fmt.Errorf("identity access: enforce account session cap: %w", err)
	}
	return evicted, nil
}

func (m *Module) DeleteAccountSession(ctx context.Context, id int64) error {
	if err := m.engine.DeleteAccountSession(ctx, id); err != nil {
		return fmt.Errorf("identity access: delete account session: %w", err)
	}
	return nil
}

func (m *Module) RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]AccountSession, error) {
	sessions, err := m.engine.RevokeAccountSessionFamily(ctx, familyID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account session family: %w", err)
	}
	return sessions, nil
}

func (m *Module) RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]AccountSession, error) {
	sessions, err := m.engine.RevokeAccountSessionsInTenant(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account sessions in tenant: %w", err)
	}
	return sessions, nil
}

func (m *Module) RevokeAllAccountSessions(ctx context.Context, accountID int64) ([]AccountSession, error) {
	sessions, err := m.engine.RevokeAllAccountSessions(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke all account sessions: %w", err)
	}
	return sessions, nil
}

func (m *Module) RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]AccountSession, error) {
	sessions, err := m.engine.RevokeAccountSessionsCreatedAtOrBefore(ctx, accountID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke account sessions created at or before: %w", err)
	}
	return sessions, nil
}

func (m *Module) RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]AccountSession, error) {
	sessions, err := m.engine.RevokeTenantAccountSessions(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("identity access: revoke tenant account sessions: %w", err)
	}
	return sessions, nil
}

func (m *Module) DeleteExpiredAccountSessions(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredAccountSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired account sessions: %w", err)
	}
	return deleted, nil
}
