package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSessionMFAProofUpgradeRequiresFreshVerification(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()

	require.NoError(t, sessionMFAProofDown(ctx, db))
	account := testpkg.CreateTestAccount(t, db, "mfa-migration@example.test")
	token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
	require.NoError(t, sessionMFAProofUp(ctx, db))

	var verified bool
	require.NoError(t, db.NewRaw(`SELECT mfa_verified FROM auth.tokens WHERE id = ?`, token.ID).Scan(ctx, &verified))
	require.False(t, verified, "existing sessions must not gain proof of a second factor")
	_, err := db.ExecContext(ctx, `UPDATE auth.tokens SET mfa_verified = NULL WHERE id = ?`, token.ID)
	require.ErrorContains(t, err, "not-null constraint")
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET mfa_verified = TRUE WHERE id = ?`, token.ID)
	require.NoError(t, err, "verified sessions can retain their proof after the upgrade")
}
