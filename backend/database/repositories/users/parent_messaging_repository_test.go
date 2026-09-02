package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// tenantCtx returns a context scoped to this test's own tenant (#2419).
func tenantCtx(tb testing.TB) context.Context {
	return testpkg.Ctx(tb)
}

func newThread(tb testing.TB, studentID, guardianAccountID int64) *usersModels.ParentMessageThread {
	t := &usersModels.ParentMessageThread{
		StudentID:         studentID,
		GuardianAccountID: guardianAccountID,
	}
	t.SetTenantID(testpkg.Tenant(tb))
	return t
}

func newMessage(tb testing.TB, threadID, studentID, accountID int64, kind, body string) *usersModels.ParentMessage {
	m := &usersModels.ParentMessage{
		ThreadID:        threadID,
		StudentID:       studentID,
		SenderAccountID: accountID,
		SenderKind:      kind,
		SenderName:      "Test",
		Body:            body,
	}
	m.SetTenantID(testpkg.Tenant(tb))
	return m
}

// TestParentMessaging_ThreadsMessagesAndReadState exercises the full data layer:
// create a thread for a (child, guardian), append guardian + staff messages
// oldest-first, and verify the read-cursor / unread arithmetic that drives both
// the staff inbox and the parent thread list.
func TestParentMessaging_ThreadsMessagesAndReadState(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	// The staff reader is a DISTINCT account from the guardian — in production the
	// inbox reader and the guardian sender are never the same account. Unread is
	// "a message after the cursor the reader did NOT send", so a reader's own
	// messages are excluded by sender_account_id (notReaderAuthored). Reusing one
	// id for both sides would hide that rule and the read-cursor arithmetic it
	// guards.
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	// Messages/reads from staffAccount FK auth.accounts without ON DELETE
	// CASCADE; clear them before the LIFO-earlier account delete above.

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)
	guardian := chain.AccountID

	thread := newThread(t, chain.StudentID, guardian)
	require.NoError(t, threadRepo.Create(ctx, thread))
	require.Positive(t, thread.ID)

	found, err := threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, guardian, found.GuardianAccountID)

	// Two guardian messages (from the guardian) and one staff reply (from the
	// staff reader's own account).
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, guardian, usersModels.ParentMessageSenderGuardian, "hallo")))
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, guardian, usersModels.ParentMessageSenderGuardian, "noch was")))
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, staffAccount.ID, usersModels.ParentMessageSenderStaff, "antwort")))

	messages, err := msgRepo.ListByThread(ctx, thread.ID, 0)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, "hallo", messages[0].Body)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, messages[2].SenderKind)

	// Staff inbox: one row, guardian + child names, unread = 2 guardian msgs.
	inbox, err := readRepo.ListInboxForStaff(ctx, staffAccount.ID, true, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, "Felix Schneider", inbox[0].StudentName)
	assert.Equal(t, "Sabine Schneider", inbox[0].GuardianName)
	assert.Equal(t, "parent", inbox[0].RelationshipType)
	assert.Equal(t, 2, inbox[0].UnreadCount)

	// Sidebar badge counts unread MESSAGES (not threads): the two guardian
	// messages above are both unread to the staff reader, so the badge is 2 —
	// matching the inbox row pill above, not "1 thread".
	unread, err := readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 2, unread)

	// After the staff reader reads up to the newest message, their unread clears.
	// (MarkReadUpTo is the production read path; the old NOW()-cursor MarkRead was
	// removed as dead code.) MarkReadUpTo reports that the cursor advanced.
	newest := messages[len(messages)-1]
	advanced, err := readRepo.MarkReadUpTo(ctx, testpkg.Tenant(t), thread.ID, staffAccount.ID, newest.CreatedAt, newest.ID)
	require.NoError(t, err)
	assert.True(t, advanced, "first read from an empty cursor advances")
	unread, err = readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 0, unread)

	// Re-marking the same newest message does NOT advance (idempotent) — this is
	// what gates the read-receipt SSE push so it can't ping-pong.
	advanced, err = readRepo.MarkReadUpTo(ctx, testpkg.Tenant(t), thread.ID, staffAccount.ID, newest.CreatedAt, newest.ID)
	require.NoError(t, err)
	assert.False(t, advanced, "re-reading the same newest message is a no-op")

	onlyUnread, err := readRepo.ListInboxForStaff(ctx, staffAccount.ID, true, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread)

	// Guardian thread list: unread counts STAFF messages after the GUARDIAN's own
	// cursor. Once the guardian has read up to the newest message, the staff reply
	// is read → 0 unread.
	_, err = readRepo.MarkReadUpTo(ctx, testpkg.Tenant(t), thread.ID, guardian, newest.CreatedAt, newest.ID)
	require.NoError(t, err)
	guardianThreads, err := readRepo.ListThreadsForGuardianStudent(ctx, guardian, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardianThreads, 1)
	assert.Equal(t, 0, guardianThreads[0].UnreadCount)

	// A new staff message makes the guardian list show 1 unread.
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, staffAccount.ID, usersModels.ParentMessageSenderStaff, "noch eine antwort")))
	guardianThreads, err = readRepo.ListThreadsForGuardianStudent(ctx, guardian, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardianThreads, 1)
	assert.Equal(t, 1, guardianThreads[0].UnreadCount)
}

