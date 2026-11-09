package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Coverage for 1.15.844 (#6294): every unbound legacy link ends regardless of
// mutable timestamps. Pending requests and bound links survive a rerun, and
// Down ends bound links whose profile address changed.
func TestGuardianInvitationAddressBinding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db))
	require.NoError(t, hashStoredTokensDown(ctx, db))

	tenantID := testpkg.Tenant(t)
	creator := testpkg.CreateTestAccount(t, db, "address-binding-creator")
	profile := func(seed string) (int64, string) {
		created := testpkg.CreateTestGuardianProfile(t, db, seed)
		return created.ID, *created.Email
	}
	// invitation plants a pre-migration row; issuedAgo places its last write
	// before (positive) or after (negative) the profile's.
	invitation := func(profileID int64, status string, accepted bool, issuedAgo time.Duration) int64 {
		var acceptedAt *time.Time
		if accepted {
			now := time.Now()
			acceptedAt = &now
		}
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.guardian_invitations
			(tenant_id, token, guardian_profile_id, created_by, expires_at, accepted_at, approval_status, created_at, updated_at)
			VALUES (?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			tenantID, profileID, creator.ID, time.Now().Add(48*time.Hour), acceptedAt, status,
			time.Now().Add(-issuedAgo), time.Now().Add(-issuedAgo)).Scan(ctx, &id))
		return id
	}
	type state struct {
		Email   *string   `bun:"email"`
		Expires time.Time `bun:"expires_at"`
	}
	read := func(id int64) state {
		var row state
		require.NoError(t, db.NewRaw(`SELECT email, expires_at FROM auth.guardian_invitations WHERE id = ?`, id).Scan(ctx, &row.Email, &row.Expires))
		return row
	}

	// Even an apparently unchanged profile cannot prove the mailed address.
	unchangedProfile, unchangedEmail := profile("binding-unchanged")
	unchanged := invitation(unchangedProfile, "not_required", false, -time.Minute)
	// The address changed after the link was issued.
	changedProfile, _ := profile("binding-changed")
	changed := invitation(changedProfile, "not_required", false, time.Hour)
	corrected := fmt.Sprintf("corrected-%d@example.test", time.Now().UnixNano())
	_, err := db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, corrected, changedProfile).Exec(ctx)
	require.NoError(t, err)
	approved := invitation(changedProfile, "approved", false, time.Hour)
	// Hashing in 1.15.521 advances updated_at without issuing a new link.
	rehashedProfile, _ := profile("binding-rehashed")
	rehashed := invitation(rehashedProfile, "not_required", false, time.Hour)
	_, err = db.NewRaw(`UPDATE auth.guardian_invitations SET token = gen_random_uuid()::text WHERE id = ?`, rehashed).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`,
		fmt.Sprintf("rehashed-corrected-%d@example.test", time.Now().UnixNano()), rehashedProfile).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, hashStoredTokensUp(ctx, db))
	var misleadingOrder bool
	require.NoError(t, db.NewRaw(`SELECT invitation.updated_at >= profile.updated_at
		FROM auth.guardian_invitations AS invitation
		JOIN users.guardian_profiles AS profile ON profile.id = invitation.guardian_profile_id
		WHERE invitation.id = ?`, rehashed).Scan(ctx, &misleadingOrder))
	require.True(t, misleadingOrder, "hashing makes the corrected profile appear older than the link")
	// Persist the timestamp order possible when an earlier-started correction
	// commits after issuance: now() records transaction start, not commit order.
	overlap := invitation(changedProfile, "not_required", false, -time.Minute)
	// A request awaiting staff was never mailed; the approval mails a new link.
	pending := invitation(changedProfile, "pending", false, time.Hour)
	// Spent links stay as they are.
	accepted := invitation(changedProfile, "not_required", true, time.Hour)
	expired := invitation(changedProfile, "not_required", false, time.Hour)
	_, err = db.NewRaw(`UPDATE auth.guardian_invitations SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Hour), expired).Exec(ctx)
	require.NoError(t, err)
	var expiredBefore time.Time
	require.NoError(t, db.NewRaw(`SELECT expires_at FROM auth.guardian_invitations WHERE id = ?`, expired).Scan(ctx, &expiredBefore))

	require.NoError(t, guardianInvitationAddressBindingUp(ctx, db))
	assertBound := func() {
		t.Helper()
		now := time.Now()
		for _, id := range []int64{unchanged, changed, approved, rehashed, overlap} {
			row := read(id)
			assert.False(t, row.Expires.After(now), "legacy link %d ends without proof of its recipient", id)
			assert.Nil(t, row.Email, "legacy link %d must not take the current profile address", id)
		}

		row := read(pending)
		assert.True(t, row.Expires.After(now), "a request awaiting staff keeps going")
		assert.Nil(t, row.Email, "approval will bind a fresh link to its recipient")
		row = read(accepted)
		assert.True(t, row.Expires.After(now), "a spent link is left alone")
		assert.Nil(t, row.Email, "a spent link keeps no address")
		row = read(expired)
		assert.Equal(t, expiredBefore, row.Expires, "an already expired link is left alone")
		assert.Nil(t, row.Email, "an expired link keeps no address")
	}
	assertBound()
	// A new application-issued link already has its recipient on a rerun.
	bound := invitation(unchangedProfile, "not_required", false, 0)
	_, err = db.NewRaw(`UPDATE auth.guardian_invitations SET email = ? WHERE id = ?`, unchangedEmail, bound).Exec(ctx)
	require.NoError(t, err)
	boundBefore := read(bound)
	require.NoError(t, guardianInvitationAddressBindingUp(ctx, db), "a rerun changes nothing")
	assertBound()
	assert.Equal(t, boundBefore, read(bound), "a bound link survives a rerun unchanged")

	// Down: a still open link whose address the profile no longer has would
	// follow the profile again without the column, so it ends first.
	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`,
		fmt.Sprintf("changed-later-%d@example.test", time.Now().UnixNano()), unchangedProfile).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db))
	var expires time.Time
	require.NoError(t, db.NewRaw(`SELECT expires_at FROM auth.guardian_invitations WHERE id = ?`, bound).Scan(ctx, &expires))
	assert.False(t, expires.After(time.Now()), "Down ends a link whose address the profile no longer has")
	var pendingExpires time.Time
	require.NoError(t, db.NewRaw(`SELECT expires_at FROM auth.guardian_invitations WHERE id = ?`, pending).Scan(ctx, &pendingExpires))
	assert.True(t, pendingExpires.After(time.Now()), "Down keeps a request awaiting staff")
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db), "a second Down finds the column gone")
}
