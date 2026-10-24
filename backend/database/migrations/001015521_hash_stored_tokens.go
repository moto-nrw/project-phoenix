package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	hashStoredTokensVersion     = "1.15.521"
	hashStoredTokensDescription = "Store session, invitation, password-reset and e-mail-change tokens only as SHA-256 (#4902)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     hashStoredTokensVersion,
		Description: hashStoredTokensDescription,
		DependsOn: []string{
			AuthTokensVersion,
			AuthPasswordResetTokensVersion,
			AuthInvitationTokensVersion,
			AuthGuardianInvitationsVersion,
			platformOperatorEmailChangeTokensVersion,
			platformOperatorInvitationTokensVersion,
			operatorRefreshTokensVersion,
			refreshTokenRotationRecoveryVersion,
		},
	})
	Migrations.MustRegister(hashStoredTokensUp, hashStoredTokensDown)
}

// hashStoredTokensUpSQL replaces every stored token by its hex SHA-256, the
// value Identity & Access writes and compares (domain.HashToken). A value
// already in that shape is left alone, so a rerun does not hash twice; no
// generated raw token has it, they are UUIDs. The CHECK constraints refuse
// any later write of a raw value.
const hashStoredTokensUpSQL = `
UPDATE auth.tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE auth.tokens SET replacement_token = encode(sha256(convert_to(replacement_token, 'UTF8')), 'hex')
	WHERE replacement_token IS NOT NULL AND replacement_token !~ '^[0-9a-f]{64}$';
UPDATE auth.password_reset_tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE auth.invitation_tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE auth.guardian_invitations SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE platform.operator_refresh_tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE platform.operator_refresh_tokens SET replacement_token = encode(sha256(convert_to(replacement_token, 'UTF8')), 'hex')
	WHERE replacement_token IS NOT NULL AND replacement_token !~ '^[0-9a-f]{64}$';
UPDATE platform.operator_invitation_tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';
UPDATE platform.operator_email_change_tokens SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex')
	WHERE token !~ '^[0-9a-f]{64}$';

ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_token_sha256;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_replacement_token_sha256;
ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_replacement_token_sha256
	CHECK (replacement_token IS NULL OR replacement_token ~ '^[0-9a-f]{64}$');
ALTER TABLE auth.password_reset_tokens DROP CONSTRAINT IF EXISTS chk_password_reset_tokens_token_sha256;
ALTER TABLE auth.password_reset_tokens ADD CONSTRAINT chk_password_reset_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE auth.invitation_tokens DROP CONSTRAINT IF EXISTS chk_invitation_tokens_token_sha256;
ALTER TABLE auth.invitation_tokens ADD CONSTRAINT chk_invitation_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE auth.guardian_invitations DROP CONSTRAINT IF EXISTS chk_guardian_invitations_token_sha256;
ALTER TABLE auth.guardian_invitations ADD CONSTRAINT chk_guardian_invitations_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE platform.operator_refresh_tokens DROP CONSTRAINT IF EXISTS chk_operator_refresh_tokens_token_sha256;
ALTER TABLE platform.operator_refresh_tokens ADD CONSTRAINT chk_operator_refresh_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE platform.operator_refresh_tokens DROP CONSTRAINT IF EXISTS chk_operator_refresh_tokens_replacement_token_sha256;
ALTER TABLE platform.operator_refresh_tokens ADD CONSTRAINT chk_operator_refresh_tokens_replacement_token_sha256
	CHECK (replacement_token IS NULL OR replacement_token ~ '^[0-9a-f]{64}$');
ALTER TABLE platform.operator_invitation_tokens DROP CONSTRAINT IF EXISTS chk_operator_invitation_tokens_token_sha256;
ALTER TABLE platform.operator_invitation_tokens ADD CONSTRAINT chk_operator_invitation_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');
ALTER TABLE platform.operator_email_change_tokens DROP CONSTRAINT IF EXISTS chk_operator_email_change_tokens_token_sha256;
ALTER TABLE platform.operator_email_change_tokens ADD CONSTRAINT chk_operator_email_change_tokens_token_sha256
	CHECK (token ~ '^[0-9a-f]{64}$');

COMMENT ON COLUMN auth.tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN auth.tokens.replacement_token IS
	'Hex SHA-256 of the successor handle (#4902), retained until refresh expiry for bounded recovery and replay-family attribution; the raw value is never stored.';
COMMENT ON COLUMN auth.password_reset_tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN auth.invitation_tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN auth.guardian_invitations.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN platform.operator_refresh_tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN platform.operator_refresh_tokens.replacement_token IS
	'Hex SHA-256 of the successor handle (#4902), retained until refresh expiry for bounded recovery and replay-family attribution; the raw value is never stored.';
COMMENT ON COLUMN platform.operator_invitation_tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
COMMENT ON COLUMN platform.operator_email_change_tokens.token IS 'Hex SHA-256 of the token (#4902); the raw value is never stored.';
`

