package contracttest_test

import (
	"context"
	"testing"
	"time"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestParentAudienceWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	poll, _ := api.ParentStorePollAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time { return databaseTimestamp(t, db) }, repo, chain.AccountID, chain.TenantID, userModels.ParentAnnouncementResponseSingleChoice)
	membershipID := testpkg.SeparateStudentMembership(t, db, chain.StudentID)
	testpkg.AssertStudentCompatibilityStorageAbsent(t, db)

	count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{chain.TenantID}}))
	require.NoError(t, err)
	require.Len(t, feed, 1)
	children, err := testpkg.TenantQuery2(t, ctx, db, repo.PollChildren, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, chain.StudentID, children[0].StudentID)
	require.Equal(t, "1a", children[0].SchoolClass)
	may, err := testpkg.TenantQuery4(t, ctx, db, repo.AccountMayAnswerForStudent, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.True(t, may)
	count, err = testpkg.TenantQuery2(t, ctx, db, repo.CountReachableGuardiansForStudents, chain.TenantID, []int64{chain.StudentID})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	testpkg.AssertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		may, err := testpkg.TenantQuery4(t, txCtx, db, repo.AccountMayAnswerForStudent, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.Equal(t, visible, may)
	})
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access AS a SET permissions = '{}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id AND r.student_id = ?`, chain.StudentID)
	require.NoError(t, err)
	may, err = testpkg.TenantQuery4(t, ctx, db, repo.AccountMayAnswerForStudent, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.False(t, may, "a relationship and active account are not parent authorization")
	// Restore the fixture's explicit permissions so the row-existence checks
	// below cannot pass merely because authorization was removed.
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access AS a SET permissions = '{"parent_portal.access":true,"parent_portal.poll.response":true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id AND r.student_id = ?`, chain.StudentID)
	require.NoError(t, err)

	testpkg.AssertMissingStudentProjectionStates(t, db, membershipID, func() {
		count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Zero(t, count)
		children, err := testpkg.TenantQuery2(t, ctx, db, repo.PollChildren, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Empty(t, children)
		may, err := testpkg.TenantQuery4(t, ctx, db, repo.AccountMayAnswerForStudent, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.False(t, may)
	})
}

func TestParentInboxWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestAccount(t, db, "owner-inbox")
	repos := parentRepos(t, db)
	ctx := tenantCtx(t)
	thread := api.ParentStoreThread(testpkg.Tenant(t), chain.StudentID, chain.AccountID)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.Thread.Create, thread))
	message := api.ParentStoreMessage(testpkg.Tenant(t), thread.ID, chain.StudentID, chain.AccountID, owner0.ParentMessageSenderGuardian, "Owner read")
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repos.Message.Create, message))
	membershipID := testpkg.SeparateStudentMembership(t, db, chain.StudentID)
	testpkg.AssertStudentCompatibilityStorageAbsent(t, db)

	rows, err := testpkg.TenantQuery3(t, ctx, db, repos.Read.ListInboxForStaff, staff.ID, true, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, chain.StudentID, rows[0].StudentID)
	require.Equal(t, "1a", rows[0].SchoolClass)
	count, err := testpkg.TenantQuery2(t, ctx, db, repos.Read.UnreadMessageCountForStaff, staff.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	header, err := testpkg.TenantQuery1(t, ctx, db, repos.Read.FindThreadHeader, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, header)

	testpkg.AssertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		header, err := repos.Read.FindThreadHeader(txCtx, thread.ID)
		require.NoError(t, err)
		require.Equal(t, visible, header != nil)
	})

	testpkg.AssertMissingStudentProjectionStates(t, db, membershipID, func() {
		rows, err := testpkg.TenantQuery3(t, ctx, db, repos.Read.ListInboxForStaff, staff.ID, true, false)
		require.NoError(t, err)
		require.Empty(t, rows)
		count, err := testpkg.TenantQuery2(t, ctx, db, repos.Read.UnreadMessageCountForStaff, staff.ID, true)
		require.NoError(t, err)
		require.Zero(t, count)
		header, err := testpkg.TenantQuery1(t, ctx, db, repos.Read.FindThreadHeader, thread.ID)
		require.NoError(t, err)
		require.Nil(t, header)
	})
}
