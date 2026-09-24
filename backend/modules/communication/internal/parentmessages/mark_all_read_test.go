package messaging_test

// Integration tests for "Alle als gelesen markieren" (#3663): a staff member
// marks every parent conversation they may see as read for their own account.
// They run on the real repositories so the badge, the inbox, the unread filter
// and the child card are checked together with the write path.

import (
	"context"
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

// unreadView is what one account sees across every staff unread surface.
type unreadView struct {
	badge      int
	inbox      map[int64]int
	onlyUnread map[int64]int
	cards      map[int64]int
}

func readUnreadView(t *testing.T, f *fixture, accountID int64, studentIDs ...int64) unreadView {
	t.Helper()
	ctx := adminCtx(t, accountID)
	view := unreadView{inbox: map[int64]int{}, onlyUnread: map[int64]int{}, cards: map[int64]int{}}

	count, err := f.svc.UnreadMessageCount(ctx)
	require.NoError(t, err)
	view.badge = count

	inbox, err := f.svc.ListInbox(ctx, false)
	require.NoError(t, err)
	for _, row := range inbox {
		view.inbox[row.ThreadID] = row.UnreadCount
	}
	onlyUnread, err := f.svc.ListInbox(ctx, true)
	require.NoError(t, err)
	for _, row := range onlyUnread {
		view.onlyUnread[row.ThreadID] = row.UnreadCount
	}
	for _, studentID := range studentIDs {
		threads, err := f.svc.ListStudentThreads(ctx, studentID)
		require.NoError(t, err)
		for _, row := range threads {
			view.cards[row.ThreadID] = row.UnreadCount
		}
	}
	return view
}

// startThreadWithQuestions opens a conversation from staff and adds guardian
// questions nobody has read yet. It returns the thread and its newest question.
func startThreadWithQuestions(t *testing.T, f *fixture, chain testpkg.ParentChain, questions ...string) (int64, *usersModels.ParentMessage) {
	t.Helper()
	started, err := f.svc.StartThread(adminCtx(t, f.staffAccount), chain.StudentID, chain.AccountID, "Hallo")
	require.NoError(t, err)
	var newest *usersModels.ParentMessage
	for _, question := range questions {
		newest = createGuardianMessage(t, f.db, chain, started.ThreadID, question)
	}
	return started.ThreadID, newest
}

// markAllRead runs the action and returns the unread count it reports.
func markAllRead(t *testing.T, f *fixture, ctx context.Context) int {
	t.Helper()
	count, err := f.svc.MarkAllRead(ctx)
	require.NoError(t, err)
	return count
}

type readCursor struct {
	LastReadAt        time.Time `bun:"last_read_at"`
	LastReadMessageID int64     `bun:"last_read_message_id"`
}

// cursorOf returns the account's read cursor in the thread, or nil.
func cursorOf(t *testing.T, threadID, accountID int64) *readCursor {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var rows []readCursor
	require.NoError(t, db.NewSelect().
		TableExpr("users.parent_message_reads").
		Column("last_read_at", "last_read_message_id").
		Where("thread_id = ? AND account_id = ?", threadID, accountID).
		Scan(context.Background(), &rows))
	if len(rows) == 0 {
		return nil
	}
	return &rows[0]
}

type handledBoundary struct {
	At        *time.Time `bun:"staff_handled_up_to_at"`
	MessageID *int64     `bun:"staff_handled_up_to_message_id"`
}

func handledBoundaryOf(t *testing.T, threadID int64) handledBoundary {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var boundary handledBoundary
	require.NoError(t, db.NewSelect().
		TableExpr("users.parent_message_threads").
		Column("staff_handled_up_to_at", "staff_handled_up_to_message_id").
		Where("id = ?", threadID).
		Scan(context.Background(), &boundary))
	return boundary
}

func TestMarkAllRead_OnlyForOwnAccount(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	second := testpkg.CreateTestParentGuardianChain(t, f.db)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	firstThread, firstQuestion := startThreadWithQuestions(t, f, f.chain, "Frage 1", "Frage 2")
	secondThread, secondQuestion := startThreadWithQuestions(t, f, second, "Frage 3")
	students := []int64{f.chain.StudentID, second.StudentID}

	before := readUnreadView(t, f, f.staffAccount, students...)
	require.Equal(t, 3, before.badge)
	colleagueBefore := readUnreadView(t, f, colleague.ID, students...)
	require.Equal(t, 3, colleagueBefore.badge)
	boundaries := map[int64]handledBoundary{
		firstThread:  handledBoundaryOf(t, firstThread),
		secondThread: handledBoundaryOf(t, secondThread),
	}

	f.bc.Reset()
	assert.Zero(t, markAllRead(t, f, adminCtx(t, f.staffAccount)), "reported unread count")

	after := readUnreadView(t, f, f.staffAccount, students...)
	assert.Zero(t, after.badge, "badge")
	assert.Equal(t, map[int64]int{firstThread: 0, secondThread: 0}, after.inbox, "inbox rows")
	assert.Empty(t, after.onlyUnread, "unread filter")
	assert.Equal(t, map[int64]int{firstThread: 0, secondThread: 0}, after.cards, "child cards")

	assert.Equal(t, colleagueBefore, readUnreadView(t, f, colleague.ID, students...), "a colleague sees the same numbers as before")
	for threadID, boundary := range boundaries {
		assert.Equal(t, boundary, handledBoundaryOf(t, threadID), "the team boundary stays where it was")
	}

	firstCursor := cursorOf(t, firstThread, f.staffAccount)
	require.NotNil(t, firstCursor)
	assert.Equal(t, firstQuestion.ID, firstCursor.LastReadMessageID, "the cursor stops at the newest guardian message")
	secondCursor := cursorOf(t, secondThread, f.staffAccount)
	require.NotNil(t, secondCursor)
	assert.Equal(t, secondQuestion.ID, secondCursor.LastReadMessageID)

	assert.Equal(t, 2, parentEventCount(f.bc, realtime.EventParentMessageRead), "each guardian whose message was read gets the receipt update")
}

func TestMarkAllRead_RepeatChangesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage")
	ctx := adminCtx(t, f.staffAccount)

	markAllRead(t, f, ctx)
	first := cursorOf(t, threadID, f.staffAccount)
	require.NotNil(t, first)

	f.bc.Reset()
	markAllRead(t, f, ctx)
	second := cursorOf(t, threadID, f.staffAccount)
	require.NotNil(t, second)
	assert.Equal(t, first.LastReadMessageID, second.LastReadMessageID)
	assert.True(t, first.LastReadAt.Equal(second.LastReadAt))
	assert.Zero(t, parentEventCount(f.bc, realtime.EventParentMessageRead), "nothing new was read, so no receipt update")
}

