package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newThread(studentID, guardianAccountID int64, subject string) *usersModels.ParentMessageThread {
	t := &usersModels.ParentMessageThread{
		StudentID:         studentID,
		GuardianAccountID: guardianAccountID,
		Subject:           subject,
	}
	t.SetTenantID(1)
	return t
}

func newMessage(threadID, studentID, accountID int64, kind, body string) *usersModels.ParentMessage {
	m := &usersModels.ParentMessage{
		ThreadID:        threadID,
		StudentID:       studentID,
		SenderAccountID: accountID,
		SenderKind:      kind,
		SenderName:      "Test",
		Body:            body,
	}
	m.SetTenantID(1)
	return m
}

// TestParentMessaging_ThreadsMessagesAndReadState exercises the full data layer:
// create a thread for a (child, guardian), append guardian + staff messages
// oldest-first, and verify the read-cursor / unread arithmetic that drives both
// the staff inbox and the parent thread list.
func TestParentMessaging_ThreadsMessagesAndReadState(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := usersRepo.NewParentMessageReadRepository(db)
	ctx := tenantCtx()
	reader := chain.AccountID

	thread := newThread(chain.StudentID, chain.AccountID, "Abholung")
	require.NoError(t, threadRepo.Create(ctx, thread))
	require.Positive(t, thread.ID)

	found, err := threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Abholung", found.Subject)
	assert.Equal(t, chain.AccountID, found.GuardianAccountID)

	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "hallo")))
	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "noch was")))
	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderStaff, "antwort")))

	messages, err := msgRepo.ListByThread(ctx, thread.ID, 0)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, "hallo", messages[0].Body)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, messages[2].SenderKind)

	// Staff inbox: one row, guardian + child names, unread = 2 guardian msgs.
	inbox, err := readRepo.ListInboxForStaff(ctx, reader, true, nil, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, "Abholung", inbox[0].Subject)
	assert.Equal(t, "Felix Schneider", inbox[0].StudentName)
	assert.Equal(t, "Sabine Schneider", inbox[0].GuardianName)
	assert.Equal(t, "parent", inbox[0].RelationshipType)
	assert.Equal(t, 2, inbox[0].UnreadCount)

	threadCount, err := readRepo.UnreadThreadCountForStaff(ctx, reader, true, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, threadCount)

	// After reading, unread clears.
	require.NoError(t, readRepo.MarkRead(ctx, 1, thread.ID, reader))
	threadCount, err = readRepo.UnreadThreadCountForStaff(ctx, reader, true, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, threadCount)

	onlyUnread, err := readRepo.ListInboxForStaff(ctx, reader, true, nil, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread)

	// Guardian thread list: unread counts STAFF messages after the cursor.
	// The reader has read up to now, so the staff message is read → 0 unread.
	guardianThreads, err := readRepo.ListThreadsForGuardian(ctx, reader)
	require.NoError(t, err)
	require.Len(t, guardianThreads, 1)
	assert.Equal(t, "Abholung", guardianThreads[0].Subject)
	assert.Equal(t, 0, guardianThreads[0].UnreadCount)

	// A new staff message makes the guardian list show 1 unread.
	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderStaff, "noch eine antwort")))
	guardianThreads, err = readRepo.ListThreadsForGuardian(ctx, reader)
	require.NoError(t, err)
	require.Len(t, guardianThreads, 1)
	assert.Equal(t, 1, guardianThreads[0].UnreadCount)
}

// TestParentMessaging_MultipleThreadsPerGuardian verifies a guardian can have
// several conversations about the same child, each listed separately.
func TestParentMessaging_MultipleThreadsPerGuardian(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	readRepo := usersRepo.NewParentMessageReadRepository(db)
	ctx := tenantCtx()

	for _, subject := range []string{"Abholung", "Krankmeldung", "Allgemeines"} {
		require.NoError(t, threadRepo.Create(ctx, newThread(chain.StudentID, chain.AccountID, subject)))
	}

	threads, err := readRepo.ListThreadsForGuardian(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Len(t, threads, 3, "guardian should have three separate threads about the same child")
}

// TestParentMessaging_ListGuardiansForStudent returns the child's
// account-holding guardians for the staff recipient picker.
func TestParentMessaging_ListGuardiansForStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	guardians, err := threadRepo.ListGuardiansForStudent(tenantCtx(), chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	assert.Equal(t, chain.AccountID, guardians[0].AccountID)
	assert.Equal(t, "Sabine Schneider", guardians[0].Name)
	assert.Equal(t, "parent", guardians[0].RelationshipType)
	assert.True(t, guardians[0].IsPrimary)
}
