package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

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

// TestBackfillAndConcurrentRotationKeepOneFamily pins migration 1.15.707
// against a legacy session that rotates while it runs (#5194), with and
// without a rotated predecessor. A lock on the head stops the rotation
// after it has taken the account lock; the migration then starts and waits
// for the same account lock. Once the rows are released the
// rotation commits, and only then does the migration read the chain. The
// whole chain keeps one family, the person stays signed in, and a hand-off
// recovery of the rotated session still works.
func TestBackfillAndConcurrentRotationKeepOneFamily(t *testing.T) {
	t.Parallel()
	backfillDuringRotation(t, false)
}

// TestBackfillAndConcurrentRotationWithPredecessorKeepsOneFamily is the same
// race for a chain that already has a rotated predecessor (#5194).
func TestBackfillAndConcurrentRotationWithPredecessorKeepsOneFamily(t *testing.T) {
	t.Parallel()
	backfillDuringRotation(t, true)
}

// backfillDuringRotation needs a database of its own: it counts the blocked
// backends of its database.
func backfillDuringRotation(t *testing.T, withPredecessor bool) {
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	identity, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "backfill-concurrent-rotation")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, head, err := identity.IssueTokensForAuthenticatedAccount(ctx, account.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	if withPredecessor {
		_, head, err = identity.RefreshTokenWithAudit(ctx, head, "", "", "")
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET family_id = NULL WHERE account_id = ?`, account.ID)
	require.NoError(t, err)

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID))
	_, err = holder.ExecContext(ctx, `SELECT id FROM auth.tokens WHERE account_id = ? AND rotated_at IS NULL FOR UPDATE`, account.ID)
	require.NoError(t, err)
	waiting := func(n int) func() bool {
		return func() bool {
			var blocked int
			// The migration may queue behind the rotation instead of the holder.
			err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity
				WHERE datname = current_database() AND pid <> ? AND cardinality(pg_blocking_pids(pid)) > 0`, holderPID).Scan(&blocked)
			return err == nil && blocked >= n
		}
	}

	type refreshed struct {
		access string
		err    error
	}
	const recoveryProof = "independent-recovery-secret"
	rotation := make(chan refreshed, 1)
	go func() {
		access, _, err := identity.RefreshTokenWithAudit(context.WithoutCancel(ctx), head, recoveryProof, "", "")
		rotation <- refreshed{access, err}
	}()
	require.Eventually(t, waiting(1), 5*time.Second, 10*time.Millisecond, "the rotation waits for the head")
	migration := make(chan error, 1)
	go func() { migration <- migrations.BackfillSessionFamilies(context.Background(), db) }()
	require.Eventually(t, waiting(2), 5*time.Second, 10*time.Millisecond, "the migration waits too")
	require.NoError(t, holder.Commit())

	result := <-rotation
	require.NoError(t, result.err, "the rotating legacy session stays signed in")
	require.NoError(t, <-migration)

	family := accessTokenFamily(t, result.access)
	require.NotEmpty(t, family)
	require.NoError(t, identity.VerifyAccessSession(ctx, identityaccess.AccessSession{AccountID: account.ID, FamilyID: family}))
	var families []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT COALESCE(family_id, '') FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &families))
	assert.Equal(t, []string{family}, families, "no row stays without family, and the chain keeps one")

	// The response of the rotation got lost: the client presents the rotated
	// session again and the hand-off recovery answers within the same family.
	recovered, _, err := identity.RefreshTokenWithAudit(ctx, head, recoveryProof, "", "")
	require.NoError(t, err, "the hand-off recovery of the rotated session works")
	assert.Equal(t, family, accessTokenFamily(t, recovered))
}
