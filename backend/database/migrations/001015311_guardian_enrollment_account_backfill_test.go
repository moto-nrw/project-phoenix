package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func insertGuardianBackfillRequest(t *testing.T, db *testpkg.DB, phaseID int64, email string, accountID *int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email,
			 guardian_account_id, consent_flags, custom_data, submission_source,
			 source_metadata, status_token, submitted_at)
		VALUES (?, ?, 'Guardian', 'Backfill', ?, ?, '{}'::jsonb, '{}'::jsonb,
			'public', '{}'::jsonb, ?, ?)
		RETURNING id
	`, testpkg.Tenant(t), phaseID, email, accountID,
		fmt.Sprintf("guardian-account-backfill-%d", time.Now().UnixNano()), time.Now()).Scan(context.Background(), &id))
	return id
}

func guardianAccountIDForRequest(t *testing.T, db *testpkg.DB, requestID int64) *int64 {
	t.Helper()
	var accountID *int64
	require.NoError(t, db.NewRaw(`
		SELECT guardian_account_id
		FROM enrollment.requests
		WHERE id = ?
	`, requestID).Scan(context.Background(), &accountID))
	return accountID
}

func TestGuardianEnrollmentAccountBackfill(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	uniqueAccount := testpkg.CreateTestAccount(t, db, "guardian-backfill-unique")
	guardianProfile := testpkg.CreateTestGuardianProfile(t, db, "guardian-backfill-profile")
	_, err := db.NewRaw(`
		UPDATE users.guardian_profiles
		SET account_id = ?, has_account = TRUE
		WHERE id = ?
	`, uniqueAccount.ID, guardianProfile.ID).Exec(context.Background())
	require.NoError(t, err)
	matching := insertGuardianBackfillRequest(t, db, phase.ID, "  "+strings.ToUpper(uniqueAccount.Email)+"  ", nil)
	staffOnlyAccount := testpkg.CreateTestAccount(t, db, "guardian-backfill-staff-only")
	staffOnly := insertGuardianBackfillRequest(t, db, phase.ID, staffOnlyAccount.Email, nil)

	existingOwner := testpkg.CreateTestAccount(t, db, "guardian-backfill-owner")
	alreadyLinked := insertGuardianBackfillRequest(t, db, phase.ID, uniqueAccount.Email, &existingOwner.ID)
	unrelated := insertGuardianBackfillRequest(t, db, phase.ID, "no-account@example.invalid", nil)

	ambiguousAccount := testpkg.CreateTestAccount(t, db, "guardian-backfill-ambiguous")
	var duplicateID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO auth.accounts (email, active) VALUES (?, TRUE) RETURNING id
	`, strings.ToUpper(ambiguousAccount.Email)).Scan(context.Background(), &duplicateID))
	testpkg.EnsureAccountTenant(t, db, duplicateID, testpkg.Tenant(t))
	ambiguous := insertGuardianBackfillRequest(t, db, phase.ID, ambiguousAccount.Email, nil)

	require.NoError(t, guardianEnrollmentAccountBackfillUp(context.Background(), db))

	matchingAccountID := guardianAccountIDForRequest(t, db, matching)
	require.NotNil(t, matchingAccountID)
	assert.Equal(t, uniqueAccount.ID, *matchingAccountID)
	alreadyLinkedAccountID := guardianAccountIDForRequest(t, db, alreadyLinked)
	require.NotNil(t, alreadyLinkedAccountID)
	assert.Equal(t, existingOwner.ID, *alreadyLinkedAccountID)
	assert.Nil(t, guardianAccountIDForRequest(t, db, unrelated))
	assert.Nil(t, guardianAccountIDForRequest(t, db, ambiguous))
	assert.Nil(t, guardianAccountIDForRequest(t, db, staffOnly), "staff-only accounts must not receive enrollment ownership")

	require.NoError(t, guardianEnrollmentAccountBackfillUp(context.Background(), db), "backfill must be idempotent")
	require.NoError(t, guardianEnrollmentAccountBackfillDown(context.Background(), db))
	matchingAccountID = guardianAccountIDForRequest(t, db, matching)
	require.NotNil(t, matchingAccountID)
	assert.Equal(t, uniqueAccount.ID, *matchingAccountID, "down migration must not erase valid ownership")
}
