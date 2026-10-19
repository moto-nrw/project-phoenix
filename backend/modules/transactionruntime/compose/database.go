package compose

import (
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/database"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Database is the one database value the composition roots hold: the opened
// pool and the tenant unit of work bound to it. The roots hand its parts to the
// owner compositions and close it on shutdown, so no root names the ORM
// (ADR 0053, point 5).
type Database struct {
	pool       *bun.DB
	unitOfWork tenant.UnitOfWork
}

// NewDatabase binds the tenant unit of work to an open pool. Tests and the
// shared fixtures wrap their pool with it. Close closes that pool, so a
// caller that wraps a pool it shares does not close the value.
func NewDatabase(pool *bun.DB) (Database, error) {
	return bindDatabase(pool, NewTenantRuntime)
}

// NewDemoDatabase binds the standing demo's unit of work to an open pool.
func NewDemoDatabase(pool *bun.DB) (Database, error) {
	return bindDatabase(pool, NewDemoTenantRuntime)
}

// OpenDatabase opens the superuser pool of the commands with the tenant unit
// of work.
func OpenDatabase() (Database, error) {
	return openDatabase(OpenPool, NewDatabase)
}

// OpenServeDatabase opens the least-privilege phoenix_auth pool of the HTTP
// server with the tenant unit of work and reports lock waits to its observer.
func OpenServeDatabase() (Database, error) {
	return openDatabase(OpenServePool, func(pool *bun.DB) (Database, error) {
		ObserveLockWaits(pool)
		return NewDatabase(pool)
	})
}

// OpenDemoDatabase opens the standing demo's restricted pool with its unit of
// work.
func OpenDemoDatabase() (Database, error) {
	return openDatabase(database.DBConnForDemo, NewDemoDatabase)
}

// Pool is the opened pool the owner compositions take.
func (d Database) Pool() *bun.DB { return d.pool }

// UnitOfWork is the tenant unit of work bound to the pool.
func (d Database) UnitOfWork() tenant.UnitOfWork { return d.unitOfWork }

// Close releases the pool. A value without a pool has nothing to close.
func (d Database) Close() error {
	if d.pool == nil {
		return nil
	}
	return database.ClosePool(d.pool)
}

// AddQueryHook registers hook on the pool every component already holds.
func (d Database) AddQueryHook(hook bun.QueryHook) {
	database.AttachQueryHook(d.pool, hook)
}

// LogQueries logs every statement of the pool through logger; the server
// enables it with db_debug.
func (d Database) LogQueries(logger *slog.Logger) {
	d.AddQueryHook(database.NewQueryHook(logger))
}

// Capacity snapshots the pool's connection statistics for the metrics
// endpoint and the capacity log.
func (d Database) Capacity() database.CapacityStats {
	return database.SnapshotCapacity(d.pool)
}

func bindDatabase(pool *bun.DB, bind func(*bun.DB) (tenant.UnitOfWork, error)) (Database, error) {
	unitOfWork, err := bind(pool)
	if err != nil {
		return Database{}, err
	}
	return Database{pool: pool, unitOfWork: unitOfWork}, nil
}

func openDatabase(open func() (*bun.DB, error), bind func(*bun.DB) (Database, error)) (Database, error) {
	pool, err := open()
	if err != nil {
		return Database{}, err
	}
	value, err := bind(pool)
	if err != nil {
		_ = database.ClosePool(pool)
		return Database{}, err
	}
	return value, nil
}
