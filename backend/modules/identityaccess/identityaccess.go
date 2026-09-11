// Package identityaccess is the public Identity & Access capability other
// owners consume. The full Identity & Access migration comes late in #2580;
// this package exposes the narrow facts and commands earlier cutovers need
// just in time, starting with guardian portal access for enrollment
// acceptance (#2699) and the platform operator identity and refresh
// sessions behind operator login, refresh and revocation (#2720).
//
// Accounts are platform-wide rows without a tenant. The school mapping
// (`auth.account_tenants`) and the guardian base role assignment
// (`auth.account_roles`) belong to the tenant in context. Operators and
// their refresh sessions are platform-wide as well and never carry a tenant.
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
)

// ErrorCode maps a capability error to the stable code recorded in metrics.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrGuardianRoleMissing),
		errors.Is(err, ErrOperatorNotFound), errors.Is(err, ErrOperatorSessionNotFound):
		return "not_found"
	case errors.Is(err, ErrOperatorSessionRotated):
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

// Engine is the composed implementation behind the public module.
type Engine interface {
	GuardianAccess
	OperatorAccess
}

// Module is the public Identity & Access facade.
type Module struct {
	engine Engine
}

// NewModule wraps the composed engine. Composition supplies it; consumers
// depend on the interfaces above.
func NewModule(engine Engine) *Module {
	if engine == nil {
		panic("identity access: engine is required")
	}
	return &Module{engine: engine}
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
