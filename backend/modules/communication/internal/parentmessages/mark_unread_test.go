package messaging_test

// Integration tests for "Als ungelesen markieren" (#3654): a staff member marks
// a parent conversation unread for the whole team. They run on the real
// repositories so the inbox projection, the aggregate badge and the student
// card are checked together with the write path.

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// assertTeamUnread checks every staff-facing unread surface for one account:
// the sidebar badge, the inbox row, the "Nur ungelesen" filter and the card in
// the child profile.
func assertTeamUnread(t *testing.T, f *fixture, accountID int64, expected int) {
	t.Helper()
	ctx := adminCtx(t, accountID)

	count, err := f.svc.UnreadMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, expected, count, "badge for account %d", accountID)

	inbox, err := f.svc.ListInbox(ctx, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, expected, inbox[0].UnreadCount, "inbox row for account %d", accountID)

	onlyUnread, err := f.svc.ListInbox(ctx, true)
	require.NoError(t, err)
	if expected == 0 {
		assert.Empty(t, onlyUnread, "unread filter for account %d", accountID)
	} else {
		require.Len(t, onlyUnread, 1, "unread filter for account %d", accountID)
		assert.Equal(t, expected, onlyUnread[0].UnreadCount)
	}

	threads, err := f.svc.ListStudentThreads(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, expected, threads[0].UnreadCount, "student card for account %d", accountID)
}

// startReadThread opens a conversation with one guardian question that the
// fixture's staff account (A) has read.
func startReadThread(t *testing.T, f *fixture) int64 {
	t.Helper()
	started, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage")
	_, err = f.svc.GetThread(adminCtx(t, f.staffAccount), started.ThreadID)
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
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))
	calls := f.bc.CallsByMethod("parent")
	require.Len(t, calls, 1, "marking wakes the staff tabs")
	assert.Equal(t, realtime.EventParentMessage, calls[0].Event.Type)
	assert.Zero(t, calls[0].GuardianID, "the guardian is not woken: parents see no change")

	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 1)

	f.bc.Reset()
	_, err := f.svc.GetThread(adminCtx(t, colleague.ID), threadID)
	require.NoError(t, err)
	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessage), "a cleared mark wakes the other staff tabs")
	assertTeamUnread(t, f, f.staffAccount, 0)
	assertTeamUnread(t, f, colleague.ID, 0)

	f.bc.Reset()
	_, err = f.svc.GetThread(adminCtx(t, colleague.ID), threadID)
	require.NoError(t, err)
	assert.Zero(t, parentEventCount(f.bc, realtime.EventParentMessage), "opening an unmarked thread broadcasts nothing new")
}

func TestMarkUnread_ClearedByReply(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))

	detail, err := f.svc.GetThread(adminCtx(t, colleague.ID), threadID)
	require.NoError(t, err)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), threadID))
	assertTeamUnread(t, f, f.staffAccount, 1)

	_, err = f.svc.PostMessage(adminCtx(t, colleague.ID), threadID, "Antwort", detail.Messages[len(detail.Messages)-1].ID)
	require.NoError(t, err)
	assertTeamUnread(t, f, f.staffAccount, 0)
	assertTeamUnread(t, f, colleague.ID, 0)
}

func TestMarkUnread_AfterTeamReplyStillUnreadForEveryone(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	detail, err := f.svc.GetThread(adminCtx(t, f.staffAccount), threadID)
	require.NoError(t, err)
	_, err = f.svc.PostMessage(adminCtx(t, f.staffAccount), threadID, "Antwort", detail.Messages[len(detail.Messages)-1].ID)
	require.NoError(t, err)
	assertTeamUnread(t, f, colleague.ID, 0)

	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 1)
}

func TestMarkUnread_NewGuardianMessageCountsRealNumber(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))

	createGuardianMessage(t, f.db, f.chain, threadID, "Noch eine Frage")
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 2)
}

func TestMarkUnread_IsIdempotent(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	ctx := adminCtx(t, f.staffAccount)

	require.NoError(t, f.svc.MarkUnread(ctx, threadID))
	first, err := repos.ParentMessageThread.FindByID(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, first.StaffMarkedUnreadAt)
	require.NotNil(t, first.StaffMarkedUnreadByAccountID)
	assert.Equal(t, f.staffAccount, *first.StaffMarkedUnreadByAccountID)

	require.NoError(t, f.svc.MarkUnread(ctx, threadID))
	second, err := repos.ParentMessageThread.FindByID(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, second.StaffMarkedUnreadAt)
	assert.True(t, first.StaffMarkedUnreadAt.Equal(*second.StaffMarkedUnreadAt), "a repeated mark keeps the first one")
	assertTeamUnread(t, f, f.staffAccount, 1)
}

