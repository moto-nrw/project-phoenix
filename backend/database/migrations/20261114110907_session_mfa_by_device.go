package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const sessionMFAByDeviceVersion = "20261114110907"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     sessionMFAByDeviceVersion,
		Description: "Sitzungen halten fest, ob nur ein gemerktes Gerät den zweiten Faktor bewiesen hat (#6955)",
		DependsOn: []string{
			"1.15.484",                   // auth.tokens.mfa_verified, beside which the mark stands
			traegerTrustedDevicesVersion, // the Träger's remembered devices the backfill reads
		},
	})
	Migrations.MustRegister(sessionMFAByDeviceUp, sessionMFAByDeviceDown)
}

// sessionMFAByDeviceUpSQL adds auth.tokens.mfa_by_device (#6955): true when
// only a remembered device proved the session's second factor. An Absprung
// from the Träger-Portal and a switch between schools carry the mark; a
// school that switched remembered devices off accepts no such session and
// asks for the code (ADR 0084, addendum on the Absprung).
const sessionMFAByDeviceUpSQL = `
ALTER TABLE auth.tokens ADD COLUMN mfa_by_device BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN auth.tokens.mfa_by_device IS
	'Der zweite Faktor dieser Sitzung ist nur durch ein gemerktes Gerät nachgewiesen; eine Schule mit ausgeschaltetem Gerät-Merken verlangt dann den Code (#6955).';
`

// sessionMFAByDeviceBackfillSQL marks the live sessions that a Träger device
// may have proven before the column existed. The row does not say how its
// factor was proven, so every verified session at a Träger whose account
// remembers a device there counts as device-proven: the error costs a
// sign-in with a code at a school without remembered devices, never an
// Absprung the school refused. School sessions proven by a device of their
// own school passed that school's setting at sign-in and stay unmarked.
const sessionMFAByDeviceBackfillSQL = `
UPDATE auth.tokens t SET mfa_by_device = TRUE
WHERE t.organization_id IS NOT NULL AND t.mfa_verified
  AND EXISTS (SELECT 1 FROM auth.mfa_trusted_devices d
    WHERE d.account_id = t.account_id AND d.organization_id = t.organization_id);
`

const sessionMFAByDeviceDownSQL = `
ALTER TABLE auth.tokens DROP COLUMN mfa_by_device;
`

func sessionMFAByDeviceUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, sessionMFAByDeviceUpSQL); err != nil {
			return fmt.Errorf("add the session's device mark: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sessionMFAByDeviceBackfillSQL); err != nil {
			return fmt.Errorf("mark sessions a Träger device may have proven: %w", err)
		}
		return nil
	})
}

func sessionMFAByDeviceDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, sessionMFAByDeviceDownSQL); err != nil {
			return fmt.Errorf("drop the session's device mark: %w", err)
		}
		return nil
	})
}
