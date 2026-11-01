package behavior_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// legacyRefreshToken re-signs a refresh token the way a backend before
// #5422 issued it: without family_id and lineage_id.
func legacyRefreshToken(t *testing.T, refresh string) string {
	t.Helper()
	claims := decodePreviewTokenPayload(t, refresh)
	auth, err := sessiontoken.NewTokenAuthWithDurations(authSigningSecret, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	legacy := sessiontoken.RefreshClaims{ID: int(claims["id"].(float64)), Token: claims["token"].(string)}
	if tenantID, ok := claims["tenant_id"].(float64); ok {
		legacy.TenantID = int64(tenantID)
	}
	legacy.ExpiresAt = int64(claims["exp"].(float64))
	token, err := auth.CreateRefreshJWT(legacy)
	require.NoError(t, err)
	_, hasFamily := decodePreviewTokenPayload(t, token)["family_id"]
	require.False(t, hasFamily, "the legacy token carries no lineage claims")
	return token
}

type familyHead struct {
	Expiry          time.Time  `bun:"expiry"`
	FamilyExpiryCap *time.Time `bun:"family_expiry_cap"`
}

func headOfFamily(t *testing.T, db *bun.DB, familyID string) familyHead {
	t.Helper()
	var head familyHead
	require.NoError(t, db.NewRaw(`SELECT expiry, family_expiry_cap FROM auth.tokens
		WHERE family_id = ? AND rotated_at IS NULL ORDER BY generation DESC LIMIT 1`, familyID).Scan(context.Background(), &head))
	return head
}

// ageFamily moves the family's live rows to a shorter remaining lifetime,
// as a family issued days before the deploy has.
func ageFamily(t *testing.T, db *bun.DB, familyID string, remaining time.Duration) time.Time {
	t.Helper()
	expiry := time.Now().Add(remaining)
	_, err := db.NewRaw(`UPDATE auth.tokens SET expiry = ? WHERE family_id = ? AND rotated_at IS NULL`, expiry, familyID).Exec(context.Background())
	require.NoError(t, err)
	return expiry
}

// TestLegacyFamilyThatMayDescendFromASwitchKeepsItsDeadline pins the
// transition of #5422: a family issued before the switch link existed may
// descend from a school switch without carrying the link, so no logout of
// its lineage reaches it. Its next rotation must not slide its lifetime:
// the successor keeps the family's deadline, and so does every later one,
// so the family ends one refresh lifetime after the deploy at the latest.
func TestLegacyFamilyThatMayDescendFromASwitchKeepsItsDeadline(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "legacy-lineage-cap")
	ctx := testpkg.Ctx(t)
	family := accessSessionOf(t, f.access).FamilyID
	deadline := ageFamily(t, f.db, family, time.Hour)

	_, rotated, err := f.service.RefreshToken(ctx, legacyRefreshToken(t, f.refresh))
	require.NoError(t, err)
	head := headOfFamily(t, f.db, family)
	assert.False(t, head.Expiry.After(deadline), "the rotation of a legacy family of a multi-school account does not extend it")
	require.NotNil(t, head.FamilyExpiryCap, "the deadline is kept on the family")
	assert.WithinDuration(t, deadline, *head.FamilyExpiryCap, time.Second)

	_, _, err = f.service.RefreshToken(ctx, rotated)
	require.NoError(t, err)
	assert.False(t, headOfFamily(t, f.db, family).Expiry.After(deadline), "later rotations keep the deadline too")
}

// TestLegacyFamilyOfASingleSchoolAccountKeepsSliding keeps the transition
// narrow: an account mapped to one school only and without Träger
// membership never switched, so its sessions renew as before and no user
// is signed out by the deploy.
func TestLegacyFamilyOfASingleSchoolAccountKeepsSliding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("legacy-lineage-single")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	access, refresh, err := service.IssueTokensForAuthenticatedAccount(ctx, account.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	family := accessSessionOf(t, access).FamilyID
	deadline := ageFamily(t, db, family, time.Hour)

	_, _, err = service.RefreshToken(ctx, legacyRefreshToken(t, refresh))
	require.NoError(t, err)
	head := headOfFamily(t, db, family)
	assert.True(t, head.Expiry.After(deadline.Add(time.Hour)), "a single-school family renews its full lifetime")
	assert.Nil(t, head.FamilyExpiryCap)
}