// TestParentMessaging_UnreadCreatedAtTie guards the read-cursor tie-breaker: two
// messages in a thread can share a created_at (clock_timestamp() is microsecond
// precision, so rapid/concurrent sends can tie), and the message list breaks
// those ties by id DESC. A timestamp-only cursor would treat a tied message that
// the reader never saw as already read; the composite (last_read_at,
// last_read_message_id) cursor must keep it unread.
func TestParentMessaging_UnreadCreatedAtTie(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Distinct staff reader (see TestParentMessaging_ThreadsMessagesAndReadState):
	// the guardian messages must be sent by an account OTHER than the reader, or
	// the own-message exclusion (notReaderAuthored) would drop them before the
	// tie-break is exercised.
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	// The staff reader records reads that FK auth.accounts without ON DELETE
	// CASCADE; clear them before the LIFO-earlier account delete above.

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)
	reader := staffAccount.ID

	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	// Two guardian messages forced to the SAME created_at instant; the second
	// gets the higher id (the tie-break the thread list orders by).
	tie := time.Now().Truncate(time.Microsecond)
	m1 := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "erste")
	m1.CreatedAt, m1.UpdatedAt = tie, tie
	require.NoError(t, msgRepo.Create(ctx, m1))
	m2 := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "zweite")
	m2.CreatedAt, m2.UpdatedAt = tie, tie
	require.NoError(t, msgRepo.Create(ctx, m2))
	require.Greater(t, m2.ID, m1.ID, "second insert must get the higher id")

	// Reader read up to m1 (the message it actually saw). m2 committed with the
	// SAME created_at but a higher id, so it must remain unread — a timestamp-only
	// cursor (created_at > last_read_at) would silently drop it.
	advanced, err := readRepo.MarkReadUpTo(ctx, testpkg.Tenant(t), thread.ID, reader, m1.CreatedAt, m1.ID)
	require.NoError(t, err)
	assert.True(t, advanced, "first read advances the cursor")
	count, err := readRepo.UnreadMessageCountForStaff(ctx, reader, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "tied message with a higher id must stay unread")

	// Reading up to m2 (the higher tie-break id) clears it.
	advanced, err = readRepo.MarkReadUpTo(ctx, testpkg.Tenant(t), thread.ID, reader, m2.CreatedAt, m2.ID)
	require.NoError(t, err)
	assert.True(t, advanced, "reading the higher tie-break id advances the cursor")
	count, err = readRepo.UnreadMessageCountForStaff(ctx, reader, true)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestParentMessaging_TeamHandledCursorDoesNotSkipTiedNewMessage pins the
