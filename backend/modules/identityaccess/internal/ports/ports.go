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
