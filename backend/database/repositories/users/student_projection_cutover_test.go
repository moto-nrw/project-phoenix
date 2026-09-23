package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestParentAudienceWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)
	poll, _ := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID, usersModels.ParentAnnouncementResponseSingleChoice)
	membershipID := testpkg.SeparateStudentMembership(t, db, chain.StudentID)
	testpkg.AssertStudentCompatibilityStorageAbsent(t, db)

	count, err := repo.CountAudience(ctx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, usersModels.AnnouncementFeedScope{TenantIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, feed, 1)
	children, err := repo.PollChildren(ctx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, chain.StudentID, children[0].StudentID)
	require.Equal(t, "1a", children[0].SchoolClass)
	may, err := repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.True(t, may)
	count, err = repo.CountReachableGuardiansForStudents(ctx, chain.TenantID, []int64{chain.StudentID})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	testpkg.AssertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		may, err := repo.AccountMayAnswerForStudent(txCtx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.Equal(t, visible, may)
	})
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = '{}'::jsonb WHERE student_id = ?`, chain.StudentID)
	require.NoError(t, err)
	may, err = repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.False(t, may, "a relationship and active account are not parent authorization")
	// Restore the fixture's explicit permissions so the row-existence checks
	// below cannot pass merely because authorization was removed.
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = '{"parent_portal.access":true,"parent_portal.poll.response":true}'::jsonb WHERE student_id = ?`, chain.StudentID)
	require.NoError(t, err)

	testpkg.AssertMissingStudentProjectionStates(t, db, membershipID, func() {
		count, err := repo.CountAudience(ctx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Zero(t, count)
		children, err := repo.PollChildren(ctx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Empty(t, children)
		may, err := repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
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
	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, repos.Thread.Create(ctx, thread))
	message := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "Owner read")
	require.NoError(t, repos.Message.Create(ctx, message))
	membershipID := testpkg.SeparateStudentMembership(t, db, chain.StudentID)
	testpkg.AssertStudentCompatibilityStorageAbsent(t, db)

	rows, err := repos.Read.ListInboxForStaff(ctx, staff.ID, true, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, chain.StudentID, rows[0].StudentID)
	require.Equal(t, "1a", rows[0].SchoolClass)
	count, err := repos.Read.UnreadMessageCountForStaff(ctx, staff.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	header, err := repos.Read.FindThreadHeader(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, header)

	testpkg.AssertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		header, err := repos.Read.FindThreadHeader(txCtx, thread.ID)
		require.NoError(t, err)
		require.Equal(t, visible, header != nil)
	})

	testpkg.AssertMissingStudentProjectionStates(t, db, membershipID, func() {
		rows, err := repos.Read.ListInboxForStaff(ctx, staff.ID, true, false)
		require.NoError(t, err)
		require.Empty(t, rows)
		count, err := repos.Read.UnreadMessageCountForStaff(ctx, staff.ID, true)
		require.NoError(t, err)
		require.Zero(t, count)
		header, err := repos.Read.FindThreadHeader(ctx, thread.ID)
		require.NoError(t, err)
		require.Nil(t, header)
	})
}
