package compose

import (
	"errors"
	"log/slog"
	"time"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/auditpostgres"
	auditService "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/application"
	auditModel "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Audit reads the ambient transaction of its producers itself (#4013): the
// compositions hand it the database, never a transaction they picked out of
// the context.

// NewTenantPersistence builds the Audit persistence of a service graph over
// the tenant runtime's ambient transaction. Appends need the caller's
// transaction. Reads and appends fail without the caller's transaction.
func NewTenantPersistence(db *bun.DB) (*Persistence, error) {
	if db == nil {
		return nil, errors.New("audit persistence: database is required")
	}

	return NewPersistence(PersistenceDependencies{
		Reads:   audit.NewHandoffRuntime(audit.NewTenantHandoff(tenant.FromContext, tenant.IsAdminTx, tenant.TransactionFromContext)),
		Appends: audit.NewHandoffRuntime(audit.NewTenantHandoff(tenant.FromContext, tenant.IsAdminTx, tenant.TransactionFromContext)),
	})
}

// NewHandoffPersistence builds the Audit persistence of a command root over
// the Audit handoff of its unit of work, without a root database: outside a
// transaction every write fails.
func NewHandoffPersistence() (*Persistence, error) {
	return NewPersistence(PersistenceDependencies{Reads: audit.NewHandoffRuntime(audit.ContextHandoff)})
}

// NewObservedCommand builds the fail-closed append command over store. Every
// append is reported to observe and logged, at error level when it failed.
func NewObservedCommand(store auditModel.AppendStore, logger *slog.Logger, observe func(eventType string, duration time.Duration, rows int, err error)) (*Command, error) {
	if store == nil || logger == nil || observe == nil {
		return nil, errors.New("audit command store, logger, and observer are required")
	}
	return NewCommand(store, auditService.NewLoggingObserver(logger, observe))
}
