package owners

import (
	"context"

	transactionruntimeCompose "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CommunicationRequestRuntime binds ambient school identity and commit hooks.
type CommunicationRequestRuntime struct{}

func (CommunicationRequestRuntime) TenantID(ctx context.Context) int64 {
	return tenant.FromContext(ctx)
}

func (CommunicationRequestRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

// CommunicationEventRuntime retains the root's unit of work for detached pills.
type CommunicationEventRuntime struct {
	unit       tenant.UnitOfWork
	configured bool
}

// NewCommunicationEventRuntime binds the unit of work and existing pool-presence
// guard without exposing an ORM handle to the root.
func NewCommunicationEventRuntime(configured bool, unit tenant.UnitOfWork) *CommunicationEventRuntime {
	return &CommunicationEventRuntime{unit: unit, configured: configured}
}

func (r *CommunicationEventRuntime) BackgroundContext() context.Context {
	return tenant.WithUnitOfWork(context.Background(), r.unit)
}

func (*CommunicationEventRuntime) InTransaction(ctx context.Context) bool {
	_, active := tenant.TransactionFromContext(ctx)
	return active
}

func (r *CommunicationEventRuntime) WithinTenant(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(tenant.WithUnitOfWork(ctx, r.unit), r.configured, schoolID, transactionruntimeCompose.TenantCallback(fn))
}

func (r *CommunicationEventRuntime) Configured() bool { return r != nil && r.configured }

func (*CommunicationEventRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}
