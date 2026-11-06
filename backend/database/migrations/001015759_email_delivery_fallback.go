package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailDeliveryFallbackVersion     = "1.15.759"
	emailDeliveryFallbackDescription = "Notweg und Abgleich der Zustellung (#5933)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailDeliveryFallbackVersion,
		Description: emailDeliveryFallbackDescription,
		DependsOn:   []string{emailDeliveryEventsVersion},
	})
	Migrations.MustRegister(emailDeliveryFallbackUp, emailDeliveryFallbackDown)
}

// emailDeliveryFallbackUpSQL adds two columns (#5933):
//
//   - fallback_requested_at: the provider reported that it could not deliver a
//     security mail, which then goes out once over the fallback transport.
//   - delivery_checked_at: when the reconciliation last asked the provider. It
//     rotates through the open mails.
//
// Both are nullable, so the table is not rewritten. The index holds only the
// open mails the reconciliation asks about.
const emailDeliveryFallbackUpSQL = `
		ALTER TABLE platform.email_outbox
			ADD COLUMN fallback_requested_at TIMESTAMPTZ,
			ADD COLUMN delivery_checked_at TIMESTAMPTZ;
		CREATE INDEX idx_email_outbox_open_provider_mail
			ON platform.email_outbox (delivery_checked_at NULLS FIRST, id)
			WHERE status = 'sent' AND provider_message_id IS NOT NULL AND fallback_requested_at IS NULL
				AND (delivery_state IS NULL OR delivery_state = 'delayed');
`

const emailDeliveryFallbackDownSQL = `
		DROP INDEX IF EXISTS platform.idx_email_outbox_open_provider_mail;
		ALTER TABLE platform.email_outbox
			DROP COLUMN IF EXISTS delivery_checked_at,
			DROP COLUMN IF EXISTS fallback_requested_at;
`

func emailDeliveryFallbackUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailDeliveryFallbackUpSQL); err != nil {
			return fmt.Errorf("add the email delivery fallback columns: %w", err)
		}
		return nil
	})
}

func emailDeliveryFallbackDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailDeliveryFallbackDownSQL); err != nil {
			return fmt.Errorf("remove the email delivery fallback columns: %w", err)
		}
		return nil
	})
}
