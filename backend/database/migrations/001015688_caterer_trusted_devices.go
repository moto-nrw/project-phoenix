package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const catererTrustedDevicesVersion = "1.15.688"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     catererTrustedDevicesVersion,
		Description: "Vertrauenswürdige Geräte des Caterer-Portals ohne Schule (#4497)",
		DependsOn: []string{
			authTenantRLSVersion,        // the school policy on auth.mfa_trusted_devices
			schoolAbsenceReportsVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(catererTrustedDevicesUp, catererTrustedDevicesDown)
}

// catererTrustedDevicesUpSQL lets the caterer portal remember a device
// (#4497, ADR 0077 point 7). A caterer's second factor belongs to its
// account, not to a school, so its remembered device names none: tenant_id
// stays NULL. A device of a school keeps its school and stays bound to it;
// the school policy never shows a device without a school to a school's
// request role, and the sign-in reads it with the pre-auth role, whose
// policy is unchanged. A device without a school is matched only by the
// caterer portal, which asks for tenant_id IS NULL.
const catererTrustedDevicesUpSQL = `
ALTER TABLE auth.mfa_trusted_devices ALTER COLUMN tenant_id DROP NOT NULL;
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS
	'Schule, an der das Gerät gemerkt ist; NULL beim Caterer-Portal, dessen zweiter Faktor an keiner Schule hängt (#4497).';
`

// catererTrustedDevicesDownSQL forgets the caterer's remembered devices: a
// school column without a value cannot become NOT NULL again. The caterer
// then confirms its next sign-in with a code once more.
const catererTrustedDevicesDownSQL = `
DELETE FROM auth.mfa_trusted_devices WHERE tenant_id IS NULL;
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS NULL;
ALTER TABLE auth.mfa_trusted_devices ALTER COLUMN tenant_id SET NOT NULL;
`

func catererTrustedDevicesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, catererTrustedDevicesUpSQL); err != nil {
			return fmt.Errorf("allow trusted devices without a school: %w", err)
		}
		return nil
	})
}

func catererTrustedDevicesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, catererTrustedDevicesDownSQL); err != nil {
			return fmt.Errorf("require a school on trusted devices: %w", err)
		}
		return nil
	})
}
