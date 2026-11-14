package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const mfaTrustedDevicePortalVersion = "20261114091126"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaTrustedDevicePortalVersion,
		Description: "Gemerkte Geräte gelten nur in ihrem Portal, im Träger-Portal nur beim Träger (#6955)",
		DependsOn: []string{
			catererTrustedDevicesVersion, // the devices without a school this one names a portal for (1.15.688)
			traegerPortalVersion,         // platform.organizations of the Träger-Portal (1.15.514)
		},
	})
	Migrations.MustRegister(mfaTrustedDevicePortalUp, mfaTrustedDevicePortalDown)
}

// mfaTrustedDevicePortalUpSQL binds a remembered device to the portal that
// remembered it (#6955). Until now a device named its account and, where it
// had one, its school; the tenant portal and moto schule of one school
// shared their devices, and a device without a school answered for every
// portal without one. The Träger-Portal now remembers devices too, so the
// row says where it counts:
//
//   - portal is the portal's name: tenant, school, caterer or org. A device
//     of the tenant portal or of moto schule names its school, one of the
//     caterer names none, one of the Träger-Portal names its Träger and no
//     school. The check keeps these shapes; every lookup compares the portal,
//     the school and the Träger.
//   - Existing devices with a school were remembered by the tenant portal or
//     by moto schule; the row does not say which. They become tenant-portal
//     devices: a Lehrkraft confirms the next sign-in to moto schule with a
//     code once more. Devices without a school are the caterer's.
//   - A Träger that is deleted takes its devices with it.
const mfaTrustedDevicePortalUpSQL = `
ALTER TABLE auth.mfa_trusted_devices
	ADD COLUMN portal TEXT,
	ADD COLUMN organization_id BIGINT REFERENCES platform.organizations(id) ON DELETE CASCADE;
UPDATE auth.mfa_trusted_devices SET portal = CASE WHEN tenant_id IS NULL THEN 'caterer' ELSE 'tenant' END;
ALTER TABLE auth.mfa_trusted_devices
	ALTER COLUMN portal SET NOT NULL,
	ADD CONSTRAINT chk_mfa_trusted_devices_portal CHECK (
		(portal IN ('tenant', 'school') AND tenant_id IS NOT NULL AND organization_id IS NULL)
		OR (portal = 'caterer' AND tenant_id IS NULL AND organization_id IS NULL)
		OR (portal = 'org' AND tenant_id IS NULL AND organization_id IS NOT NULL));
COMMENT ON COLUMN auth.mfa_trusted_devices.portal IS
	'Portal, das das Gerät gemerkt hat (tenant, school, caterer, org); das Gerät gilt nur dort (#6955).';
COMMENT ON COLUMN auth.mfa_trusted_devices.organization_id IS
	'Träger, bei dem das Träger-Portal das Gerät gemerkt hat; NULL in jedem anderen Portal (#6955).';
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS
	'Schule, an der das Gerät gemerkt ist; NULL im Caterer- und im Träger-Portal, deren zweiter Faktor an keiner Schule hängt (#4497, #6955).';
`

// mfaTrustedDevicePortalDownSQL forgets the Träger-Portal's devices, which
// the older code would take for the caterer's, and drops the binding. The
// devices of moto schule answer for the tenant portal of their school again,
// as before.
const mfaTrustedDevicePortalDownSQL = `
DELETE FROM auth.mfa_trusted_devices WHERE portal = 'org';
ALTER TABLE auth.mfa_trusted_devices
	DROP CONSTRAINT chk_mfa_trusted_devices_portal,
	DROP COLUMN organization_id,
	DROP COLUMN portal;
COMMENT ON COLUMN auth.mfa_trusted_devices.tenant_id IS
	'Schule, an der das Gerät gemerkt ist; NULL beim Caterer-Portal, dessen zweiter Faktor an keiner Schule hängt (#4497).';
`

func mfaTrustedDevicePortalUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, mfaTrustedDevicePortalUpSQL); err != nil {
			return fmt.Errorf("bind trusted devices to their portal: %w", err)
		}
		return nil
	})
}

func mfaTrustedDevicePortalDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, mfaTrustedDevicePortalDownSQL); err != nil {
			return fmt.Errorf("unbind trusted devices from their portal: %w", err)
		}
		return nil
	})
}
