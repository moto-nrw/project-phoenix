package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const parentTrustedDevicesVersion = "20261114180000"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentTrustedDevicesVersion,
		Description: "Gemerkte Geräte des Eltern-Portals (#7051)",
		DependsOn: []string{
			traegerTrustedDevicesVersion, // the one-portal CHECK and the Träger binding (#6955)
		},
	})
	Migrations.MustRegister(parentTrustedDevicesUp, parentTrustedDevicesDown)
}

// parentTrustedDevicesUpSQL lets the parents portal remember a device
// (#7051). A parent session is bound to the account, not to a school or a
// Träger (ADR 0216), so its device names neither, like the caterer portal's
// (#4497). parent_portal tells the two apart: a device remembered at the
// caterer portal never answers at the parents portal, nor the other way
// round. The CHECK keeps a parents-portal device free of a school and a
// Träger. The device goes with its account (account_id ON DELETE CASCADE
// and the erasure functions, which delete by account).
const parentTrustedDevicesUpSQL = `
ALTER TABLE auth.mfa_trusted_devices
	ADD COLUMN parent_portal BOOLEAN NOT NULL DEFAULT false,
	ADD CONSTRAINT chk_mfa_trusted_devices_parent_portal CHECK (NOT parent_portal OR (tenant_id IS NULL AND organization_id IS NULL));
COMMENT ON COLUMN auth.mfa_trusted_devices.parent_portal IS
	'Gerät, das im Eltern-Portal gemerkt ist; ohne Schule und ohne Träger, wie beim Caterer-Portal (#7051).';
`

// parentTrustedDevicesDownSQL forgets the parents portal's remembered
// devices: a row without the column would read as the caterer portal's.
// Parents then confirm their next sign-in with a code once more.
const parentTrustedDevicesDownSQL = `
DELETE FROM auth.mfa_trusted_devices WHERE parent_portal;
ALTER TABLE auth.mfa_trusted_devices DROP COLUMN parent_portal;
`

func parentTrustedDevicesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentTrustedDevicesUpSQL); err != nil {
			return fmt.Errorf("bind trusted devices to the parents portal: %w", err)
		}
		return nil
	})
}

func parentTrustedDevicesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentTrustedDevicesDownSQL); err != nil {
			return fmt.Errorf("unbind trusted devices from the parents portal: %w", err)
		}
		return nil
	})
}
