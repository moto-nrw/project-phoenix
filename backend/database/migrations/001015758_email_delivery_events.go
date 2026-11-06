package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailDeliveryEventsVersion     = "1.15.758"
	emailDeliveryEventsDescription = "Zustellereignisse, Sperrliste und Vorrang für Sicherheitsmails (#5932)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailDeliveryEventsVersion,
		Description: emailDeliveryEventsDescription,
		DependsOn: []string{
			payrollWageTypesVersion, // extends its erasure functions
			storedNamesNFCVersion,   // the last migration before this one
		},
	})
	Migrations.MustRegister(emailDeliveryEventsUp, emailDeliveryEventsDown)
}

// emailDeliveryEventsUpSQL lets a mail be matched to what happens to it after
// the mail server accepted it (#5932):
//
//   - delivery_token is the random value of the X-Moto-Outbox-Id header and the
//     local part of the Message-ID. Recipients see it, so it is neither the row
//     ID nor anything about the school. It stays the same across retries.
//   - message_id and provider_message_id are stored when the server accepts.
//   - delivery_state is the summary of the mail's events, NULL before the first.
//   - security marks sign-in codes, password resets and staff invitations,
//     which the worker claims first and which the suppression list never holds
//     back.
//
// platform.email_delivery_events keeps each reported event once. tenant_id is
// NULL for system and Träger mail, as on the mail itself; those rows are for
// the administrative worker only. The rows go with their mail.
//
// platform.email_suppressions lists a school's addresses that bounced for good
// or complained. The address is compared in lower case.
//
// platform.delivery_known_email_addresses is Delivery's projection for the
// nightly retention: an address no guardian or account of the school holds any
// more leaves the list, so deleting a person also ends the entry.
const emailDeliveryEventsUpSQL = `
		ALTER TABLE platform.email_outbox
			ADD COLUMN delivery_token TEXT NOT NULL DEFAULT uuid_generate_v4()::text,
			ADD COLUMN message_id TEXT,
			ADD COLUMN provider_message_id TEXT,
			ADD COLUMN security BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN delivery_state TEXT,
			ADD COLUMN delivery_state_at TIMESTAMPTZ,
			ADD CONSTRAINT email_outbox_delivery_state_check CHECK (delivery_state IN
				('delayed', 'delivered', 'suppressed', 'failed', 'complained'));
		CREATE UNIQUE INDEX uq_email_outbox_delivery_token
			ON platform.email_outbox (delivery_token);
		CREATE INDEX idx_email_outbox_message_id
			ON platform.email_outbox (message_id) WHERE message_id IS NOT NULL;
		CREATE INDEX idx_email_outbox_provider_message_id
			ON platform.email_outbox (provider_message_id) WHERE provider_message_id IS NOT NULL;

		CREATE TABLE platform.email_delivery_events (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT REFERENCES platform.schools(id) ON DELETE CASCADE,
			outbox_id         BIGINT NOT NULL REFERENCES platform.email_outbox(id) ON DELETE CASCADE,
			provider          TEXT NOT NULL,
			provider_event_id TEXT NOT NULL,
			state             TEXT NOT NULL CHECK (state IN
				('delayed', 'delivered', 'suppressed', 'failed', 'complained')),
			occurred_at       TIMESTAMPTZ NOT NULL,
			diagnostic        TEXT,
			received_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_email_delivery_events_provider_event
				UNIQUE (outbox_id, provider, provider_event_id)
		);
		CREATE INDEX idx_email_delivery_events_tenant
			ON platform.email_delivery_events (tenant_id);
		COMMENT ON TABLE platform.email_delivery_events IS
			'Zustellereignisse je Mail (#5932), mit der Mail gelöscht.';

		CREATE TABLE platform.email_suppressions (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			address    TEXT NOT NULL CHECK (address = LOWER(address) AND address <> ''),
			reason     TEXT NOT NULL CHECK (reason IN ('failed', 'complained')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_email_suppressions_tenant_address UNIQUE (tenant_id, address)
		);
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932), ein Jahr nach dem letzten Ereignis gelöscht.';

		CREATE VIEW platform.delivery_known_email_addresses
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT guardian.tenant_id, LOWER(guardian.email) AS address
		FROM users.guardian_profiles AS guardian
		WHERE guardian.email IS NOT NULL
		UNION
		SELECT account_tenant.tenant_id, LOWER(account.email) AS address
		FROM auth.account_tenants AS account_tenant
		INNER JOIN auth.accounts AS account ON account.id = account_tenant.account_id
		WHERE account.email IS NOT NULL;
		REVOKE ALL ON platform.delivery_known_email_addresses FROM PUBLIC, phoenix_tenant;
		GRANT SELECT ON platform.delivery_known_email_addresses TO phoenix_tenant;

		REVOKE ALL ON platform.email_delivery_events FROM PUBLIC, phoenix_tenant;
		GRANT SELECT, DELETE ON platform.email_delivery_events TO phoenix_tenant;
		REVOKE ALL ON platform.email_suppressions FROM PUBLIC, phoenix_tenant;
		GRANT SELECT, DELETE ON platform.email_suppressions TO phoenix_tenant;
`

const emailDeliveryEventsDownSQL = `
		DROP VIEW IF EXISTS platform.delivery_known_email_addresses;
		DROP TABLE IF EXISTS platform.email_suppressions;
		DROP TABLE IF EXISTS platform.email_delivery_events;
		DROP INDEX IF EXISTS platform.idx_email_outbox_provider_message_id;
		DROP INDEX IF EXISTS platform.idx_email_outbox_message_id;
		DROP INDEX IF EXISTS platform.uq_email_outbox_delivery_token;
		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT IF EXISTS email_outbox_delivery_state_check,
			DROP COLUMN IF EXISTS delivery_state_at,
			DROP COLUMN IF EXISTS delivery_state,
			DROP COLUMN IF EXISTS security,
			DROP COLUMN IF EXISTS provider_message_id,
			DROP COLUMN IF EXISTS message_id,
			DROP COLUMN IF EXISTS delivery_token;
`

func emailDeliveryEventsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailDeliveryEventsUpSQL); err != nil {
			return fmt.Errorf("create the email delivery events: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx,
			"platform.email_delivery_events",
			"platform.email_suppressions",
		); err != nil {
			return err
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installEmailDeliveryEventErasurePlan, installEmailDeliveryEventErasurePreflight, installEmailDeliveryEventErasureResidual,
			installEmailDeliveryEventErasureDelete, installEmailDeliveryEventErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the email delivery event school erasure: %w", err)
			}
		}
		return nil
	})
}

// emailDeliveryEventsDown restores the 1.15.733 erasure before it drops the
// tables, so the installed plan never names a missing table.
func emailDeliveryEventsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installPayrollWageTypeErasurePlan, installPayrollWageTypeErasurePreflight, installPayrollWageTypeErasureResidual,
			installPayrollWageTypeErasureDelete, installPayrollWageTypeErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the school erasure of 1.15.733: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, emailDeliveryEventsDownSQL); err != nil {
			return fmt.Errorf("drop the email delivery events: %w", err)
		}
		return nil
	})
}
