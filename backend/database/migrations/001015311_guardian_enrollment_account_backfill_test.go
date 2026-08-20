package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func insertGuardianBackfillRequest(t *testing.T, db *bun.DB, phaseID int64, email string, accountID *int64) *enrollmentModels.Request {
	t.Helper()
	request := &enrollmentModels.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Guardian",
		GuardianLastName:  "Backfill",
		GuardianEmail:     email,
		GuardianAccountID: accountID,
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("guardian-account-backfill-%d", time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	request.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(request).ModelTableExpr(`enrollment.requests AS "request"`).Exec(context.Background())
	require.NoError(t, err)
	return request
}

func guardianAccountIDForRequest(t *testing.T, db *bun.DB, requestID int64) *int64 {
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
	matching := insertGuardianBackfillRequest(t, db, phase.ID, "  "+strings.ToUpper(uniqueAccount.Email)+"  ", nil)

	existingOwner := testpkg.CreateTestAccount(t, db, "guardian-backfill-owner")
	alreadyLinked := insertGuardianBackfillRequest(t, db, phase.ID, uniqueAccount.Email, &existingOwner.ID)
	unrelated := insertGuardianBackfillRequest(t, db, phase.ID, "no-account@example.invalid", nil)

	ambiguousAccount := testpkg.CreateTestAccount(t, db, "guardian-backfill-ambiguous")
	duplicate := &authModels.Account{Email: strings.ToUpper(ambiguousAccount.Email), Active: true}
	_, err := db.NewInsert().Model(duplicate).ModelTableExpr(`auth.accounts AS "account"`).Exec(context.Background())
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, duplicate.ID, testpkg.Tenant(t))
	ambiguous := insertGuardianBackfillRequest(t, db, phase.ID, ambiguousAccount.Email, nil)

	require.NoError(t, guardianEnrollmentAccountBackfillUp(context.Background(), db))

	matchingAccountID := guardianAccountIDForRequest(t, db, matching.ID)
	require.NotNil(t, matchingAccountID)
	assert.Equal(t, uniqueAccount.ID, *matchingAccountID)
	alreadyLinkedAccountID := guardianAccountIDForRequest(t, db, alreadyLinked.ID)
	require.NotNil(t, alreadyLinkedAccountID)
	assert.Equal(t, existingOwner.ID, *alreadyLinkedAccountID)
	assert.Nil(t, guardianAccountIDForRequest(t, db, unrelated.ID))
	assert.Nil(t, guardianAccountIDForRequest(t, db, ambiguous.ID))

	require.NoError(t, guardianEnrollmentAccountBackfillUp(context.Background(), db), "backfill must be idempotent")
	require.NoError(t, guardianEnrollmentAccountBackfillDown(context.Background(), db))
	matchingAccountID = guardianAccountIDForRequest(t, db, matching.ID)
	require.NotNil(t, matchingAccountID)
	assert.Equal(t, uniqueAccount.ID, *matchingAccountID, "down migration must not erase valid ownership")
}
