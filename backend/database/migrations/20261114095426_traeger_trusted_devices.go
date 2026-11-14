package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerTrustedDevicesVersion = "20261114095426"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerTrustedDevicesVersion,
		Description: "Gemerkte Geräte des Träger-Portals je Träger (#6955)",
		DependsOn: []string{
			catererTrustedDevicesVersion, // a device without a school (1.15.688)
			traegerPortalVersion,         // the Träger memberships the device belongs to (1.15.514)
		},
	})
	Migrations.MustRegister(traegerTrustedDevicesUp, traegerTrustedDevicesDown)
}

// traegerTrustedDevicesUpSQL lets the Träger-Portal remember a device
// (#6955). A Träger session names its Träger and no school (ADR 0084), so
// its device names its Träger and no school: tenant_id stays NULL and
// organization_id holds the Träger. A device with neither is the caterer
// portal's (#4497). The CHECK keeps a row in one portal: no device is a
// school's and a Träger's at once. The school policy (1.15.476) still never
// shows a device without a school to a school's request role; the sign-in
// reads it with the pre-auth role. The device goes with its account
// (account_id ON DELETE CASCADE and the erasure functions, which delete by
// account) and with its Träger.
const traegerTrustedDevicesUpSQL = `
ALTER TABLE auth.mfa_trusted_devices
	ADD COLUMN organization_id BIGINT REFERENCES platform.organizations(id) ON DELETE CASCADE,
	ADD CONSTRAINT chk_mfa_trusted_devices_one_portal CHECK (tenant_id IS NULL OR organization_id IS NULL);
CREATE INDEX idx_mfa_trusted_devices_organization ON auth.mfa_trusted_devices (organization_id) WHERE organization_id IS NOT NULL;
COMMENT ON COLUMN auth.mfa_trusted_devices.organization_id IS
	'Träger, an dem das Gerät im Träger-Portal gemerkt ist; NULL bei Geräten einer Schule und des Caterer-Portals (#6955).';
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS
	'Schule, an der das Gerät gemerkt ist; NULL beim Caterer-Portal und beim Träger-Portal, deren zweiter Faktor an keiner Schule hängt (#4497, #6955).';
`

// traegerTrustedDevicesDownSQL forgets the Träger's remembered devices: a
// row without the column would read as the caterer portal's. The Träger-Büro
// then confirms its next sign-in with a code once more.
const traegerTrustedDevicesDownSQL = `
DELETE FROM auth.mfa_trusted_devices WHERE organization_id IS NOT NULL;
ALTER TABLE auth.mfa_trusted_devices DROP COLUMN organization_id;
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS
	'Schule, an der das Gerät gemerkt ist; NULL beim Caterer-Portal, dessen zweiter Faktor an keiner Schule hängt (#4497).';
`

func traegerTrustedDevicesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerTrustedDevicesUpSQL); err != nil {
			return fmt.Errorf("bind trusted devices to a Träger: %w", err)
		}
		return nil
	})
}

func traegerTrustedDevicesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerTrustedDevicesDownSQL); err != nil {
			return fmt.Errorf("unbind trusted devices from a Träger: %w", err)
		}
		return nil
	})
}
