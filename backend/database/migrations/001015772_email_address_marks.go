package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailAddressMarksVersion     = "1.15.772"
	emailAddressMarksDescription = "Zustellstand für Schulen: Stopp je Elternkontakt, Abmeldung, Markierung bis zur Prüfung (#5934)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailAddressMarksVersion,
		Description: emailAddressMarksDescription,
		DependsOn:   []string{emailDeliveryFallbackVersion},
	})
	Migrations.MustRegister(emailAddressMarksUp, emailAddressMarksDown)
}

// emailAddressMarksUpSQL (#5934):
//
//   - users.guardian_profiles.emails_stopped_at: the school decided that this
//     person gets no school mail. It belongs to the person, not the address, so it
//     survives a changed address and touches nobody else. Nullable, no rewrite.
//   - platform.delivery_guardian_contacts: Delivery's projection of the people who
//     hold an address. A person holds the address on the contact and, with an
//     account, the account's address. address is NULL for a contact without one.
//   - platform.delivery_traeger_schools: the schools of a Träger, so a Träger's mail
//     to a parent can honour the list and the stop of those schools.
//   - delivery_state 'unsubscribed': a person's wish, not a broken address.
//   - The index serves the lookup of the newest mail of a kind to an address. Mails
//     leave the outbox after 30 days, so the table stays small enough to build it
//     inside the migration.
const emailAddressMarksUpSQL = `
		ALTER TABLE users.guardian_profiles ADD COLUMN emails_stopped_at TIMESTAMPTZ;
		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT email_outbox_delivery_state_check,
			ADD CONSTRAINT email_outbox_delivery_state_check CHECK (delivery_state IN
				('delayed', 'delivered', 'suppressed', 'unsubscribed', 'failed', 'complained'));
		ALTER TABLE platform.email_delivery_events
			DROP CONSTRAINT email_delivery_events_state_check,
			ADD CONSTRAINT email_delivery_events_state_check CHECK (state IN
				('delayed', 'delivered', 'suppressed', 'unsubscribed', 'failed', 'complained'));
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932, #5934). Die Markierung bleibt ohne feste Frist, bis jemand an der Schule die Adresse geprüft hat oder niemand an der Schule sie mehr führt.';
		CREATE VIEW platform.delivery_guardian_contacts
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT guardian.tenant_id, guardian.id AS guardian_profile_id, 'profile'::text AS source,
			LOWER(TRIM(guardian.email)) AS address, guardian.emails_stopped_at
		FROM users.guardian_profiles AS guardian
		UNION ALL
		SELECT guardian.tenant_id, guardian.id, 'account'::text,
			LOWER(TRIM(account.email)), guardian.emails_stopped_at
		FROM users.guardian_profiles AS guardian
		INNER JOIN auth.accounts AS account ON account.id = guardian.account_id
		WHERE account.email IS NOT NULL;
		REVOKE ALL ON platform.delivery_guardian_contacts FROM PUBLIC, phoenix_tenant;
		GRANT SELECT ON platform.delivery_guardian_contacts TO phoenix_tenant;
		CREATE VIEW platform.delivery_traeger_schools
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT school.organization_id, school.id AS tenant_id
		FROM platform.schools AS school
		WHERE school.organization_id IS NOT NULL;
		REVOKE ALL ON platform.delivery_traeger_schools FROM PUBLIC, phoenix_tenant;
		GRANT SELECT ON platform.delivery_traeger_schools TO phoenix_tenant;
		CREATE INDEX idx_email_outbox_school_kind_address
			ON platform.email_outbox (tenant_id, kind, LOWER(TRIM(recipient->>'address')), id DESC)
			WHERE tenant_id IS NOT NULL;
`

const emailAddressMarksDownSQL = `
		DROP INDEX IF EXISTS platform.idx_email_outbox_school_kind_address;
		DROP VIEW IF EXISTS platform.delivery_traeger_schools;
		DROP VIEW IF EXISTS platform.delivery_guardian_contacts;
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932), ein Jahr nach dem letzten Ereignis gelöscht.';
		UPDATE platform.email_outbox SET delivery_state = 'suppressed' WHERE delivery_state = 'unsubscribed';
		UPDATE platform.email_delivery_events SET state = 'suppressed' WHERE state = 'unsubscribed';
		ALTER TABLE platform.email_delivery_events
			DROP CONSTRAINT email_delivery_events_state_check,
			ADD CONSTRAINT email_delivery_events_state_check CHECK (state IN
				('delayed', 'delivered', 'suppressed', 'failed', 'complained'));
		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT email_outbox_delivery_state_check,
			ADD CONSTRAINT email_outbox_delivery_state_check CHECK (delivery_state IN
				('delayed', 'delivered', 'suppressed', 'failed', 'complained'));
		ALTER TABLE users.guardian_profiles DROP COLUMN IF EXISTS emails_stopped_at;
`

func emailAddressMarksUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailAddressMarksUpSQL); err != nil {
			return fmt.Errorf("extend the delivery state of a school: %w", err)
		}
		return nil
	})
}

func emailAddressMarksDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailAddressMarksDownSQL); err != nil {
			return fmt.Errorf("restore the delivery state of a school: %w", err)
		}
		return nil
	})
}
