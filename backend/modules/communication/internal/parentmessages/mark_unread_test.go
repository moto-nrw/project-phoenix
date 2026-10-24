package messaging_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Integration tests for "Als ungelesen markieren" (#3654): a staff member marks
	// a parent conversation unread for the whole team. They run on the real
	// repositories so the inbox projection, the aggregate badge and the student
	// card are checked together with the write path.

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"

	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// assertTeamUnread checks every staff-facing unread surface for one account:
// the sidebar badge, the inbox row, the "Nur ungelesen" filter and the card in
// the child profile.
func assertTeamUnread(t *testing.T, f *fixture, accountID int64, expected int) {
	t.Helper()
	ctx := adminCtx(t, accountID)

	count, err := testpkg.TenantQuery0(t, ctx, f.db, f.svc.UnreadMessageCount)
	require.NoError(t, err)
	assert.Equal(t, expected, count, "badge for account %d", accountID)

	inbox, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListInbox, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, expected, inbox[0].UnreadCount, "inbox row for account %d", accountID)

	onlyUnread, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListInbox, true)
	require.NoError(t, err)
	if expected == 0 {
		assert.Empty(t, onlyUnread, "unread filter for account %d", accountID)
	} else {
		require.Len(t, onlyUnread, 1, "unread filter for account %d", accountID)
		assert.Equal(t, expected, onlyUnread[0].UnreadCount)
	}

	threads, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListStudentThreads, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, expected, threads[0].UnreadCount, "student card for account %d", accountID)
}

// startReadThread opens a conversation with one guardian question that the
// fixture's staff account (A) has read.
func startReadThread(t *testing.T, f *fixture) int64 {
	t.Helper()
	started, err := testpkg.TenantQuery3(t, adminCtx(t, f.staffAccount), f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage")
	_, err = testpkg.TenantQuery1(t, adminCtx(t, f.staffAccount), f.db, f.svc.GetThread, started.ThreadID)
	require.NoError(t, err)
	return started.ThreadID
}

func TestMarkUnread_IsTeamWideUntilOpened(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)

	assertTeamUnread(t, f, f.staffAccount, 0)
	assertTeamUnread(t, f, colleague.ID, 1)

	f.bc.Reset()
	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))
	calls := f.bc.CallsByMethod("parent")
	require.Len(t, calls, 1, "marking wakes the staff tabs")
	assert.Equal(t, realtime.EventParentMessageUnreadChanged, calls[0].Event.Type,
		"its own event type, so an open conversation does not reload and end the mark")
	assert.Zero(t, calls[0].GuardianID, "the guardian is not woken: parents see no change")

	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 1)

	f.bc.Reset()
	_, err := testpkg.TenantQuery1(t, adminCtx(t, colleague.ID), f.db, f.svc.GetThread, threadID)
	require.NoError(t, err)
	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessageUnreadChanged), "a cleared mark wakes the other staff tabs")
	assertTeamUnread(t, f, f.staffAccount, 0)
	assertTeamUnread(t, f, colleague.ID, 0)

	f.bc.Reset()
	_, err = testpkg.TenantQuery1(t, adminCtx(t, colleague.ID), f.db, f.svc.GetThread, threadID)
	require.NoError(t, err)
	assert.Zero(t, parentEventCount(f.bc, realtime.EventParentMessageUnreadChanged), "opening an unmarked thread broadcasts no mark change")
}

func TestMarkUnread_ClearedByReply(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))

	detail, err := testpkg.TenantQuery1(t, adminCtx(t, colleague.ID), f.db, f.svc.GetThread, threadID)
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, colleague.ID), f.db, f.svc.MarkUnread, threadID))
	assertTeamUnread(t, f, f.staffAccount, 1)

	_, err = testpkg.TenantQuery3(t, adminCtx(t, colleague.ID), f.db, f.svc.PostMessage, threadID, "Antwort", detail.Messages[len(detail.Messages)-1].ID)
	require.NoError(t, err)
	assertTeamUnread(t, f, f.staffAccount, 0)
	assertTeamUnread(t, f, colleague.ID, 0)
}

func TestMarkUnread_AfterTeamReplyStillUnreadForEveryone(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	detail, err := testpkg.TenantQuery1(t, adminCtx(t, f.staffAccount), f.db, f.svc.GetThread, threadID)
	require.NoError(t, err)
	_, err = testpkg.TenantQuery3(t, adminCtx(t, f.staffAccount), f.db, f.svc.PostMessage, threadID, "Antwort", detail.Messages[len(detail.Messages)-1].ID)
	require.NoError(t, err)
	assertTeamUnread(t, f, colleague.ID, 0)

	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 1)
}

