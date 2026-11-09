// Package cleanupruns keeps the last successful nightly cleanup run of each
// job and school in platform.cleanup_runs (#3950), together with the runs
// that failed in a row since (#6327). Every statement joins the
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

// Store reads and records the runs. Every statement is a compile-time
// constant so the architecture evaluator can read which tables it touches.
type Store struct{}

// NewStore builds the store. It holds no connection: it joins the
// transaction of each call.
func NewStore() *Store { return &Store{} }

const cleanupRunSQL = `SELECT succeeded_at, consecutive_failures, last_failed_at FROM platform.cleanup_runs
WHERE tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  AND job_id = ?`

// recordSucceededSQL also ends the failure streak: a success makes the
// failures before it irrelevant for the retry delay and the alert.
const recordSucceededSQL = `INSERT INTO platform.cleanup_runs AS run (tenant_id, job_id, succeeded_at)
VALUES (NULLIF(current_setting('app.current_tenant_id', true), '')::bigint, ?, ?)
ON CONFLICT (tenant_id, job_id) DO UPDATE
SET succeeded_at = GREATEST(run.succeeded_at, EXCLUDED.succeeded_at),
    consecutive_failures = 0,
    last_failed_at = NULL`

const recordFailedSQL = `INSERT INTO platform.cleanup_runs AS run (tenant_id, job_id, consecutive_failures, last_failed_at)
VALUES (NULLIF(current_setting('app.current_tenant_id', true), '')::bigint, ?, 1, ?)
ON CONFLICT (tenant_id, job_id) DO UPDATE
SET consecutive_failures = run.consecutive_failures + 1,
    last_failed_at = GREATEST(run.last_failed_at, EXCLUDED.last_failed_at)
RETURNING consecutive_failures`

// CleanupRun returns the last success of jobID for the school of the
// transaction in ctx, the failed runs in a row since and the last of them.
// Zero times mean none; a job that never ran there has neither.
func (s *Store) CleanupRun(ctx context.Context, jobID string) (time.Time, int, time.Time, error) {
	db, err := tenantTransaction(ctx)
	if err != nil {
		return time.Time{}, 0, time.Time{}, err
	}
	var (
		succeededAt, failedAt sql.NullTime
		failures              int
	)
	err = db.NewRaw(cleanupRunSQL, jobID).Scan(ctx, &succeededAt, &failures, &failedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, 0, time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, 0, time.Time{}, fmt.Errorf("read cleanup run %s: %w", jobID, err)
	}
	return succeededAt.Time, failures, failedAt.Time, nil
}

// RecordCleanupSucceeded stores at as the last success of jobID for the
// school of the transaction in ctx and ends its failure streak. An older
// time never replaces a newer one.
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

// RecordCleanupFailed adds a failed run at at to the streak of jobID for the
// school of the transaction in ctx and returns the streak's new length. The
// last success stays as it is.
func (s *Store) RecordCleanupFailed(ctx context.Context, jobID string, at time.Time) (int, error) {
	if jobID == "" || at.IsZero() {
		return 0, errors.New("cleanup run: job and time are required")
	}
	db, err := tenantTransaction(ctx)
	if err != nil {
		return 0, err
	}
	var failures int
	if err := db.NewRaw(recordFailedSQL, jobID, at).Scan(ctx, &failures); err != nil {
		return 0, fmt.Errorf("record failed cleanup run %s: %w", jobID, err)
	}
	return failures, nil
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
