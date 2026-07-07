package users_test

// Extra repository tests for the parent-messaging data layer that the main
// repository test does not reach: ParentMessage.FindByID, the positive-limit
// "conversation tail" load, and the staff group-scoping branches of the inbox /
// unread-count queries (allStudents=false). The shared helpers (tenantCtx,
// newThread, newMessage) live in parent_messaging_repository_test.go.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestParentMessage_FindByIDAndTail covers FindByID (hit + miss returns nil, not
// an error) and the positive-limit tail load, which must return the most recent
// `limit` messages back in oldest-first chat order.
func TestParentMessage_FindByIDAndTail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	ctx := tenantCtx()

	thread := newThread(chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	bodies := []string{"eins", "zwei", "drei"}
	var lastID int64
	for _, b := range bodies {
		m := newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, b)
		require.NoError(t, msgRepo.Create(ctx, m))
		lastID = m.ID
	}

	// FindByID hit.
	found, err := msgRepo.FindByID(ctx, lastID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "drei", found.Body)

	// FindByID miss → nil, no error (an absent id is not a failure).
	missing, err := msgRepo.FindByID(ctx, lastID+999999)
	require.NoError(t, err)
	assert.Nil(t, missing)

	// Positive-limit tail: the two MOST RECENT messages, returned oldest-first.
	tail, err := msgRepo.ListByThread(ctx, thread.ID, 2)
	require.NoError(t, err)
	require.Len(t, tail, 2)
	assert.Equal(t, "zwei", tail[0].Body)
	assert.Equal(t, "drei", tail[1].Body)
}

// TestParentMessage_FindEventByRef covers the lookup a staff decision uses to
// mark exactly the "request created" pill read: it must match on
// (event_type, ref_table, ref_id) and ignore a same-ref decision pill, a plain
// chat message, and a non-matching ref (nil, not an error).
func TestParentMessage_FindEventByRef(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	ctx := tenantCtx()

	thread := newThread(chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	const refTable = "schedule.care_schedule_change_requests"
	refID := int64(4242)

	pill := newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderSystem, "Anfrage gestellt")
	pill.Kind = usersModels.ParentMessageKindEvent
	pill.EventType = "request_created"
	pill.EventActorKind = usersModels.ParentMessageSenderGuardian
	pill.RefTable = refTable
	pill.RefID = &refID
	require.NoError(t, msgRepo.Create(ctx, pill))

	// Same ref, but a decision pill (different event_type) — must NOT match.
	decision := newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderSystem, "Anfrage bestätigt")
	decision.Kind = usersModels.ParentMessageKindEvent
	decision.EventType = "request_status"
	decision.RefTable = refTable
	decision.RefID = &refID
	require.NoError(t, msgRepo.Create(ctx, decision))
	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "Frage")))

	found, err := msgRepo.FindEventByRef(ctx, thread.ID, "request_created", refTable, refID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, pill.ID, found.ID)

	// Wrong ref_id → nil, no error.
	other, err := msgRepo.FindEventByRef(ctx, thread.ID, "request_created", refTable, refID+1)
	require.NoError(t, err)
	assert.Nil(t, other)

	// Wrong ref_table (a master-data request) → nil.
	none, err := msgRepo.FindEventByRef(ctx, thread.ID, "request_created", "users.student_data_change_requests", refID)
	require.NoError(t, err)
	assert.Nil(t, none)
}

// TestListInboxForStaff_GroupScoped pins the non-admin staff scoping branches of
// applyStaffScope: a staffer who supervises the child's education group sees the
// conversation, a staffer scoped to a DIFFERENT group does not, and a staffer who
// supervises NO groups (empty slice) sees nothing at all (the `1 = 0` guard that
// stops an unscoped staffer from reading the whole tenant inbox).
func TestListInboxForStaff_GroupScoped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	group := testpkg.CreateTestEducationGroup(t, db, "Gruppe Sonne")
	testpkg.AssignStudentToGroup(t, db, chain.StudentID, group.ID)

	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, db, staffAccount.ID)
	defer testpkg.CleanupParentMessagingForAccount(t, db, staffAccount.ID)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := usersRepo.NewParentMessageReadRepository(db)
	ctx := tenantCtx()

	thread := newThread(chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))
	require.NoError(t, msgRepo.Create(ctx, newMessage(thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "Frage")))

	// Supervises the child's group → sees the conversation, unread = 1.
	inbox, err := readRepo.ListInboxForStaff(ctx, staffAccount.ID, false, []int64{group.ID}, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, 1, inbox[0].UnreadCount)

	count, err := readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, false, []int64{group.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Scoped to a different group → sees nothing.
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Gruppe Mond")
	inbox, err = readRepo.ListInboxForStaff(ctx, staffAccount.ID, false, []int64{otherGroup.ID}, false)
	require.NoError(t, err)
	assert.Empty(t, inbox, "a staffer scoped to a group the child is not in must not see the conversation")

	// Supervises no groups at all → the 1=0 guard yields nothing.
	inbox, err = readRepo.ListInboxForStaff(ctx, staffAccount.ID, false, nil, false)
	require.NoError(t, err)
	assert.Empty(t, inbox, "an unscoped staffer must not read the whole tenant inbox")

	count, err = readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, false, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
