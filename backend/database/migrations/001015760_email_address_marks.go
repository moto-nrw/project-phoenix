package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailAddressMarksVersion     = "1.15.760"
	emailAddressMarksDescription = "Markierte Adressen: Stopp der Schule und Prüfung bis zur Aufhebung (#5934)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailAddressMarksVersion,
		Description: emailAddressMarksDescription,
		DependsOn:   []string{emailDeliveryFallbackVersion},
	})
	Migrations.MustRegister(emailAddressMarksUp, emailAddressMarksDown)
}

// emailAddressMarksUpSQL lets a school stop mail to one address (#5934) and read
// the outcome of its mails by address:
//
//   - reason 'school_stop': the school's own decision, next to a bounce and a
//     complaint. The role writes the list now, always under the school's RLS.
//   - The index serves the lookup of the newest mail of a kind to an address.
//     Mails leave the outbox after 30 days, so the table stays small enough to
//     build it inside the migration.
const emailAddressMarksUpSQL = `
		ALTER TABLE platform.email_suppressions
			DROP CONSTRAINT email_suppressions_reason_check,
			ADD CONSTRAINT email_suppressions_reason_check
				CHECK (reason IN ('failed', 'complained', 'school_stop'));
		COMMENT ON TABLE platform.email_suppressions IS
			'Markierte Adressen einer Schule (#5932, #5934): endgültiger Fehler, Beschwerde oder Stopp der Schule. Die Markierung bleibt, bis jemand an der Schule die Adresse geprüft hat oder niemand an der Schule sie mehr führt.';
		GRANT INSERT, UPDATE ON platform.email_suppressions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE platform.email_suppressions_id_seq TO phoenix_tenant;
		CREATE INDEX idx_email_outbox_school_kind_address
			ON platform.email_outbox (tenant_id, kind, LOWER(TRIM(recipient->>'address')), id DESC)
			WHERE tenant_id IS NOT NULL;
`

// Going back removes the school's stops: the older code knows no such reason.
const emailAddressMarksDownSQL = `
		DROP INDEX IF EXISTS platform.idx_email_outbox_school_kind_address;
		DELETE FROM platform.email_suppressions WHERE reason = 'school_stop';
		REVOKE USAGE ON SEQUENCE platform.email_suppressions_id_seq FROM phoenix_tenant;
		REVOKE INSERT, UPDATE ON platform.email_suppressions FROM phoenix_tenant;
		COMMENT ON TABLE platform.email_suppressions IS
			'Adressen einer Schule mit endgültigem Fehler oder Beschwerde (#5932), ein Jahr nach dem letzten Ereignis gelöscht.';
		ALTER TABLE platform.email_suppressions
			DROP CONSTRAINT email_suppressions_reason_check,
			ADD CONSTRAINT email_suppressions_reason_check CHECK (reason IN ('failed', 'complained'));
`

func emailAddressMarksUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailAddressMarksUpSQL); err != nil {
			return fmt.Errorf("extend the email address marks: %w", err)
		}
		return nil
	})
}

func emailAddressMarksDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, emailAddressMarksDownSQL); err != nil {
			return fmt.Errorf("restore the email address marks: %w", err)
		}
		return nil
	})
}
