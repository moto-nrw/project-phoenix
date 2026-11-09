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

// Coverage for 1.15.832 (#6294): an open link whose profile changed after
// the link was issued ends, every other link takes the address on file, a
// request awaiting staff keeps going, and Down ends what the binding ended
// before the column goes.
func TestGuardianInvitationAddressBinding(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db))

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

	// The profile was written before the link: the link went to its address.
	unchangedProfile, unchangedEmail := profile("binding-unchanged")
	unchanged := invitation(unchangedProfile, "not_required", false, -time.Minute)
	// The address changed after the link was issued.
	changedProfile, _ := profile("binding-changed")
	changed := invitation(changedProfile, "not_required", false, time.Hour)
	corrected := fmt.Sprintf("corrected-%d@example.test", time.Now().UnixNano())
	_, err := db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, corrected, changedProfile).Exec(ctx)
	require.NoError(t, err)
	// A request awaiting staff was never mailed; the approval mails a new link.
	pending := invitation(changedProfile, "pending", false, time.Hour)
	// Spent links stay as they are.
	accepted := invitation(changedProfile, "not_required", true, time.Hour)

	require.NoError(t, guardianInvitationAddressBindingUp(ctx, db))
	assertBound := func() {
		t.Helper()
		now := time.Now()
		row := read(unchanged)
		require.NotNil(t, row.Email)
		assert.Equal(t, unchangedEmail, *row.Email, "an unchanged profile keeps its link, bound to its address")
		assert.True(t, row.Expires.After(now), "an unchanged profile keeps its link open")

		row = read(changed)
		assert.False(t, row.Expires.After(now), "a link issued before the profile changed ends")
		require.NotNil(t, row.Email)
		assert.Equal(t, corrected, *row.Email)

		row = read(pending)
		assert.True(t, row.Expires.After(now), "a request awaiting staff keeps going")
		require.NotNil(t, row.Email)
		assert.True(t, read(accepted).Expires.After(now), "a spent link is left alone")
	}
	assertBound()
	require.NoError(t, guardianInvitationAddressBindingUp(ctx, db), "a rerun changes nothing")
	assertBound()

	// Down: a still open link whose address the profile no longer has would
	// follow the profile again without the column, so it ends first.
	_, err = db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`,
		fmt.Sprintf("changed-later-%d@example.test", time.Now().UnixNano()), unchangedProfile).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db))
	var expires time.Time
	require.NoError(t, db.NewRaw(`SELECT expires_at FROM auth.guardian_invitations WHERE id = ?`, unchanged).Scan(ctx, &expires))
	assert.False(t, expires.After(time.Now()), "Down ends a link whose address the profile no longer has")
	var pendingExpires time.Time
	require.NoError(t, db.NewRaw(`SELECT expires_at FROM auth.guardian_invitations WHERE id = ?`, pending).Scan(ctx, &pendingExpires))
	assert.True(t, pendingExpires.After(time.Now()), "Down keeps a request awaiting staff")
	require.NoError(t, guardianInvitationAddressBindingDown(ctx, db), "a second Down finds the column gone")
}
