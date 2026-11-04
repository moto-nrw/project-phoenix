package contracttest_test

import (
	"testing"

	owner0 "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParentMessaging_TenantIsolation pins that a school never reads another
// school's conversation rows, across all three tables the Communication owner
// holds: users.parent_message_threads, users.parent_messages and
// users.parent_message_reads.
//
// It matters more since the SQL moved out of the generic repository base: the
// entity reads now apply their own tenant_id filter, and the inbox, unread and
// receipt reads run in a projection that joins People Directory rows. A missing
// filter there is a cross-school leak of a child's name and a parent's message
// body, not a cosmetic bug — and the cross-tenant guardian queries deliberately
// run without RLS, so the filter is the only thing standing between two schools.
func TestParentMessaging_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staffAccount := testpkg.CreateTestAccount(t, db, "isolation-staff")

	repos := parentRepos(t, db)
	homeCtx := tenantCtx(t)

	thread := api.ParentStoreThread(testpkg.Tenant(t), chain.StudentID, chain.AccountID)
	require.NoError(t, testpkg.TenantCommand1(t, homeCtx, db, repos.Thread.Create, thread))
	message := api.ParentStoreMessage(testpkg.Tenant(t), thread.ID, chain.StudentID, chain.AccountID,
		owner0.ParentMessageSenderGuardian, "Nur fuer diese Schule")
	require.NoError(t, testpkg.TenantCommand1(t, homeCtx, db, repos.Message.Create, message))
	advanced, err := testpkg.TenantQuery5(t, homeCtx, db, repos.Read.MarkReadUpTo, testpkg.Tenant(t), thread.ID, staffAccount.ID, message.CreatedAt, message.ID)
	require.NoError(t, err)
	require.True(t, advanced, "the fixture cursor must actually advance")

	// A second school that shares nothing with the first.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreignCtx := testpkg.TenantContext(otherTenant)

	t.Run("thread reads are scoped to the owning school", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, foreignCtx, db, repos.Thread.FindByID, thread.ID)
		require.NoError(t, err)
		assert.Nil(t, found, "another school must not load the thread by id")

		byPair, err := testpkg.TenantQuery3(t, foreignCtx, db, repos.Thread.FindByStudentGuardian, chain.StudentID, chain.AccountID, "team")
		require.NoError(t, err)
		assert.Nil(t, byPair, "another school must not resolve the (student, guardian) conversation")
	})

	t.Run("message reads are scoped to the owning school", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, foreignCtx, db, repos.Message.FindByID, message.ID)
		require.NoError(t, err)
		assert.Nil(t, found, "another school must not load the message by id")

		listed, err := testpkg.TenantQuery2(t, foreignCtx, db, repos.Message.ListByThread, thread.ID, 0)
		require.NoError(t, err)
		assert.Empty(t, listed, "another school must not read the conversation body")
	})

	t.Run("the inbox projection is scoped to the owning school", func(t *testing.T) {
		// allStudents=true is the widest staff scope there is, so nothing but the
		// tenant filter can be keeping these rows out.
		inbox, err := testpkg.TenantQuery3(t, foreignCtx, db, repos.Read.ListInboxForStaff, staffAccount.ID, api.ParentMessageReachAllStudents, false)
		require.NoError(t, err)
		assert.Empty(t, inbox, "another school's staff must not see the thread in their inbox")

		unread, err := testpkg.TenantQuery2(t, foreignCtx, db, repos.Read.UnreadMessageCountForStaff, staffAccount.ID, api.ParentMessageReachAllStudents)
		require.NoError(t, err)
		assert.Zero(t, unread, "another school's badge must not count this school's messages")

		threads, err := testpkg.TenantQuery3(t, foreignCtx, db, repos.Read.ListThreadsForStudent, staffAccount.ID, chain.StudentID, api.ParentMessageReachAllStudents)
		require.NoError(t, err)
		assert.Empty(t, threads, "another school must not list the child's conversations")

		header, err := testpkg.TenantQuery1(t, foreignCtx, db, repos.Read.FindThreadHeader, thread.ID)
		require.NoError(t, err)
		assert.Nil(t, header, "another school must not read the child and guardian names")
	})

	t.Run("read cursors are scoped to the owning school", func(t *testing.T) {
		cursor, err := testpkg.TenantQuery1(t, foreignCtx, db, repos.Read.GuardianReadCursor, thread.ID)
		require.NoError(t, err)
		assert.Nil(t, cursor, "another school must not read the guardian's position")

		other, err := testpkg.TenantQuery2(t, foreignCtx, db, repos.Read.LatestReadCursorByOther, thread.ID, chain.AccountID)
		require.NoError(t, err)
		assert.Nil(t, other, "another school's staff cursor must not satisfy the read receipt")
	})

	t.Run("the owning school still reads its own rows", func(t *testing.T) {
		// The mirror assertion: without it a filter that excludes everything
		// would pass every check above.
		found, err := testpkg.TenantQuery1(t, homeCtx, db, repos.Thread.FindByID, thread.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		listed, err := testpkg.TenantQuery2(t, homeCtx, db, repos.Message.ListByThread, thread.ID, 0)
		require.NoError(t, err)
		assert.Len(t, listed, 1)

		header, err := testpkg.TenantQuery1(t, homeCtx, db, repos.Read.FindThreadHeader, thread.ID)
		require.NoError(t, err)
		require.NotNil(t, header)
	})
}