func TestMarkUnread_NewGuardianMessageCountsRealNumber(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))

	createGuardianMessage(t, f.db, f.chain, threadID, "Noch eine Frage")
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 2)
}

// TestMarkUnread_RepeatKeepsSameVisibleState: marking twice leaves every
// unread number exactly as after the first mark. The stored mark is renewed,
// never moved backward.
func TestMarkUnread_RepeatKeepsSameVisibleState(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, f.db))
	ctx := adminCtx(t, f.staffAccount)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, threadID))
	first, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), messaging.NewThreadRepository(repos.ParentMessageThread).FindByID, threadID)
	require.NoError(t, err)
	require.NotNil(t, first.StaffMarkedUnreadAt)
	require.NotNil(t, first.StaffMarkedUnreadByAccountID)
	assert.Equal(t, f.staffAccount, *first.StaffMarkedUnreadByAccountID)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, threadID))
	second, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), messaging.NewThreadRepository(repos.ParentMessageThread).FindByID, threadID)
	require.NoError(t, err)
	require.NotNil(t, second.StaffMarkedUnreadAt)
	assert.False(t, second.StaffMarkedUnreadAt.Before(*first.StaffMarkedUnreadAt), "a repeated mark never moves backward")
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 1)
}

// TestMarkUnread_RepeatDuringOpenSurvives: B loads a marked thread, C marks it
// again, then B's open clears what it loaded. C marked after B opened, so the
// thread must stay marked ("Eine Markierung, die nach dem Öffnen gesetzt
// wurde, bleibt bestehen").
func TestMarkUnread_RepeatDuringOpenSurvives(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, f.db))
	ctx := adminCtx(t, f.staffAccount)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, threadID))
	loadedByOpen, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), messaging.NewThreadRepository(repos.ParentMessageThread).FindByID, threadID)
	require.NoError(t, err)
	require.NotNil(t, loadedByOpen.StaffMarkedUnreadAt)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, threadID))

	cleared, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), messaging.NewReadRepository(repos.ParentMessageRead).ClearStaffUnreadMark, loadedByOpen.TenantID, threadID, *loadedByOpen.StaffMarkedUnreadAt)
	require.NoError(t, err)
	assert.False(t, cleared, "the open only removes the mark it loaded")
	assertTeamUnread(t, f, f.staffAccount, 1)
}

// TestClearStaffUnreadMark_KeepsNewerMark pins the compare-and-swap: an opening
// request only removes the mark it loaded. A mark set after that load (a later
// timestamp) survives the open.
func TestClearStaffUnreadMark_KeepsNewerMark(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, f.db))
	ctx := adminCtx(t, f.staffAccount)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, threadID))
	marked, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), messaging.NewThreadRepository(repos.ParentMessageThread).FindByID, threadID)
	require.NoError(t, err)
	require.NotNil(t, marked.StaffMarkedUnreadAt)

	loadedBeforeMark := marked.StaffMarkedUnreadAt.Add(-time.Millisecond)
	cleared, err := testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), messaging.NewReadRepository(repos.ParentMessageRead).ClearStaffUnreadMark, marked.TenantID, threadID, loadedBeforeMark)
	require.NoError(t, err)
	assert.False(t, cleared, "a mark newer than the loaded state must survive")
	assertTeamUnread(t, f, f.staffAccount, 1)

	cleared, err = testpkg.TenantQuery3(t, ctx, testpkg.SetupTestDB(t), messaging.NewReadRepository(repos.ParentMessageRead).ClearStaffUnreadMark, marked.TenantID, threadID, *marked.StaffMarkedUnreadAt)
	require.NoError(t, err)
	assert.True(t, cleared)
	assertTeamUnread(t, f, f.staffAccount, 0)
}

