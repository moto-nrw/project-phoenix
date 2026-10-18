package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/auditpostgres"
	auditModel "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Audit reads the ambient transaction of its producers itself (#4013): the
// compositions hand it the database, never a transaction they picked out of
// the context.

// NewTenantPersistence builds the Audit persistence of a service graph over
// the tenant runtime's ambient transaction. Appends need the caller's
// transaction; rootWrites lets a test graph append on db outside one. Reads
// fall back to db.
func NewTenantPersistence(db *bun.DB, rootWrites bool) (*Persistence, error) {
	if db == nil {
		return nil, errors.New("audit persistence: database is required")
	}
	appendFallback := (*bun.DB)(nil)
	if rootWrites {
		appendFallback = db
	}
	return NewPersistence(PersistenceDependencies{
		Reads:   audit.NewHandoffRuntime(tenantHandoff, db),
		Appends: audit.NewHandoffRuntime(tenantHandoff, appendFallback),
	})
}

// NewHandoffPersistence builds the Audit persistence of a command root over
// the Audit handoff of its unit of work, without a root database: outside a
// transaction every write fails.
func NewHandoffPersistence() (*Persistence, error) {
	return NewPersistence(PersistenceDependencies{Reads: audit.NewHandoffRuntime(audit.ContextHandoff, nil)})
}

func tenantHandoff(ctx context.Context) (any, bool, int64) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	return transaction, ok, tenant.FromContext(ctx)
}

// NewObservedCommand builds the fail-closed append command over store. Every
// append is reported to observe and logged, at error level when it failed.
func NewObservedCommand(store auditModel.AppendStore, logger *slog.Logger, observe func(eventType string, duration time.Duration, rows int, err error)) (*Command, error) {
	if store == nil || logger == nil || observe == nil {
		return nil, errors.New("audit command store, logger, and observer are required")
	}
	return NewCommand(store, func(observation AppendObservation) {
		observe(observation.EventType, observation.Duration, observation.Rows, observation.Err)
		log := logger.Debug
		if observation.Err != nil {
			log = logger.Error
		}
		log("audit append",
			"event_type", observation.EventType,
			"duration_ms", observation.Duration.Milliseconds(),
			"rows", observation.Rows,
			"failed", observation.Err != nil,
		)
	})
}
