package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const operatorCredentialGenerationBindingVersion = "1.15.882"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operatorCredentialGenerationBindingVersion,
		Description: "Credential-Stand der Operatoren: MFA-Codes und gemerkte Geräte daran gebunden, jeder Passwortwechsel erhöht ihn (#6308)",
		DependsOn: []string{
			demoRetirementRevokesSessionsVersion,      // the last migration before this one
			accountCredentialGenerationBindingVersion, // the account binding this one mirrors
		},
	})
	Migrations.MustRegister(operatorCredentialGenerationBindingUp, operatorCredentialGenerationBindingDown)
}

// operatorCredentialGenerationBindingUpSQL gives every operator the credential
// generation the accounts got in 1.15.831 (#6308): platform.operators
// .credential_generation names the state of the operator's password, and the
// artifacts a password check leads to (the e-mail code, the enrollment and
// challenge tokens, the remembered device) record the generation they were
// issued under. The reads that accept them require it to be the operator's
// current one, so a password write ends them all in the same statement.
//
// The database enforces the rule for every writer: the trigger
// platform.operator_credential_moves_generation refuses a change of
// password_hash unless credential_generation rises in the same statement,
// and the generation never moves back.
//
// Generation 0 is reserved for what was issued before the binding: every
// operator starts at 1 and the CHECK keeps it there, while the existing
// codes and devices and the tokens without the claim stand at 0. They are
// refused from the moment this migration runs; an operator in the middle of
// a sign-in starts again, and a remembered device asks for one code.
const operatorCredentialGenerationBindingUpSQL = `
ALTER TABLE platform.operators ADD COLUMN IF NOT EXISTS credential_generation BIGINT NOT NULL DEFAULT 1
	CONSTRAINT operators_credential_generation_positive CHECK (credential_generation > 0);
COMMENT ON COLUMN platform.operators.credential_generation IS
	'State of the operator password; every password write moves it on, and the MFA codes and remembered devices of a password check are bound to it (#6308). 0 marks artifacts from before the binding.';

ALTER TABLE platform.operator_mfa_email_challenges ADD COLUMN IF NOT EXISTS credential_generation BIGINT NOT NULL DEFAULT 0;
COMMENT ON COLUMN platform.operator_mfa_email_challenges.credential_generation IS
	'platform.operators.credential_generation when the code was issued; the code redeems only while it is the current one (#6308).';

ALTER TABLE platform.operator_mfa_trusted_devices ADD COLUMN IF NOT EXISTS credential_generation BIGINT NOT NULL DEFAULT 0;
COMMENT ON COLUMN platform.operator_mfa_trusted_devices.credential_generation IS
	'platform.operators.credential_generation when the device was remembered; it skips the second factor only while it is the current one (#6308).';

CREATE OR REPLACE FUNCTION platform.operator_credential_moves_generation() RETURNS trigger
LANGUAGE plpgsql AS $function$
BEGIN
	IF NEW.credential_generation < OLD.credential_generation THEN
		RAISE EXCEPTION 'the credential generation of operator % never moves back', OLD.id
			USING ERRCODE = 'check_violation';
	END IF;
	IF NEW.password_hash IS NOT DISTINCT FROM OLD.password_hash
		OR NEW.credential_generation > OLD.credential_generation THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'the credential of operator % changed without moving its credential generation on', OLD.id
		USING ERRCODE = 'check_violation';
END
$function$;
COMMENT ON FUNCTION platform.operator_credential_moves_generation() IS
	'Every change of an operator password moves the credential generation on (#6308); a change without a step is refused.';
DROP TRIGGER IF EXISTS operators_credential_moves_generation ON platform.operators;
CREATE TRIGGER operators_credential_moves_generation
	BEFORE UPDATE OF password_hash, credential_generation ON platform.operators
	FOR EACH ROW EXECUTE FUNCTION platform.operator_credential_moves_generation();
`

// operatorCredentialGenerationBindingDownSQL ends for good what the binding
// had already ended before the columns go: the code paths before it accept
// any active code and any unrevoked device, so a code or device of an older
// credential is spent or revoked first. Nothing else changes; a later Up
// puts the remaining artifacts at generation 0 again, which refuses them.
//
// A second Down finds the columns gone and does nothing: each step runs only
// while its column still exists.
const operatorCredentialGenerationBindingDownSQL = `
DO $down$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'platform' AND table_name = 'operators' AND column_name = 'credential_generation') THEN
		IF EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'platform' AND table_name = 'operator_mfa_email_challenges' AND column_name = 'credential_generation') THEN
			UPDATE platform.operator_mfa_email_challenges AS challenge SET consumed_at = statement_timestamp()
				FROM platform.operators AS operator
				WHERE operator.id = challenge.operator_id AND challenge.consumed_at IS NULL
					AND challenge.credential_generation <> operator.credential_generation;
		END IF;
		IF EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'platform' AND table_name = 'operator_mfa_trusted_devices' AND column_name = 'credential_generation') THEN
			UPDATE platform.operator_mfa_trusted_devices AS device SET revoked_at = statement_timestamp()
				FROM platform.operators AS operator
				WHERE operator.id = device.operator_id AND device.revoked_at IS NULL
					AND device.credential_generation <> operator.credential_generation;
		END IF;
	END IF;
END
$down$;
DROP TRIGGER IF EXISTS operators_credential_moves_generation ON platform.operators;
DROP FUNCTION IF EXISTS platform.operator_credential_moves_generation();
ALTER TABLE platform.operator_mfa_trusted_devices DROP COLUMN IF EXISTS credential_generation;
ALTER TABLE platform.operator_mfa_email_challenges DROP COLUMN IF EXISTS credential_generation;
ALTER TABLE platform.operators DROP COLUMN IF EXISTS credential_generation;
`

func operatorCredentialGenerationBindingUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, operatorCredentialGenerationBindingUpSQL); err != nil {
		return fmt.Errorf("bind operator codes and remembered devices to the credential generation: %w", err)
	}
	return nil
}

func operatorCredentialGenerationBindingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, operatorCredentialGenerationBindingDownSQL); err != nil {
		return fmt.Errorf("drop the operator credential generation: %w", err)
	}
	return nil
}