// reply/message race: handling a snapshot up to one guardian row must leave a
// later row with the same timestamp and a higher id unread for every colleague.
func TestParentMessaging_TeamHandledCursorDoesNotSkipTiedNewMessage(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Miriam", "Klein")

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)
	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	tie := time.Now().Truncate(time.Microsecond)
	first := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "erste")
	first.CreatedAt, first.UpdatedAt = tie, tie
	require.NoError(t, msgRepo.Create(ctx, first))
	require.NoError(t, readRepo.MarkStaffHandledUpTo(ctx, testpkg.Tenant(t), thread.ID, first.CreatedAt, first.ID))

	concurrent := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "gleichzeitig")
	concurrent.CreatedAt, concurrent.UpdatedAt = tie, tie
	require.NoError(t, msgRepo.Create(ctx, concurrent))
	require.Greater(t, concurrent.ID, first.ID)

	count, err := readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a tied higher-id message outside the handled snapshot must remain unread")

	require.NoError(t, readRepo.MarkStaffHandledUpTo(ctx, testpkg.Tenant(t), thread.ID, concurrent.CreatedAt, concurrent.ID))
	count, err = readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestParentMessaging_MessageAppendLockSerializesThreadWrites(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentMessageThreadRepository(db)
	ctx := tenantCtx(t)
	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, repo.Create(ctx, thread))

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	holderCtx := tenant.WithTransactionForTest(ctx, &holder)
	require.NoError(t, repo.LockForMessageAppend(holderCtx, thread.ID))

	contender, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = contender.Rollback() }()
	_, err = contender.ExecContext(ctx, "SET LOCAL lock_timeout = ?", "200ms")
	require.NoError(t, err)
	contenderCtx := tenant.WithTransactionForTest(ctx, &contender)
	err = repo.LockForMessageAppend(contenderCtx, thread.ID)
	require.Error(t, err, "a second append must wait for the first transaction")
	assert.True(t, isLockTimeoutError(err), "expected lock_timeout, got: %v", err)
	require.NoError(t, contender.Rollback())

	require.NoError(t, holder.Commit())
	followUp, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = followUp.Rollback() }()
	followUpCtx := tenant.WithTransactionForTest(ctx, &followUp)
	require.NoError(t, repo.LockForMessageAppend(followUpCtx, thread.ID))
}

func TestParentMessaging_StaffNotificationClaimDebouncesOneThread(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentMessageThreadRepository(db)
	ctx := tenantCtx(t)
	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, repo.Create(ctx, thread))

	claimed, err := repo.ClaimStaffMessageNotification(ctx, thread.ID, time.Minute)
	require.NoError(t, err)
	assert.True(t, claimed, "the first parent message claims the notification window")

	claimed, err = repo.ClaimStaffMessageNotification(ctx, thread.ID, time.Minute)
	require.NoError(t, err)
	assert.False(t, claimed, "a second message inside the window is suppressed")

	_, err = db.NewUpdate().
		TableExpr(`users.parent_message_threads AS "parent_message_thread"`).
		Set("last_staff_message_notification_at = clock_timestamp() - INTERVAL '61 seconds'").
		Where(`"parent_message_thread".id = ?`, thread.ID).
		Exec(ctx)
	require.NoError(t, err)

	claimed, err = repo.ClaimStaffMessageNotification(ctx, thread.ID, time.Minute)
	require.NoError(t, err)
	assert.True(t, claimed, "a later follow-up starts a new notification window")
}

// newRequestCreatedPill builds a "request created" event pill as the emitter
// stores it: a system event attributed to the GUARDIAN side (event_actor_kind),
// so it counts as unread to a staff reader. actorAccountID is the submitting
// guardian's account (the emitter stamps the actor in sender_account_id).
func newRequestCreatedPill(tb testing.TB, threadID, studentID, actorAccountID int64, at time.Time) *usersModels.ParentMessage {
	m := &usersModels.ParentMessage{
		ThreadID:        threadID,
		StudentID:       studentID,
		SenderAccountID: actorAccountID,
		SenderKind:      usersModels.ParentMessageSenderSystem,
		SenderName:      "System",
		Body:            "Anfrage gestellt",
		Kind:            usersModels.ParentMessageKindEvent,
		EventType:       "request_created",
		EventActorKind:  usersModels.ParentMessageSenderGuardian,
	}
	m.CreatedAt, m.UpdatedAt = at, at
	m.SetTenantID(testpkg.Tenant(tb))
	return m
}

