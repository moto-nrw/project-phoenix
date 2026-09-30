package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentRequestReadsVersion     = "1.15.432"
	enrollmentRequestReadsDescription = "Create enrollment.request_reads and enrollment.requests.parent_changed_at - per-account read state of enrollments (#3778)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRequestReadsVersion,
		Description: enrollmentRequestReadsDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion, // enrollment.requests
			AuthAccountsVersion,             // auth.accounts (readers)
			dropPersonalPINColumnsVersion,   // latest migration at authoring time
		},
	})

	Migrations.MustRegister(enrollmentRequestReadsUp, enrollmentRequestReadsDown)
}

// enrollmentRequestReadsUp stores which account has read which enrollment
// request (#3778). An unread enrollment is one without a read row, or whose
// read row is older than the request's last parent change. parent_changed_at
// moves only when parents submit, edit, replace or confirm a renewal; OGS
// decisions and worker transitions leave it alone.
//
// Existing requests take their submission time, so every open enrollment in
// an active phase starts unread for everyone.
func enrollmentRequestReadsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE enrollment.requests
			ADD COLUMN IF NOT EXISTS parent_changed_at TIMESTAMPTZ;
		UPDATE enrollment.requests SET parent_changed_at = submitted_at WHERE parent_changed_at IS NULL;
		ALTER TABLE enrollment.requests
			ALTER COLUMN parent_changed_at SET DEFAULT NOW(),
			ALTER COLUMN parent_changed_at SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error adding enrollment.requests.parent_changed_at: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS enrollment.request_reads (
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
			request_id BIGINT NOT NULL REFERENCES enrollment.requests(id) ON DELETE CASCADE,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (request_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_enrollment_request_reads_account
			ON enrollment.request_reads (tenant_id, account_id);

		GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.request_reads TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating enrollment.request_reads: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return provisionTenantRLS(ctx, db, "enrollment.request_reads")
}

func enrollmentRequestReadsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS enrollment.request_reads;
		ALTER TABLE enrollment.requests DROP COLUMN IF EXISTS parent_changed_at;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error dropping enrollment request read state: %w", err)
	}
	return nil
}
