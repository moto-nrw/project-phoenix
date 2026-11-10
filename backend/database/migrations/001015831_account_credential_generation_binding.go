package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const accountCredentialGenerationBindingVersion = "1.15.831"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     accountCredentialGenerationBindingVersion,
		Description: "Credential-Stand der Konten: Reset-Links und gemerkte Geräte daran gebunden, jede Credential-Änderung erhöht ihn (#6289)",
		DependsOn: []string{
			parentBillingSEPAMandateImportsVersion, // the last migration before this one
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
// The database enforces the rule for every writer, raw SQL and ORM alike:
// the trigger auth.account_credential_moves_generation refuses a change of
// password_hash or is_password_otp unless mfa_reset_generation rises in the
// same statement. The first password of an account without one keeps the
// generation (nothing can be bound to an earlier credential), and clearing
// the credential (the account erasures) moves it on by itself, so the
// existing erasure functions need no change. The generation never moves back.
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

CREATE OR REPLACE FUNCTION auth.account_credential_moves_generation() RETURNS trigger
LANGUAGE plpgsql AS $function$
BEGIN
	IF NEW.mfa_reset_generation < OLD.mfa_reset_generation THEN
		RAISE EXCEPTION 'the credential generation of account % never moves back', OLD.id
			USING ERRCODE = 'check_violation';
	END IF;
	IF (NEW.password_hash IS NOT DISTINCT FROM OLD.password_hash
			AND NEW.is_password_otp IS NOT DISTINCT FROM OLD.is_password_otp)
		OR NEW.mfa_reset_generation > OLD.mfa_reset_generation
		OR OLD.password_hash IS NULL THEN
		RETURN NEW;
	END IF;
	IF NEW.password_hash IS NULL THEN
		NEW.mfa_reset_generation := OLD.mfa_reset_generation + 1;
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'the credential of account % changed without moving its credential generation on', OLD.id
		USING ERRCODE = 'check_violation';
END
$function$;
COMMENT ON FUNCTION auth.account_credential_moves_generation() IS
	'Every change of an account credential moves the credential generation on (#6289): the first password of an account without one keeps it, a cleared credential moves it itself, any other change without a step is refused.';
DROP TRIGGER IF EXISTS accounts_credential_moves_generation ON auth.accounts;
CREATE TRIGGER accounts_credential_moves_generation
	BEFORE UPDATE OF password_hash, is_password_otp, mfa_reset_generation ON auth.accounts
	FOR EACH ROW EXECUTE FUNCTION auth.account_credential_moves_generation();
`

// accountCredentialGenerationBindingDownSQL ends for good what the binding
// had already ended before the column goes: a link or device of an older
// credential is spent or revoked, so a later Up, whose backfill takes the
// account's current generation, cannot make it current again.
//
// A second Down finds the columns gone and does nothing: each step runs only
// while its column still exists.
const accountCredentialGenerationBindingDownSQL = `
DO $down$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'auth' AND table_name = 'password_reset_tokens' AND column_name = 'credential_generation') THEN
		UPDATE auth.password_reset_tokens AS link SET used = TRUE
			FROM auth.accounts AS account
			WHERE account.id = link.account_id AND link.used = FALSE
				AND link.credential_generation <> account.mfa_reset_generation;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'auth' AND table_name = 'mfa_trusted_devices' AND column_name = 'credential_generation') THEN
		UPDATE auth.mfa_trusted_devices AS device SET revoked_at = statement_timestamp()
			FROM auth.accounts AS account
			WHERE account.id = device.account_id AND device.revoked_at IS NULL
				AND device.credential_generation <> account.mfa_reset_generation;
	END IF;
END
$down$;
DROP TRIGGER IF EXISTS accounts_credential_moves_generation ON auth.accounts;
DROP FUNCTION IF EXISTS auth.account_credential_moves_generation();
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
