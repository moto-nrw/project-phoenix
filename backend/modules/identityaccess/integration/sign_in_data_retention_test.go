package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The fixed retention of short-lived sign-in data (#3954): guardian
// invitations go 30 days after expiry or redemption, e-mail codes and passkey
// ceremonies 7 days after expiry, for accounts and operators alike. The
// sweeps reach every school, so each test owns its database.

func retentionRowExists(t *testing.T, db *bun.DB, table string, id any) bool {
	t.Helper()
	var found bool
	require.NoError(t, db.NewRaw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = ?)", id).Scan(context.Background(), &found))
	return found
}

func TestGuardianInvitationsAreDeletedThirtyDaysAfterExpiryOrRedemption(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := newOperatorTokens(t, db)
	creator := testpkg.CreateTestAccount(t, db, "retention-inviter@example.com")
	profile := testpkg.CreateTestGuardianProfile(t, db, "retention-guardian@example.com")
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	otherProfile := testpkg.CreateTestGuardianProfileForTenant(t, db, otherSchool, "Andere", "Schule", "retention-other@example.com")
	now := time.Now()
	invitation := func(tenantID, profileID int64, expiresAt time.Time, acceptedAt *time.Time) int64 {
		return testpkg.InsertTestGuardianInvitation(t, db, &testpkg.GuardianInvitation{
			TenantID: tenantID, Token: uuid.Must(uuid.NewV4()).String(), GuardianProfileID: profileID,
			CreatedBy: creator.ID, ExpiresAt: expiresAt, AcceptedAt: acceptedAt,
		}).ID
	}
	daysAgo := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }
	acceptedLongAgo, acceptedRecently := daysAgo(31), daysAgo(29)

	expired := invitation(testpkg.Tenant(t), profile.ID, daysAgo(31), nil)
	recentlyExpired := invitation(testpkg.Tenant(t), profile.ID, daysAgo(29), nil)
	open := invitation(testpkg.Tenant(t), profile.ID, now.Add(48*time.Hour), nil)
	redeemed := invitation(testpkg.Tenant(t), profile.ID, now.Add(48*time.Hour), &acceptedLongAgo)
	recentlyRedeemed := invitation(testpkg.Tenant(t), profile.ID, daysAgo(60), &acceptedRecently)
	otherSchoolExpired := invitation(otherSchool, otherProfile.ID, daysAgo(31), nil)

	deleted, err := module.DeleteExpiredGuardianInvitations(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 3, deleted)
	assert.False(t, retentionRowExists(t, db, "auth.guardian_invitations", expired), "an invitation expired more than 30 days ago is deleted")
	assert.False(t, retentionRowExists(t, db, "auth.guardian_invitations", redeemed), "an invitation redeemed more than 30 days ago is deleted, even before its expiry")
	assert.False(t, retentionRowExists(t, db, "auth.guardian_invitations", otherSchoolExpired), "the sweep reaches every school")
	assert.True(t, retentionRowExists(t, db, "auth.guardian_invitations", recentlyExpired), "an invitation expired less than 30 days ago stays")
	assert.True(t, retentionRowExists(t, db, "auth.guardian_invitations", open), "an open invitation stays")
	assert.True(t, retentionRowExists(t, db, "auth.guardian_invitations", recentlyRedeemed), "redemption starts the window, not the earlier expiry")
}

func TestMFAEmailCodesAreDeletedSevenDaysAfterExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := newOperatorTokens(t, db)
	account := testpkg.CreateTestAccount(t, db, "retention-mfa@example.com")
	operator := testpkg.CreateTestOperator(t, db)
	now := time.Now()
	insert := func(table, owner string, ownerID int64, expiresAt time.Time) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO `+table+` (`+owner+`, code_hash, expires_at, ip_address)
			VALUES (?, 'hash', ?, '192.0.2.1') RETURNING id`, ownerID, expiresAt).Scan(context.Background(), &id))
		return id
	}
	const accountTable, operatorTable = "auth.mfa_email_challenges", "platform.operator_mfa_email_challenges"

	accountExpired := insert(accountTable, "account_id", account.ID, now.Add(-8*24*time.Hour))
	accountRecent := insert(accountTable, "account_id", account.ID, now.Add(-6*24*time.Hour))
	accountLive := insert(accountTable, "account_id", account.ID, now.Add(10*time.Minute))
	operatorExpired := insert(operatorTable, "operator_id", operator.ID, now.Add(-8*24*time.Hour))
	operatorRecent := insert(operatorTable, "operator_id", operator.ID, now.Add(-6*24*time.Hour))

	deleted, err := module.DeleteExpiredMFAEmailChallenges(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, deleted)
	assert.False(t, retentionRowExists(t, db, accountTable, accountExpired), "an account code expired more than 7 days ago is deleted with its IP address")
	assert.False(t, retentionRowExists(t, db, operatorTable, operatorExpired), "an operator code expired more than 7 days ago is deleted")
	assert.True(t, retentionRowExists(t, db, accountTable, accountRecent), "a code expired less than 7 days ago stays")
	assert.True(t, retentionRowExists(t, db, accountTable, accountLive), "a live code stays")
	assert.True(t, retentionRowExists(t, db, operatorTable, operatorRecent))
}

func TestPasskeySessionsAreDeletedSevenDaysAfterExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := newOperatorTokens(t, db)
	now := time.Now()
	insert := func(table string, expiresAt time.Time) string {
		id := uuid.Must(uuid.NewV4()).String()
		_, err := db.ExecContext(context.Background(), `INSERT INTO `+table+` (id, purpose, rp_id, expected_origin, session_json, expires_at)
			VALUES (?, 'login', 'localhost', 'http://localhost', '{}'::jsonb, ?)`, id, expiresAt)
		require.NoError(t, err)
		return id
	}
	const accountTable, operatorTable = "auth.passkey_sessions", "platform.operator_passkey_sessions"

	accountExpired := insert(accountTable, now.Add(-8*24*time.Hour))
	accountRecent := insert(accountTable, now.Add(-6*24*time.Hour))
	operatorExpired := insert(operatorTable, now.Add(-8*24*time.Hour))
	operatorLive := insert(operatorTable, now.Add(5*time.Minute))

	deleted, err := module.DeleteExpiredPasskeySessions(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, deleted)
	assert.False(t, retentionRowExists(t, db, accountTable, accountExpired), "an account ceremony expired more than 7 days ago is deleted")
	assert.False(t, retentionRowExists(t, db, operatorTable, operatorExpired), "an operator ceremony expired more than 7 days ago is deleted")
	assert.True(t, retentionRowExists(t, db, accountTable, accountRecent), "a ceremony expired less than 7 days ago stays")
	assert.True(t, retentionRowExists(t, db, operatorTable, operatorLive), "a live ceremony stays")
}
