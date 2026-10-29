package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accessTokenFamily(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		FamilyID string `json:"family_id"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims.FamilyID
}

// TestBackfilledLegacyChainStaysSignedInAndRevocable pins migration 1.15.707
// with the session check of #5194: a refresh chain from before refresh
// families, head and rotated predecessor alike, gets one family, keeps
// signing in through its next refresh, and its logout then revokes the
// whole chain.
func TestBackfilledLegacyChainStaysSignedInAndRevocable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	identity, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "backfill-legacy-chain")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	// A chain with a rotated predecessor in its hand-off grace, then made
	// what a chain from before 1.4.7 is: no family on any of its rows.
	_, first, err := identity.IssueTokensForAuthenticatedAccount(ctx, account.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	_, head, err := identity.RefreshTokenWithAudit(ctx, first, "", "", "")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET family_id = NULL WHERE account_id = ?`, account.ID)
	require.NoError(t, err)

	require.NoError(t, migrations.BackfillSessionFamilies(context.Background(), db))

	var families []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT family_id FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &families))
	require.Len(t, families, 1, "head and predecessor share one family")
	require.NotEmpty(t, families[0])

	// The person stays signed in: the next refresh succeeds and its access
	// token rests on the backfilled family.
	access, renewed, err := identity.RefreshTokenWithAudit(ctx, head, "", "", "")
	require.NoError(t, err)
	session := identityaccess.AccessSession{AccountID: account.ID, FamilyID: accessTokenFamily(t, access)}
	assert.Equal(t, families[0], session.FamilyID)
	require.NoError(t, identity.VerifyAccessSession(ctx, session))

	// Logout revokes the whole chain.
	require.NoError(t, identity.LogoutWithAudit(ctx, renewed, "", ""))
	require.ErrorIs(t, identity.VerifyAccessSession(ctx, session), identityaccess.ErrInvalidToken)
	remaining, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, remaining, "no row of the chain survives its logout")
}
