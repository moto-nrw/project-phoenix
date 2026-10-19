package database

import (
	"log/slog"

	"github.com/uptrace/bun"
)

// BoundDatabase owns the pool and the immutable unit of work supplied at construction.
// Unit is opaque here: the SQL adapter does not import Tenant Runtime.
type BoundDatabase[Unit any] struct {
	pool       *bun.DB
	unitOfWork Unit
}

func NewBoundDatabase[Unit any](pool *bun.DB, unit Unit) BoundDatabase[Unit] {
	return BoundDatabase[Unit]{pool: pool, unitOfWork: unit}
}

// Pool is the opened pool the owner compositions take.
func (d BoundDatabase[Unit]) Pool() *bun.DB { return d.pool }

// UnitOfWork is the tenant unit of work bound to the pool.
func (d BoundDatabase[Unit]) UnitOfWork() Unit { return d.unitOfWork }

// Close releases the pool. A value without a pool has nothing to close.
func (d BoundDatabase[Unit]) Close() error {
	if d.pool == nil {
		return nil
	}
	return ClosePool(d.pool)
}

// AddQueryHook registers hook on the pool every component already holds.
func (d BoundDatabase[Unit]) AddQueryHook(hook bun.QueryHook) {
	AttachQueryHook(d.pool, hook)
}

// LogQueries logs every statement of the pool through logger; the server
// enables it with db_debug.
func (d BoundDatabase[Unit]) LogQueries(logger *slog.Logger) {
	d.AddQueryHook(NewQueryHook(logger))
}

// Capacity snapshots the pool's connection statistics for the metrics
// endpoint and the capacity log.
func (d BoundDatabase[Unit]) Capacity() CapacityStats {
	return SnapshotCapacity(d.pool)
}
