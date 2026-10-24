// Package cleanupruns keeps the last successful nightly cleanup run of each
// job and school in platform.cleanup_runs (#3950). Every statement joins the
// tenant transaction already open in its context and takes the school from
// that transaction, so a recorded run commits only together with its
// deletions and never names another school.
package cleanupruns

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Store reads and records the last successful run. Every statement is a
// compile-time constant so the architecture evaluator can read which tables
// it touches.
type Store struct{}

// NewStore builds the store. It holds no connection: it joins the
// transaction of each call.
func NewStore() *Store { return &Store{} }

const lastSucceededSQL = `SELECT succeeded_at FROM platform.cleanup_runs
WHERE tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND job_id = ?`

const recordSucceededSQL = `INSERT INTO platform.cleanup_runs AS run (tenant_id, job_id, succeeded_at)
VALUES (NULLIF(current_setting('app.current_tenant_id', true), '')::bigint, ?, ?)
ON CONFLICT (tenant_id, job_id) DO UPDATE
SET succeeded_at = GREATEST(run.succeeded_at, EXCLUDED.succeeded_at)`

// LastCleanupSucceeded returns when jobID last succeeded for the school of
// the transaction in ctx, and false when it never did.
func (s *Store) LastCleanupSucceeded(ctx context.Context, jobID string) (time.Time, bool, error) {
	db, err := tenantTransaction(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	var succeededAt time.Time
	err = db.NewRaw(lastSucceededSQL, jobID).Scan(ctx, &succeededAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read last cleanup run %s: %w", jobID, err)
	}
	return succeededAt, true, nil
}

// RecordCleanupSucceeded stores at as the last success of jobID for the
// school of the transaction in ctx. An older time never replaces a newer one.
func (s *Store) RecordCleanupSucceeded(ctx context.Context, jobID string, at time.Time) error {
	if jobID == "" || at.IsZero() {
		return errors.New("cleanup run: job and time are required")
	}
	db, err := tenantTransaction(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(recordSucceededSQL, jobID, at).Exec(ctx); err != nil {
		return fmt.Errorf("record cleanup run %s: %w", jobID, err)
	}
	return nil
}

// tenantTransaction returns the transaction open in ctx. Outside a tenant
// transaction app.current_tenant_id is unset: the read finds nothing and the
// insert violates tenant_id NOT NULL, so no row lands without a school.
func tenantTransaction(ctx context.Context) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("cleanup run: a tenant transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("cleanup run: unsupported transaction %T", transaction)
	}
	return tx, nil
}
