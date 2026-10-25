package migrations

// Coverage for 1.15.521 (#4902): a raw token stored before the migration is
// replaced by its hex SHA-256, so the presented value still finds its row;
// NULL hand-offs stay NULL, a rerun does not hash twice, and afterwards
// every token column refuses a raw value.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashStoredTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	// SHA-256 of the raw values below, computed independently.
	sha := map[string]string{
		"0f6e7f0e-3b9e-4c55-9d43-6c2b1f0a8e11": "83ec3fcec941b97493c827b52149086175e3b7f470c8e6ee8ad3093c8118bfe0",
		"3c1d2b4a-6f7e-4d8c-9b0a-1e2f3a4b5c6d": "2cc3ab47d693d1efa5fd9c661a36ee7250fe035ade982abeffbe7cc436150230",
		"7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d": "b79f93cc41c13ed4b9d3b4c086c877f13ab21327439255ecb96c5538f4c3970c",
		"9d8c7b6a-5f4e-4d3c-8b2a-1f0e9d8c7b6a": "60abbf836a2cff794a3d517ef3ea218516c0537667701de143526fe8b9fd7bda",
	}

	// The state before the migration: raw values, no constraints.
	require.NoError(t, hashStoredTokensDown(ctx, db))
	account := testpkg.CreateTestAccount(t, db, "hash-stored-tokens")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	const live, rotated, successor, reset = "0f6e7f0e-3b9e-4c55-9d43-6c2b1f0a8e11", "3c1d2b4a-6f7e-4d8c-9b0a-1e2f3a4b5c6d", "7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d", "9d8c7b6a-5f4e-4d3c-8b2a-1f0e9d8c7b6a"
	var rotatedID, liveID, resetID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.tokens (account_id, tenant_id, token, expiry, family_id, generation)
		VALUES (?, ?, ?, ?, 'family', 1) RETURNING id`, account.ID, tenantID, live, time.Now().Add(time.Hour)).Scan(ctx, &liveID))
	require.NoError(t, db.NewRaw(`INSERT INTO auth.tokens (account_id, tenant_id, token, expiry, family_id, generation, rotated_at, replacement_token)
		VALUES (?, ?, ?, ?, 'family', 0, NOW(), ?) RETURNING id`, account.ID, tenantID, rotated, time.Now().Add(time.Hour), successor).Scan(ctx, &rotatedID))
	require.NoError(t, db.NewRaw(`INSERT INTO auth.password_reset_tokens (account_id, token, expiry)
		VALUES (?, ?, ?) RETURNING id`, account.ID, reset, time.Now().Add(time.Hour)).Scan(ctx, &resetID))

	column := func(query string, id int64) *string {
		t.Helper()
		var value *string
		require.NoError(t, db.NewRaw(query, id).Scan(ctx, &value))
		return value
	}
	assertHashed := func() {
		t.Helper()
		assert.Equal(t, sha[live], *column(`SELECT token FROM auth.tokens WHERE id = ?`, liveID))
		assert.Nil(t, column(`SELECT replacement_token FROM auth.tokens WHERE id = ?`, liveID), "a session without hand-off keeps NULL")
		assert.Equal(t, sha[rotated], *column(`SELECT token FROM auth.tokens WHERE id = ?`, rotatedID))
		assert.Equal(t, sha[successor], *column(`SELECT replacement_token FROM auth.tokens WHERE id = ?`, rotatedID))
		assert.Equal(t, sha[reset], *column(`SELECT token FROM auth.password_reset_tokens WHERE id = ?`, resetID))
	}

	require.NoError(t, hashStoredTokensUp(ctx, db))
	assertHashed()
	require.NoError(t, hashStoredTokensUp(ctx, db), "a rerun leaves the digests alone")
	assertHashed()

	for _, c := range []struct{ table, column, constraint string }{
		{"auth.tokens", "token", "chk_tokens_token_sha256"},
		{"auth.tokens", "replacement_token", "chk_tokens_replacement_token_sha256"},
		{"auth.password_reset_tokens", "token", "chk_password_reset_tokens_token_sha256"},
		{"auth.invitation_tokens", "token", "chk_invitation_tokens_token_sha256"},
		{"auth.guardian_invitations", "token", "chk_guardian_invitations_token_sha256"},
		{"platform.operator_refresh_tokens", "token", "chk_operator_refresh_tokens_token_sha256"},
		{"platform.operator_refresh_tokens", "replacement_token", "chk_operator_refresh_tokens_replacement_token_sha256"},
		{"platform.operator_invitation_tokens", "token", "chk_operator_invitation_tokens_token_sha256"},
		{"platform.operator_email_change_tokens", "token", "chk_operator_email_change_tokens_token_sha256"},
	} {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conname = ? AND conrelid = ?::regclass`, c.constraint, c.table).Scan(ctx, &definition),
			"%s.%s is constrained", c.table, c.column)
		assert.Contains(t, definition, c.column)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO auth.password_reset_tokens (account_id, token, expiry) VALUES (?, ?, ?)`,
		account.ID, "0f6e7f0e-raw-link", time.Now().Add(time.Hour))
	require.Error(t, err, "a raw token is refused")
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET replacement_token = ? WHERE id = ?`, successor, rotatedID)
	require.Error(t, err, "a raw hand-off is refused")

	require.NoError(t, hashStoredTokensDown(ctx, db))
	var constraints int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint WHERE conname LIKE 'chk\_%\_sha256'`).Scan(ctx, &constraints))
	assert.Zero(t, constraints, "Down drops the constraints")
	var comment string
	require.NoError(t, db.NewRaw(`SELECT col_description('auth.tokens'::regclass, attnum) FROM pg_attribute
		WHERE attrelid = 'auth.tokens'::regclass AND attname = 'replacement_token'`).Scan(ctx, &comment))
	assert.Equal(t, "Opaque successor handle retained until refresh expiry for bounded recovery and replay-family attribution; never returned by APIs or logs.",
		comment, "Down restores the previous comment")
	require.NoError(t, hashStoredTokensUp(ctx, db))
	assertHashed()
}
