package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The unit of work and the observation the flows run in.

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction. A missing school is an error.
	RunRead(context.Context, func(context.Context) error) error
	// RunPlatform joins the caller's transaction when one is active and
	// otherwise opens an explicit administrative transaction. Operator rows are platform-wide
	// and carry no tenant, so no tenant transaction is opened for them; the
	// operator flows open their own administrative transaction where several
	// writes must commit together.
	RunPlatform(context.Context, func(context.Context) error) error
	// RunAdmin is the explicit operator/cross-school worker boundary.
	RunAdmin(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)

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
	// AcquireLock takes a transaction-scoped advisory lock.
	AcquireLock(ctx context.Context, key string) error
}
