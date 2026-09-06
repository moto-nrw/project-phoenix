package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPrivacyEventsTimestampAfterTheSerializedWriteStarts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	protection := familyProtectionEvent(chain.StudentID, chain.AccountID, chain.TenantID, true)
	share := requestShareEvent(chain.StudentID, chain.AccountID, chain.TenantID, nil)
	var transactionStarted time.Time
	err := tenant.WithTenantTx(ctx, db, chain.TenantID, func(txCtx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().ColumnExpr("CURRENT_TIMESTAMP").Scan(txCtx, &transactionStarted); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
		if err := repos.FamilyProtection.Create(txCtx, protection); err != nil {
			return err
		}
		return repos.ParentRequestShare.Create(txCtx, share)
	})
	require.NoError(t, err)
	assert.True(t, protection.CreatedAt.After(transactionStarted))
	assert.True(t, share.CreatedAt.After(transactionStarted))
}

func TestFamilyProtectionCurrentUsesAppendOrderNotTransactionTimestamp(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	older := familyProtectionEvent(chain.StudentID, chain.AccountID, chain.TenantID, false)
	newer := familyProtectionEvent(chain.StudentID, chain.AccountID, chain.TenantID, true)
	require.NoError(t, repos.FamilyProtection.Create(ctx, older))
	require.NoError(t, repos.FamilyProtection.Create(ctx, newer))
	invertPrivacyEventTimestamps(t, db, ctx, "users.student_family_protection_events", older.ID, newer.ID)

	current, err := repos.FamilyProtection.CurrentForStudents(ctx, []int64{chain.StudentID})
	require.NoError(t, err)
	assert.Equal(t, newer.ID, current[chain.StudentID].ID)
	assert.True(t, current[chain.StudentID].Enabled)
}

func TestRequestSharingCurrentUsesAppendOrderNotTransactionTimestamp(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	older := requestShareEvent(chain.StudentID, chain.AccountID, chain.TenantID, []int64{91})
	newer := requestShareEvent(chain.StudentID, chain.AccountID, chain.TenantID, []int64{92})
	require.NoError(t, repos.ParentRequestShare.Create(ctx, older))
	require.NoError(t, repos.ParentRequestShare.Create(ctx, newer))
	invertPrivacyEventTimestamps(t, db, ctx, "users.parent_request_share_events", older.ID, newer.ID)

	current, err := repos.ParentRequestShare.CurrentForStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	assert.Equal(t, []int64{92}, current[0].RecipientAccountIDs)
}

func familyProtectionEvent(studentID, actorID, tenantID int64, enabled bool) *userModels.FamilyProtectionEvent {
	event := &userModels.FamilyProtectionEvent{StudentID: studentID, ActorAccountID: actorID, Enabled: enabled, Reason: "Test"}
	event.SetTenantID(tenantID)
	return event
}

func requestShareEvent(studentID, authorID, tenantID int64, recipients []int64) *userModels.ParentRequestShareEvent {
	event := &userModels.ParentRequestShareEvent{
		StudentID: studentID, AuthorAccountID: authorID, RequestType: "master_data", RequestID: 77,
		RecipientAccountIDs: recipients,
	}
	event.SetTenantID(tenantID)
	return event
}

func invertPrivacyEventTimestamps(t *testing.T, db *bun.DB, ctx context.Context, table string, olderID, newerID int64) {
	t.Helper()
	_, err := db.NewUpdate().TableExpr(table).
		Set("created_at = CASE WHEN id = ? THEN ?::timestamptz ELSE ?::timestamptz END", olderID, time.Now(), time.Now().Add(-time.Hour)).
		Where("id IN (?, ?)", olderID, newerID).Exec(ctx)
	require.NoError(t, err)
}