// TestParentMessaging_RequestCreatedPillNotCounted pins the #1803 duplicate-signal
// fix: a request_created pill is a queue notice surfaced on the Änderungsanfragen
// badge, not an unread chat message, so it must NOT inflate the staff Nachrichten
// unread count — while a plain guardian message in the SAME thread still does. It
// also asserts the per-thread unread_count column agrees with the aggregate badge,
// the invariant the exclusion is baked into counterpartUnread to preserve.
func TestParentMessaging_RequestCreatedPillNotCounted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)

	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	base := time.Now().Truncate(time.Microsecond)
	// A request_created pill (guardian-side system event) alone must leave the
	// staff badge at zero — it belongs on the Änderungsanfragen queue count.
	pill := newRequestCreatedPill(t, thread.ID, chain.StudentID, chain.AccountID, base)
	require.NoError(t, msgRepo.Create(ctx, pill))

	count, err := readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a request_created pill must not count toward the staff Nachrichten badge")

	inbox, err := readRepo.ListInboxForStaff(ctx, staffAccount.ID, true, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1, "the thread is still listed — it has a message")
	assert.Equal(t, 0, inbox[0].UnreadCount, "per-thread unread_count agrees with the badge: pill excluded")

	// A plain guardian chat message in the same thread DOES count — the exclusion
	// is specific to request_created, not a blanket mute of the thread.
	chat := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "Frage")
	chat.CreatedAt, chat.UpdatedAt = base.Add(time.Second), base.Add(time.Second)
	require.NoError(t, msgRepo.Create(ctx, chat))

	count, err = readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a plain guardian message still counts; only the pill is excluded")

	inbox, err = readRepo.ListInboxForStaff(ctx, staffAccount.ID, true, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, 1, inbox[0].UnreadCount, "per-thread count still matches the badge")
}

// TestParentMessaging_OneThreadPerGuardian verifies the chat model: exactly one
// conversation per (child, guardian). A second create for the same pair is
// rejected by the unique constraint, and FindByStudentGuardian returns the
// existing thread so the "send" path is get-or-create.
func TestParentMessaging_OneThreadPerGuardian(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)

	first := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, first))

	// A second conversation for the same (child, guardian) is rejected.
	err := threadRepo.Create(ctx, newThread(t, chain.StudentID, chain.AccountID))
	require.Error(t, err, "unique constraint must reject a duplicate conversation")

	// get-or-create lookup returns the existing thread.
	found, err := threadRepo.FindByStudentGuardian(ctx, chain.StudentID, chain.AccountID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, first.ID, found.ID)

	// An opened-but-unwritten conversation (created by the "open chat" path)
	// stays hidden from the guardian's list until its first message, so opening
	// a chat never litters the list.
	threads, err := readRepo.ListThreadsForGuardianStudent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, threads, "an empty conversation must stay hidden until the first message")

	// After the first message it appears — still exactly one conversation.
	msgRepo := usersRepo.NewParentMessageRepository(db)
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, first.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderStaff, "hallo")))
	threads, err = readRepo.ListThreadsForGuardianStudent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Len(t, threads, 1, "guardian should have exactly one conversation about the child")
}

// TestParentMessaging_ListGuardiansForStudent returns the child's
// account-holding guardians for the staff recipient picker.
func TestParentMessaging_ListGuardiansForStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	guardians, err := threadRepo.ListGuardiansForStudent(tenantCtx(t), chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	assert.Equal(t, chain.AccountID, guardians[0].AccountID)
	assert.Equal(t, "Sabine Schneider", guardians[0].Name)
	assert.Equal(t, "parent", guardians[0].RelationshipType)
	assert.True(t, guardians[0].IsPrimary)
}