func TestMarkAllRead_CursorNeverMovesBackward(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID, question := startThreadWithQuestions(t, f, f.chain, "Frage")
	// A cursor ahead of every message the thread holds.
	ahead := question.CreatedAt.Add(time.Hour)
	_, err := f.db.ExecContext(context.Background(), `
		INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id, last_read_at, last_read_message_id)
		VALUES (?, ?, ?, ?, ?)
	`, f.chain.TenantID, threadID, f.staffAccount, ahead, question.ID+1000)
	require.NoError(t, err)

	markAllRead(t, f, adminCtx(t, f.staffAccount))
	cursor := cursorOf(t, threadID, f.staffAccount)
	require.NotNil(t, cursor)
	assert.Equal(t, question.ID+1000, cursor.LastReadMessageID)
	assert.True(t, cursor.LastReadAt.Equal(ahead))
}

func TestMarkAllRead_LaterGuardianMessageIsUnreadAgain(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage")
	markAllRead(t, f, adminCtx(t, f.staffAccount))
	require.Zero(t, readUnreadView(t, f, f.staffAccount).badge)

	createGuardianMessage(t, f.db, f.chain, threadID, "Noch eine Frage")
	view := readUnreadView(t, f, f.staffAccount, f.chain.StudentID)
	assert.Equal(t, 1, view.badge)
	assert.Equal(t, map[int64]int{threadID: 1}, view.onlyUnread)
}