// TestClearStaffUnreadMark_KeepsNewerMark pins the compare-and-swap: an opening
// request only removes the mark it loaded. A mark set after that load (a later
// timestamp) survives the open.
func TestClearStaffUnreadMark_KeepsNewerMark(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	ctx := adminCtx(t, f.staffAccount)

	require.NoError(t, f.svc.MarkUnread(ctx, threadID))
	marked, err := repos.ParentMessageThread.FindByID(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, marked.StaffMarkedUnreadAt)

	loadedBeforeMark := marked.StaffMarkedUnreadAt.Add(-time.Millisecond)
	cleared, err := repos.ParentMessageRead.ClearStaffUnreadMark(ctx, marked.TenantID, threadID, loadedBeforeMark)
	require.NoError(t, err)
	assert.False(t, cleared, "a mark newer than the loaded state must survive")
	assertTeamUnread(t, f, f.staffAccount, 1)

	cleared, err = repos.ParentMessageRead.ClearStaffUnreadMark(ctx, marked.TenantID, threadID, *marked.StaffMarkedUnreadAt)
	require.NoError(t, err)
	assert.True(t, cleared)
	assertTeamUnread(t, f, f.staffAccount, 0)
}

// TestMarkUnread_KeepsParentReadReceipt: marking changes nothing for parents.
// "Von der OGS gelesen" on the message and on the parent's thread list stays.
func TestMarkUnread_KeepsParentReadReceipt(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	staffCtx := adminCtx(t, f.staffAccount)
	guardianCtx := adminCtx(t, f.chain.AccountID)
	started, err := f.svc.StartThread(staffCtx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	threadID := started.ThreadID
	// Append the guardian question the way the parent side does, so the thread
	// preview (last sender) carries it and the list receipt can apply.
	question := createGuardianMessage(t, f.db, f.chain, threadID, "Frage")
	require.NoError(t, repos.ParentMessageThread.TouchLastMessage(staffCtx, threadID, question.CreatedAt, question.ID, question.SenderKind, question.Body))
	_, err = f.svc.GetThread(staffCtx, threadID)
	require.NoError(t, err)

	receipts := func() (bool, bool) {
		t.Helper()
		messages, err := repos.ParentMessage.ListByThread(guardianCtx, threadID, 0)
		require.NoError(t, err)
		messaging.DecorateReadReceipts(guardianCtx, repos.ParentMessageRead, nil, threadID, f.chain.AccountID, messages)
		var guardianMessage *usersModels.ParentMessage
		for _, message := range messages {
			if message.SenderKind == usersModels.ParentMessageSenderGuardian {
				guardianMessage = message
			}
		}
		require.NotNil(t, guardianMessage)
		threads, err := repos.ParentMessageRead.ListThreadsForGuardianStudent(guardianCtx, f.chain.AccountID, f.chain.StudentID)
		require.NoError(t, err)
		require.Len(t, threads, 1)
		return guardianMessage.ReadByStaff, threads[0].LastMessageReadByStaff
	}

	readByStaff, lastReadByStaff := receipts()
	require.True(t, readByStaff)
	require.True(t, lastReadByStaff)

	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))
	readByStaff, lastReadByStaff = receipts()
	assert.True(t, readByStaff, "read_by_staff must survive the mark")
	assert.True(t, lastReadByStaff, "last_message_read_by_staff must survive the mark")
}

func TestMarkUnread_Authorization(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)

	outsider := testpkg.CreateTestAccount(t, f.db, "outsider-mark")
	t.Cleanup(func() {
		_, _ = f.db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, outsider.ID)
	})
	err := f.svc.MarkUnread(claimsCtx(t, outsider.ID, []string{"users:read"}), threadID)
	require.ErrorIs(t, err, messaging.ErrForbidden)

	err = f.svc.MarkUnread(adminCtx(t, f.staffAccount), 999999999)
	require.ErrorIs(t, err, messaging.ErrThreadNotFound)

	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL
	`, usersModels.StudentStatusAlumnus, f.chain.StudentID)
	require.NoError(t, err)
	err = f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID)
	require.ErrorIs(t, err, messaging.ErrForbidden, "a former child is forbidden")
}

func TestMarkUnread_OtherSchoolIsNotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)

	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		err := f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID)
		require.ErrorIs(t, err, messaging.ErrThreadNotFound)
	})
}

func TestMarkUnread_DisabledSchoolShowsNoUnread(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, f.staffAccount), threadID))

	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	disabled := messaging.NewService(messaging.Config{
		ThreadRepo: repos.ParentMessageThread, MessageRepo: repos.ParentMessage, ReadRepo: repos.ParentMessageRead,
		Persons:     newPersons(repos, f.db),
		Settings:    stubSettings{messagingEnabled: false},
		Broadcaster: f.bc, DB: f.db, Logger: slog.Default(),
	})
	ctx := adminCtx(t, f.staffAccount)

	count, err := disabled.UnreadMessageCount(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
	inbox, err := disabled.ListInbox(ctx, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Zero(t, inbox[0].UnreadCount)
	onlyUnread, err := disabled.ListInbox(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread)
}