// TestMarkUnread_KeepsParentReadReceipt: marking changes nothing for parents.
// "Von der OGS gelesen" on the message and on the parent's thread list stays.
func TestMarkUnread_KeepsParentReadReceipt(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, f.db))
	staffCtx := adminCtx(t, f.staffAccount)
	guardianCtx := adminCtx(t, f.chain.AccountID)
	started, err := testpkg.TenantQuery3(t, staffCtx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	threadID := started.ThreadID
	// Append the guardian question the way the parent side does, so the thread
	// preview (last sender) carries it and the list receipt can apply.
	question := createGuardianMessage(t, f.db, f.chain, threadID, "Frage")
	require.NoError(t, testpkg.TenantCommand5(t, staffCtx, testpkg.SetupTestDB(t), messaging.NewThreadRepository(repos.ParentMessageThread).TouchLastMessage, threadID, question.CreatedAt, question.ID, question.SenderKind, question.Body))
	_, err = testpkg.TenantQuery1(t, staffCtx, f.db, f.svc.GetThread, threadID)
	require.NoError(t, err)

	receipts := func() (bool, bool) {
		t.Helper()
		messages, err := testpkg.TenantQuery2(t, guardianCtx, testpkg.SetupTestDB(t), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(f.db)).ListByThread, threadID, 0)
		require.NoError(t, err)
		require.NoError(t, testpkg.WithinTenantContext(t, guardianCtx, f.db, f.chain.TenantID, func(txCtx context.Context) error {
			messaging.DecorateReadReceipts(txCtx, messaging.NewReadRepository(repos.ParentMessageRead), nil, threadID, f.chain.AccountID, messages)
			return nil
		}))
		var guardianMessage *messaging.ParentMessage
		for _, message := range messages {
			if message.SenderKind == messaging.ParentMessageSenderGuardian {
				guardianMessage = message
			}
		}
		require.NotNil(t, guardianMessage)
		threads, err := testpkg.TenantQuery2(t, guardianCtx, f.db, repos.ParentMessageRead.PortalListThreadsForGuardianStudent, f.chain.AccountID, f.chain.StudentID)
		require.NoError(t, err)
		require.Len(t, threads, 1)
		return guardianMessage.ReadByStaff, threads[0].LastMessageReadByStaff
	}

	readByStaff, lastReadByStaff := receipts()
	require.True(t, readByStaff)
	require.True(t, lastReadByStaff)

	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))
	readByStaff, lastReadByStaff = receipts()
	assert.True(t, readByStaff, "read_by_staff must survive the mark")
	assert.True(t, lastReadByStaff, "last_message_read_by_staff must survive the mark")
}

func TestMarkUnread_Authorization(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)

	outsider := testpkg.CreateTestAccount(t, f.db, "outsider-mark")
	err := testpkg.TenantCommand1(t, claimsCtx(t, outsider.ID, []string{"users:read"}), f.db, f.svc.MarkUnread, threadID)
	require.ErrorIs(t, err, messaging.ErrForbidden)

	err = testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, 999999999)
	require.ErrorIs(t, err, messaging.ErrThreadNotFound)

	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL
	`, "alumnus", f.chain.StudentID)
	require.NoError(t, err)
	err = testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID)
	require.ErrorIs(t, err, messaging.ErrForbidden, "a former child is forbidden")
}

func TestMarkUnread_OtherSchoolIsNotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)

	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		err := testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID)
		require.ErrorIs(t, err, messaging.ErrThreadNotFound)
	})
}

func TestMarkUnread_DisabledSchoolShowsNoUnread(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	require.NoError(t, testpkg.TenantCommand1(t, adminCtx(t, f.staffAccount), f.db, f.svc.MarkUnread, threadID))

	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, f.db))
	reposAccounts := api.NewAccountSuiteRepositories(testpkg.NewDatabase(t, f.db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, f.db))
	disabled := messaging.NewService(messaging.Config{
		UserContext: owners.StaffStudentReads{},
		Session:     api.CommunicationSession{},
		ThreadRepo:  messaging.NewThreadRepository(reposMessages.ParentMessageThread), MessageRepo: messaging.NewMessageRepository(parentStore.NewParentMessageRepository(f.db)), ReadRepo: messaging.NewReadRepository(reposMessages.ParentMessageRead),
		Persons:     newPersons(reposPeople, reposAccounts, f.db),
		Settings:    settingsValues.NewParentMessagingValues(stubSettings{messagingEnabled: false}),
		Broadcaster: f.bc, Logger: slog.Default(),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	ctx := adminCtx(t, f.staffAccount)

	count, err := testpkg.TenantQuery0(t, ctx, testpkg.SetupTestDB(t), disabled.UnreadMessageCount)
	require.NoError(t, err)
	assert.Zero(t, count)
	inbox, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), disabled.ListInbox, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Zero(t, inbox[0].UnreadCount)
	onlyUnread, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), disabled.ListInbox, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread)
}