// TestMarkAllRead_LeavesChildrenOutsideScope: a former child's conversation is
// hidden from the inbox, so the action must not read it either. It shows up
// unread again once the child returns.
func TestMarkAllRead_LeavesChildrenOutsideScope(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	former := testpkg.CreateTestParentGuardianChain(t, f.db)
	visibleThread, _ := startThreadWithQuestions(t, f, f.chain, "Frage")
	hiddenThread, _ := startThreadWithQuestions(t, f, former, "Frage")
	setStudentStatus := func(status usersModels.StudentStatus) {
		t.Helper()
		_, err := f.db.ExecContext(context.Background(), `
			UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL
		`, status, former.StudentID)
		require.NoError(t, err)
	}

	setStudentStatus(usersModels.StudentStatusAlumnus)
	markAllRead(t, f, adminCtx(t, f.staffAccount))
	setStudentStatus(usersModels.StudentStatusActive)

	assert.NotNil(t, cursorOf(t, visibleThread, f.staffAccount))
	assert.Nil(t, cursorOf(t, hiddenThread, f.staffAccount), "the hidden child's conversation keeps its cursor")
	view := readUnreadView(t, f, f.staffAccount, former.StudentID)
	assert.Equal(t, map[int64]int{hiddenThread: 1}, view.onlyUnread)
}

// TestMarkAllRead_WithoutReadScopeTouchesNothing: a caller who may not read
// the children sees an empty inbox, so marking all read writes no cursor.
func TestMarkAllRead_WithoutReadScopeTouchesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage")
	outsider := testpkg.CreateTestAccount(t, f.db, "outsider-mark-all")

	markAllRead(t, f, claimsCtx(t, outsider.ID, []string{"users:read"}))
	assert.Nil(t, cursorOf(t, threadID, outsider.ID))
}

func TestMarkAllRead_OtherSchoolUntouched(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage")

	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		markAllRead(t, f, adminCtx(t, f.staffAccount))
	})
	assert.Nil(t, cursorOf(t, threadID, f.staffAccount))
	assert.Equal(t, 1, readUnreadView(t, f, f.staffAccount).badge)
}

// TestMarkAllRead_KeepsTeamMark: a conversation a colleague marked unread for
// the team (#3654) stays marked. Marking all read only moves the own cursor.
func TestMarkAllRead_KeepsTeamMark(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleague := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")
	threadID := startReadThread(t, f)
	require.NoError(t, f.svc.MarkUnread(adminCtx(t, colleague.ID), threadID))
	createGuardianMessage(t, f.db, f.chain, threadID, "Noch eine Frage")

	assert.Equal(t, 1, markAllRead(t, f, adminCtx(t, f.staffAccount)),
		"the reported count keeps the team mark, so the client can explain it")

	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	thread, err := repos.ParentMessageThread.FindByID(adminCtx(t, f.staffAccount), threadID)
	require.NoError(t, err)
	assert.NotNil(t, thread.StaffMarkedUnreadAt, "the team mark stays")
	assertTeamUnread(t, f, f.staffAccount, 1)
	assertTeamUnread(t, f, colleague.ID, 2)
}

// TestMarkAllRead_SetsParentReadReceipt: the parent-facing "Von der OGS
// gelesen" follows the staff cursors, so marking all read sets it. This is the
// accepted consequence of a deliberate action.
func TestMarkAllRead_SetsParentReadReceipt(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	repos := repositories.NewFactory(f.db, repositories.NewUnobservedTimetableDependencies(f.db))
	threadID, _ := startThreadWithQuestions(t, f, f.chain, "Frage")
	guardianCtx := adminCtx(t, f.chain.AccountID)

	readByStaff := func() bool {
		t.Helper()
		messages, err := repos.ParentMessage.ListByThread(guardianCtx, threadID, 0)
		require.NoError(t, err)
		messaging.DecorateReadReceipts(guardianCtx, repos.ParentMessageRead, nil, threadID, f.chain.AccountID, messages)
		return messages[len(messages)-1].ReadByStaff
	}

	require.False(t, readByStaff())
	markAllRead(t, f, adminCtx(t, f.staffAccount))
	assert.True(t, readByStaff())
}
