package behavior_test

import (
	"context"
	"fmt"
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

// ageFamily turns the family into one issued before #5422, days before the
// deploy: its rows name no lineage origin, and its live rows have a shorter
// remaining lifetime.
func ageFamily(t *testing.T, db *bun.DB, familyID string, remaining time.Duration) time.Time {
	t.Helper()
	expiry := time.Now().Add(remaining)
	_, err := db.NewRaw(`UPDATE auth.tokens SET switched_from_family_id = NULL WHERE family_id = ?`, familyID).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.tokens SET expiry = ? WHERE family_id = ? AND rotated_at IS NULL`, expiry, familyID).Exec(context.Background())
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

// A rotation committed before deployment can be recovered with its legacy
// predecessor. Recovery must persist the deadline before adding lineage claims.
func TestLegacyRefreshRecoveryKeepsItsDeadline(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "legacy-recovery-cap")
	ctx := testpkg.Ctx(t)
	family := accessSessionOf(t, f.access).FamilyID
	legacy := legacyRefreshToken(t, f.refresh)
	const proof = "legacy-recovery-proof"
	_, _, err := f.service.RefreshTokenWithAudit(ctx, f.refresh, proof, "", "")
	require.NoError(t, err)
	deadline := ageFamily(t, f.db, family, time.Hour)

	service := setupAuthService(t, f.db)
	_, recovered, err := service.RefreshTokenWithAudit(ctx, legacy, proof, "", "")
	require.NoError(t, err)
	head := headOfFamily(t, f.db, family)
	require.NotNil(t, head.FamilyExpiryCap, "recovery persists the legacy deadline")
	assert.WithinDuration(t, deadline, *head.FamilyExpiryCap, time.Second)
	assert.False(t, head.Expiry.After(deadline))

	_, _, err = service.RefreshToken(ctx, recovered)
	require.NoError(t, err)
	assert.False(t, headOfFamily(t, f.db, family).Expiry.After(deadline), "claims issued by recovery cannot bypass the deadline")
}

// A legacy switch descendant is unlinked from its original login. Switching
// it again must not let it escape the deadline imposed at its first refresh.
func TestLegacyFamilySwitchKeepsItsDeadline(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "legacy-switch-cap")
	ctx := testpkg.Ctx(t)
	family := accessSessionOf(t, f.access).FamilyID
	deadline := ageFamily(t, f.db, family, time.Hour)
	_, _, err := f.service.RefreshToken(ctx, legacyRefreshToken(t, f.refresh))
	require.NoError(t, err)

	access, refresh, err := f.switchToTarget(ctx, family)
	require.NoError(t, err)
	switchedFamily := accessSessionOf(t, access).FamilyID
	head := headOfFamily(t, f.db, switchedFamily)
	require.NotNil(t, head.FamilyExpiryCap, "a school switch inherits the legacy deadline")
	assert.WithinDuration(t, deadline, *head.FamilyExpiryCap, time.Second)
	assert.False(t, head.Expiry.After(deadline), "the new family cannot outlive its legacy source")

	_, _, err = f.service.RefreshToken(ctx, refresh)
	require.NoError(t, err)
	assert.False(t, headOfFamily(t, f.db, switchedFamily).Expiry.After(deadline), "rotation of the switched family keeps the deadline")
}

// TestSwitchFromARetiredFamilyKeepsAFullLifetime keeps the transition to
// families issued before #5422: a second tab that switches out of a family a
// first tab's switch just retired gets a family of the full lifetime, not the
// five-minute grace of its retired source.
func TestSwitchFromARetiredFamilyKeepsAFullLifetime(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "retired-switch-lifetime")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access).FamilyID
	_, _, err := f.switchToTarget(ctx, source)
	require.NoError(t, err)
	require.False(t, headOfFamily(t, f.db, source).Expiry.After(time.Now().Add(recoveryGrace)), "the first switch retired the source")

	access, _, err := f.switchToTarget(ctx, source)
	require.NoError(t, err)
	head := headOfFamily(t, f.db, accessSessionOf(t, access).FamilyID)
	assert.True(t, head.Expiry.After(time.Now().Add(time.Hour)), "the second switch keeps a full lifetime")
	assert.Nil(t, head.FamilyExpiryCap, "a retirement grace is no transition deadline")
}

// TestLegacyDeadlineSurvivesConsecutiveSwitches pins that the transition
// deadline travels with the lineage: a family switched out of a legacy one
// is linked, and a further switch out of it must still keep the deadline.
func TestLegacyDeadlineSurvivesConsecutiveSwitches(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "legacy-switch-twice")
	ctx := testpkg.Ctx(t)
	family := accessSessionOf(t, f.access).FamilyID
	deadline := ageFamily(t, f.db, family, time.Hour)

	firstAccess, _, err := f.switchToTarget(ctx, family)
	require.NoError(t, err)
	secondAccess, secondRefresh, err := f.service.SwitchTenant(ctx, f.accountID, fmt.Sprintf("t%d", f.homeTenantID), accessSessionOf(t, firstAccess).FamilyID)
	require.NoError(t, err)

	second := accessSessionOf(t, secondAccess).FamilyID
	head := headOfFamily(t, f.db, second)
	require.NotNil(t, head.FamilyExpiryCap, "the second switch keeps the legacy deadline")
	assert.WithinDuration(t, deadline, *head.FamilyExpiryCap, time.Second)
	assert.False(t, head.Expiry.After(deadline))

	_, _, err = f.service.RefreshToken(ctx, secondRefresh)
	require.NoError(t, err)
	assert.False(t, headOfFamily(t, f.db, second).Expiry.After(deadline), "its rotation keeps the deadline too")
}

// TestEveryLegacyFamilyKeepsItsDeadline pins the conservative transition of
// #5422: no rule can tell which family issued before it came from a switch.
// A switch also accepts the current school, so an account with one school
// can hold unlinked siblings from two switches there. Every family without a
// lineage origin keeps its deadline at its first rotation, whatever school
// count or portal it has.
func TestEveryLegacyFamilyKeepsItsDeadline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("legacy-single-school")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	school := testpkg.Tenant(t)
	testpkg.EnsureAccountTenant(t, db, account.ID, school)
	loginAccess, _, err := service.IssueTokensForAuthenticatedAccount(ctx, account.ID, school, "", "")
	require.NoError(t, err)
	login := accessSessionOf(t, loginAccess).FamilyID
	slug := fmt.Sprintf("t%d", school)
	firstAccess, firstRefresh, err := service.SwitchTenant(ctx, account.ID, slug, login)
	require.NoError(t, err, "a switch into the current school")
	secondAccess, secondRefresh, err := service.SwitchTenant(ctx, account.ID, slug, login)
	require.NoError(t, err, "a second tab switches into the same school")

	for name, sibling := range map[string]struct{ access, refresh string }{
		"first": {firstAccess, firstRefresh}, "second": {secondAccess, secondRefresh},
	} {
		family := accessSessionOf(t, sibling.access).FamilyID
		deadline := ageFamily(t, db, family, time.Hour)

		_, rotated, err := service.RefreshToken(ctx, legacyRefreshToken(t, sibling.refresh))
		require.NoError(t, err, name)
		head := headOfFamily(t, db, family)
		assert.False(t, head.Expiry.After(deadline), "%s: a legacy family of a single-school account does not renew", name)
		require.NotNil(t, head.FamilyExpiryCap, name)
		assert.WithinDuration(t, deadline, *head.FamilyExpiryCap, time.Second, name)

		_, _, err = service.RefreshToken(ctx, rotated)
		require.NoError(t, err, name)
		assert.False(t, headOfFamily(t, db, family).Expiry.After(deadline), "%s: later rotations keep the deadline", name)
	}
}
