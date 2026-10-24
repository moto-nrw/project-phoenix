package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	deliveryRetentionVersion     = "1.15.448"
	deliveryRetentionDescription = "Mail- und Push-Warteschlange und Zustellprotokolle: nächtliche Löschung nach 30 Tagen erlauben (#3955)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     deliveryRetentionVersion,
		Description: deliveryRetentionDescription,
		DependsOn:   []string{auditLedgerRetentionVersion},
	})
	Migrations.MustRegister(deliveryRetentionUp, deliveryRetentionDown)
}

// The subject check of audit.data_deletions (#3950) with and without the
// delivery retention, which logs a whole data kind without a subject.
const (
	deliveryRetentionUpSQL = `
		GRANT DELETE ON platform.push_outbox TO phoenix_tenant;
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention'
					)
				)
			);`
	deliveryRetentionDownSQL = `
		REVOKE DELETE ON platform.push_outbox FROM phoenix_tenant;
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention'
					)
				)
			);`
)

// deliveryRetentionUp lets the nightly retention of a school delete its
// delivered, finally failed or cancelled mail and push intents and their
// delivery log 30 days later (#3955), and log that as one row per run. The
// run holds the school's tenant transaction on phoenix_tenant, so RLS keeps
// it to that school. platform.email_outbox and platform.email_delivery
// already grant DELETE (1.15.180, 1.15.338); platform.push_outbox did not.
func deliveryRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, deliveryRetentionUpSQL); err != nil {
		return fmt.Errorf("allow delivery retention: %w", err)
	}
	return tx.Commit()
}

func deliveryRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = 'delivery_retention' AND student_id IS NULL AND staff_id IS NULL`).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check delivery retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d delivery retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, deliveryRetentionDownSQL); err != nil {
		return fmt.Errorf("revoke delivery retention: %w", err)
	}
	return nil
}