// hashStoredTokensDownSQL drops the constraints and restores the column
// comments of 1.15.204. The digests cannot be reversed.
const hashStoredTokensDownSQL = `
ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_token_sha256;
ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_replacement_token_sha256;
ALTER TABLE auth.password_reset_tokens DROP CONSTRAINT IF EXISTS chk_password_reset_tokens_token_sha256;
ALTER TABLE auth.invitation_tokens DROP CONSTRAINT IF EXISTS chk_invitation_tokens_token_sha256;
ALTER TABLE auth.guardian_invitations DROP CONSTRAINT IF EXISTS chk_guardian_invitations_token_sha256;
ALTER TABLE platform.operator_refresh_tokens DROP CONSTRAINT IF EXISTS chk_operator_refresh_tokens_token_sha256;
ALTER TABLE platform.operator_refresh_tokens DROP CONSTRAINT IF EXISTS chk_operator_refresh_tokens_replacement_token_sha256;
ALTER TABLE platform.operator_invitation_tokens DROP CONSTRAINT IF EXISTS chk_operator_invitation_tokens_token_sha256;
ALTER TABLE platform.operator_email_change_tokens DROP CONSTRAINT IF EXISTS chk_operator_email_change_tokens_token_sha256;

COMMENT ON COLUMN auth.tokens.token IS NULL;
COMMENT ON COLUMN auth.tokens.replacement_token IS
	'Opaque successor handle retained until refresh expiry for bounded recovery and replay-family attribution; never returned by APIs or logs.';
COMMENT ON COLUMN auth.password_reset_tokens.token IS NULL;
COMMENT ON COLUMN auth.invitation_tokens.token IS NULL;
COMMENT ON COLUMN auth.guardian_invitations.token IS NULL;
COMMENT ON COLUMN platform.operator_refresh_tokens.token IS NULL;
COMMENT ON COLUMN platform.operator_refresh_tokens.replacement_token IS
	'Opaque successor handle retained until refresh expiry for bounded recovery and replay-family attribution; never returned by APIs or logs.';
COMMENT ON COLUMN platform.operator_invitation_tokens.token IS NULL;
COMMENT ON COLUMN platform.operator_email_change_tokens.token IS NULL;
`

// hashStoredTokensUp replaces every stored token by its hex SHA-256 (#4902).
// A reader of the database or a backup can then neither take over a session
// nor redeem a mailed link, as the AVV (Anhang 1 Ziff. 2) promises. Lookups
// hash the presented value, so live sessions and mailed links keep working.
// A rotation hand-off recorded before this migration named a random
// successor; it cannot be recovered and is treated as a replay, so the
// affected client signs in again.
func hashStoredTokensUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, hashStoredTokensUpSQL); err != nil {
			return fmt.Errorf("hash stored tokens: %w", err)
		}
		return nil
	})
}

// hashStoredTokensDown drops the constraints. After a rollback the previous
// code finds none of the stored sessions and links, so every user signs in
// again and open invitations and reset links have to be sent anew.
func hashStoredTokensDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, hashStoredTokensDownSQL); err != nil {
		return fmt.Errorf("drop stored token hash constraints: %w", err)
	}
	return nil
}
