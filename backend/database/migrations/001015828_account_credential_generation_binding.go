package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const accountCredentialGenerationBindingVersion = "1.15.828"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     accountCredentialGenerationBindingVersion,
		Description: "Reset-Links und gemerkte Geräte gelten nur für den Credential-Stand, unter dem sie entstanden (#6289)",
		DependsOn: []string{
			parentBillingSEPADebitsVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(accountCredentialGenerationBindingUp, accountCredentialGenerationBindingDown)
}

// accountCredentialGenerationBindingUpSQL binds the two credential artifacts
// that outlive a request to the account's credential generation
// (auth.accounts.mfa_reset_generation, #6289): a reset link and a remembered
// device record the generation they were issued under, and the reads that
// accept them require it to be the account's current one. Every credential
// write moves the generation on (one writer, guarded by
// TestAccountCredentialHasOneWriter), so the write ends both at every school
// at once, also from a school's own transaction, whose row security hides the
// devices of other schools.
//
// Existing rows take the account's current generation: they stay valid
// until the next credential write, as before. The application always writes
// the generation explicitly; the default 0 only serves inserts that name no
// generation (test fixtures, a backend from before this migration), and such
// a row is valid only for an account still at generation 0.
const accountCredentialGenerationBindingUpSQL = `
ALTER TABLE auth.password_reset_tokens ADD COLUMN IF NOT EXISTS credential_generation BIGINT;
UPDATE auth.password_reset_tokens AS link SET credential_generation = account.mfa_reset_generation
	FROM auth.accounts AS account WHERE account.id = link.account_id AND link.credential_generation IS NULL;
UPDATE auth.password_reset_tokens SET credential_generation = 0 WHERE credential_generation IS NULL;
ALTER TABLE auth.password_reset_tokens ALTER COLUMN credential_generation SET DEFAULT 0,
	ALTER COLUMN credential_generation SET NOT NULL;
COMMENT ON COLUMN auth.password_reset_tokens.credential_generation IS
	'auth.accounts.mfa_reset_generation when the link was issued; the link redeems only while it is the current one (#6289).';

ALTER TABLE auth.mfa_trusted_devices ADD COLUMN IF NOT EXISTS credential_generation BIGINT;
UPDATE auth.mfa_trusted_devices AS device SET credential_generation = account.mfa_reset_generation
	FROM auth.accounts AS account WHERE account.id = device.account_id AND device.credential_generation IS NULL;
UPDATE auth.mfa_trusted_devices SET credential_generation = 0 WHERE credential_generation IS NULL;
ALTER TABLE auth.mfa_trusted_devices ALTER COLUMN credential_generation SET DEFAULT 0,
	ALTER COLUMN credential_generation SET NOT NULL;
COMMENT ON COLUMN auth.mfa_trusted_devices.credential_generation IS
	'auth.accounts.mfa_reset_generation when the device was remembered; it skips the second factor only while it is the current one (#6289).';
`

const accountCredentialGenerationBindingDownSQL = `
ALTER TABLE auth.mfa_trusted_devices DROP COLUMN IF EXISTS credential_generation;
ALTER TABLE auth.password_reset_tokens DROP COLUMN IF EXISTS credential_generation;
`

func accountCredentialGenerationBindingUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, accountCredentialGenerationBindingUpSQL); err != nil {
		return fmt.Errorf("bind reset links and remembered devices to the credential generation: %w", err)
	}
	return nil
}

func accountCredentialGenerationBindingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, accountCredentialGenerationBindingDownSQL); err != nil {
		return fmt.Errorf("drop the credential generation of reset links and remembered devices: %w", err)
	}
	return nil
}