// A pickup-only guardian holds a portal account but no parent_portal.access for
// this child, so the parent-side reads reject that thread. The staff recipient
// picker must therefore NOT offer them — otherwise staff could send a message
// the recipient can never see.
func TestParentMessaging_ListGuardiansForStudent_ExcludesNoPortalAccess(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := tenantCtx(t)
	// Second guardian: account-holding, linked to the SAME child, but pickup_only
	// (no parent_portal.access in the relationship's permissions).
	pickupAccount := testpkg.CreateTestAccount(t, db, "pickup-only")
	pickupProfile := &usersModels.GuardianProfile{
		FirstName:  "Olaf",
		LastName:   "Helfer",
		Email:      &pickupAccount.Email,
		AccountID:  &pickupAccount.ID,
		HasAccount: true,
	}
	pickupProfile.SetTenantID(chain.TenantID)
	_, err := db.NewInsert().Model(pickupProfile).ModelTableExpr(`users.guardian_profiles`).Exec(context.Background())
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.guardian_profiles WHERE id = ?`, pickupProfile.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, pickupAccount.ID)
	}()

	pickupLink := &usersModels.StudentGuardian{
		StudentID:         chain.StudentID,
		GuardianProfileID: pickupProfile.ID,
		RelationshipType:  "other",
		CanPickup:         true,
	}
	authorize.ApplyStudentGuardianRole(pickupLink, authorize.GuardianRolePickupOnly)
	pickupLink.SetTenantID(chain.TenantID)
	_, err = db.NewInsert().Model(pickupLink).ModelTableExpr(`users.students_guardians`).Exec(context.Background())
	require.NoError(t, err)
	// students_guardians for this student is cleaned by CleanupParentGuardianChain.

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	guardians, err := threadRepo.ListGuardiansForStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 1, "pickup-only guardian without parent_portal.access must be excluded")
	assert.Equal(t, chain.AccountID, guardians[0].AccountID)
}

// A guardian who still has parent_portal.access on the relationship but whose
// auth.account_tenants membership for this school is no longer active can no
// longer read the child (the parent-side reads require status = 'active'). The
// staff recipient picker must therefore exclude them — otherwise staff could
// open a thread the recipient can never see and BroadcastParentMessage would
// still wake the revoked account.
func TestParentMessaging_ListGuardiansForStudent_ExcludesInactiveTenantMembership(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := tenantCtx(t)
	threadRepo := usersRepo.NewParentMessageThreadRepository(db)

	// Sanity: with an active membership the primary guardian is offered.
	guardians, err := threadRepo.ListGuardiansForStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)

	// Revoke access by flipping the membership to inactive.
	_, err = db.ExecContext(context.Background(),
		`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`,
		chain.AccountID, chain.TenantID)
	require.NoError(t, err)

	guardians, err = threadRepo.ListGuardiansForStudent(ctx, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, guardians, "guardian without an active account_tenants membership must be excluded")
}

// TestParentMessaging_TouchLastMessage_Monotonic verifies the denormalized
// last-activity fields only ever move FORWARD in time. This is the concurrency
// guard: when a staff reply and a guardian message hit the same thread at nearly
// the same time, each loads the thread and stamps its own instant. The older one
// must NOT overwrite the newer one's preview/order just because it committed
// last. TouchLastMessage no-ops on a stale (older-or-equal) instant.
func TestParentMessaging_TouchLastMessage_Monotonic(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := tenantCtx(t)
	threadRepo := usersRepo.NewParentMessageThreadRepository(db)

	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))
	require.Positive(t, thread.ID)

	t1 := time.Now().Truncate(time.Second)
	t2 := t1.Add(2 * time.Second)

	// First real activity: a guardian message at t2 wins the empty (NULL) thread.
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, t2, 1, usersModels.ParentMessageSenderGuardian, "neuere Nachricht"))
	got, err := threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastMessageAt)
	assert.WithinDuration(t, t2, *got.LastMessageAt, time.Second)
	assert.Equal(t, usersModels.ParentMessageSenderGuardian, derefStr(got.LastSenderKind))
	assert.Equal(t, "neuere Nachricht", got.LastMessageBody)

	// A staff send that committed afterwards but captured an OLDER instant (t1)
	// must be a no-op: preview/order/last-sender stay on the t2 message.
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, t1, 2, usersModels.ParentMessageSenderStaff, "ältere Antwort"))
	got, err = threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, t2, *got.LastMessageAt, time.Second, "stale older send must not move last_message_at back")
	assert.Equal(t, usersModels.ParentMessageSenderGuardian, derefStr(got.LastSenderKind), "stale older send must not steal last sender")
	assert.Equal(t, "neuere Nachricht", got.LastMessageBody, "stale older send must not overwrite the preview")

	// A genuinely newer send (t3 > t2) does advance the thread.
	t3 := t2.Add(2 * time.Second)
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, t3, 3, usersModels.ParentMessageSenderStaff, "neueste Antwort"))
	got, err = threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, t3, *got.LastMessageAt, time.Second)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, derefStr(got.LastSenderKind))
	assert.Equal(t, "neueste Antwort", got.LastMessageBody)
}

// TestParentMessaging_TouchLastMessage_TiedTimestamp pins the id tie-breaker:
// clock_timestamp() can hand two messages in the same thread an identical
// created_at, so the monotonic preview guard must fall back to the message id
// (the second half of the (created_at, id) order ListByThread and the unread
// cursor use). On an equal instant the higher-id message owns the preview/sender,
// and a lower-id message that commits afterwards must NOT steal it back.
func TestParentMessaging_TouchLastMessage_TiedTimestamp(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := tenantCtx(t)
	threadRepo := usersRepo.NewParentMessageThreadRepository(db)

	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))
	require.Positive(t, thread.ID)

	at := time.Now().Truncate(time.Second)

	// Lower-id message lands first and wins the empty thread.
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, at, 10, usersModels.ParentMessageSenderGuardian, "erste gleiche Zeit"))

	// Same instant, HIGHER id: the genuinely newer message must take over.
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, at, 20, usersModels.ParentMessageSenderStaff, "neuere gleiche Zeit"))
	got, err := threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, derefStr(got.LastSenderKind), "higher id at equal instant must win")
	assert.Equal(t, "neuere gleiche Zeit", got.LastMessageBody)

	// Same instant, LOWER id committing afterwards: must be a no-op.
	require.NoError(t, threadRepo.TouchLastMessage(ctx, thread.ID, at, 15, usersModels.ParentMessageSenderGuardian, "ältere gleiche Zeit"))
	got, err = threadRepo.FindByID(ctx, thread.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, derefStr(got.LastSenderKind), "lower id at equal instant must not steal the preview")
	assert.Equal(t, "neuere gleiche Zeit", got.LastMessageBody, "lower id at equal instant must not overwrite the preview")
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TestParentMessaging_UnreadCountExcludesAlumni guards the sidebar badges
// against graduated children: both inboxes hide an alumnus child's threads, so
// the shared unread-count query must not keep counting their messages — a
// nonzero badge no portal can open or clear (#405 review).
func TestParentMessaging_UnreadCountExcludesAlumni(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	threadRepo := usersRepo.NewParentMessageThreadRepository(db)
	msgRepo := usersRepo.NewParentMessageRepository(db)
	readRepo := repositories.NewFactory(db).ParentMessageRead
	ctx := tenantCtx(t)

	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, threadRepo.Create(ctx, thread))

	// One unread message per reader side: the guardian message badges the staff
	// sidebar, the staff message badges the parent portal.
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "hallo")))
	require.NoError(t, msgRepo.Create(ctx, newMessage(t, thread.ID, chain.StudentID, staffAccount.ID, usersModels.ParentMessageSenderStaff, "antwort")))

	staffUnread, err := readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, staffUnread, "guard: the guardian message counts while the child is active")
	guardianUnread, err := readRepo.UnreadMessageCountForGuardianTenants(ctx, chain.AccountID, []int64{testpkg.Tenant(t)})
	require.NoError(t, err)
	require.Equal(t, 1, guardianUnread, "guard: the staff message counts while the child is active")

	// The child graduates (soft delete via Jahrgangswechsel).
	_, err = db.NewRaw(`UPDATE users.students SET status = 'alumnus' WHERE id = ?`, chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	staffUnread, err = readRepo.UnreadMessageCountForStaff(ctx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 0, staffUnread, "a graduated child's messages must not badge the staff sidebar")
	guardianUnread, err = readRepo.UnreadMessageCountForGuardianTenants(ctx, chain.AccountID, []int64{testpkg.Tenant(t)})
	require.NoError(t, err)
	assert.Equal(t, 0, guardianUnread, "a graduated child's messages must not badge the parent portal")
}
