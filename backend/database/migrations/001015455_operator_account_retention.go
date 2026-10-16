package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	operatorAccountRetentionVersion     = "1.15.455"
	operatorAccountRetentionDescription = "Operator-Konten deaktivieren und nach der Frist des Operator-Protokolls löschen (#4047)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operatorAccountRetentionVersion,
		Description: operatorAccountRetentionDescription,
		DependsOn: []string{
			protocolRetentionVersion,                // previous migration
			remediateLostCollisionMigrationsVersion, // last change to platform.announcements
		},
	})
	Migrations.MustRegister(operatorAccountRetentionUp, operatorAccountRetentionDown)
}

// operatorAccountRetentionUpSQL prepares the deletion of operator accounts of
// team members who left (#4047).
//
//   - platform.operators.deactivated_at records when an account was
//     deactivated; the hourly cleanup deletes the account once the operator
//     ledger has lost the entries before that moment. Accounts deactivated by
//     hand before this migration count from their last update. A trigger
//     keeps the column in step with active on every path, the dashboard as
//     well as an update by hand: a deactivation without a time gets the
//     current one, a reactivation clears it. The check holds the result.
//   - platform.announcements.created_by no longer blocks that deletion: an
//     announcement stays and loses its author.
//
// platform.operator_audit_log.operator_id keeps its foreign key without an
// action on purpose: an account whose ledger entries are still there cannot
// be deleted. It is the only reference to platform.operators that refuses a
// deletion: suggestions.operator_comments went with migration 1.12.1, whose
// suggestions.comments names its author without a foreign key.
const operatorAccountRetentionUpSQL = `
		ALTER TABLE platform.operators ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
		UPDATE platform.operators SET deactivated_at = updated_at WHERE NOT active AND deactivated_at IS NULL;

		CREATE OR REPLACE FUNCTION platform.track_operator_deactivation()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF NEW.active THEN
				NEW.deactivated_at := NULL;
			ELSIF NEW.deactivated_at IS NULL THEN
				NEW.deactivated_at := clock_timestamp();
			END IF;
			RETURN NEW;
		END;
		$$;
		DROP TRIGGER IF EXISTS track_operator_deactivation ON platform.operators;
		CREATE TRIGGER track_operator_deactivation
			BEFORE INSERT OR UPDATE OF active, deactivated_at ON platform.operators
			FOR EACH ROW EXECUTE FUNCTION platform.track_operator_deactivation();

		ALTER TABLE platform.operators DROP CONSTRAINT IF EXISTS chk_operators_deactivated_at;
		ALTER TABLE platform.operators ADD CONSTRAINT chk_operators_deactivated_at
			CHECK (active = (deactivated_at IS NULL));

		ALTER TABLE platform.announcements ALTER COLUMN created_by DROP NOT NULL;
		ALTER TABLE platform.announcements DROP CONSTRAINT IF EXISTS announcements_created_by_fkey;
		ALTER TABLE platform.announcements ADD CONSTRAINT announcements_created_by_fkey
			FOREIGN KEY (created_by) REFERENCES platform.operators(id) ON DELETE SET NULL;
`

const operatorAccountRetentionDownSQL = `
		ALTER TABLE platform.announcements DROP CONSTRAINT IF EXISTS announcements_created_by_fkey;
		ALTER TABLE platform.announcements ADD CONSTRAINT announcements_created_by_fkey
			FOREIGN KEY (created_by) REFERENCES platform.operators(id);
		ALTER TABLE platform.announcements ALTER COLUMN created_by SET NOT NULL;

		DROP TRIGGER IF EXISTS track_operator_deactivation ON platform.operators;
		DROP FUNCTION IF EXISTS platform.track_operator_deactivation();
		ALTER TABLE platform.operators DROP CONSTRAINT IF EXISTS chk_operators_deactivated_at;
		ALTER TABLE platform.operators DROP COLUMN IF EXISTS deactivated_at;
`

func operatorAccountRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackOperatorAccountRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, operatorAccountRetentionUpSQL); err != nil {
		return fmt.Errorf("prepare operator account retention: %w", err)
	}
	return tx.Commit()
}

func operatorAccountRetentionDown(ctx context.Context, db *bun.DB) error {
	var authorless int
	if err := db.NewRaw(`SELECT COUNT(*) FROM platform.announcements WHERE created_by IS NULL`).Scan(ctx, &authorless); err != nil {
		return fmt.Errorf("check announcements without author: %w", err)
	}
	if authorless > 0 {
		return fmt.Errorf("cannot require an announcement author while %d announcements have none", authorless)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackOperatorAccountRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, operatorAccountRetentionDownSQL); err != nil {
		return fmt.Errorf("revert operator account retention: %w", err)
	}
	return tx.Commit()
}

func rollbackOperatorAccountRetention(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
