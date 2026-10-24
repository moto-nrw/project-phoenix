package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/database"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Database is the one database value the composition roots hold: the opened
// pool and the tenant unit of work bound to it. The roots hand its parts to the
// owner compositions and close it on shutdown, so no root names the ORM
// (ADR 0053, point 5).
type Database struct {
	database.BoundDatabase[tenant.UnitOfWork]
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
	return openDatabase(OpenServePool, database.ObservedBinder(NewDatabase, ObserveLockWaits))
}

// OpenDemoDatabase opens the standing demo's restricted pool with its unit of
// work.
func OpenDemoDatabase() (Database, error) {
	return openDatabase(database.DBConnForDemo, NewDemoDatabase)
}

func bindDatabase(pool *bun.DB, bind func(*bun.DB) (tenant.UnitOfWork, error)) (Database, error) {
	unitOfWork, err := bind(pool)
	if err != nil {
		return Database{}, err
	}
	return Database{BoundDatabase: database.NewBoundDatabase(pool, unitOfWork)}, nil
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
