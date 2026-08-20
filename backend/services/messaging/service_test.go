package messaging_test

// Integration tests for the staff-side parent-OGS messaging service. They run
// against the real test DB through the real repositories, the real PersonService
// (for staff-name resolution and the read-access student load), the shared
// parentmessaging core, and a capturing broadcaster, so the inbox / thread /
// reply / new-conversation flows and their authorization and feature-flag gates
// are exercised end to end exactly as the HTTP handlers drive them.
//
// Most reads/writes run on the ADMIN read path (perms "admin:*"), which is how a
// school admin sees every child's inbox; the forbidden test drops to a
// permissionless caller to pin the CanReadStudent gate.

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- test doubles -------------------------------------------------------

// stubSettings answers only the lookups the messaging service makes: the
// parent-messaging feature flag and the staff-name visibility flag. Every other
// SettingsService method is inherited from the embedded nil interface and would
// panic if unexpectedly called — student read access no longer consults any
// setting (#2329), it follows from the caller's staff record alone.
type stubSettings struct {
	configService.SettingsService
	messagingEnabled bool
	// staffNameVisible answers operations.parent_message_staff_name_visible, the
	// flag the send path freezes onto each staff message. Defaults false so
	// existing tests exercise the masked ("OGS") path unchanged.
	staffNameVisible bool
}

func (s stubSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	switch key {
	case configModels.KeyParentNotesEnabled:
		return s.messagingEnabled, nil
	case configModels.KeyParentMessageStaffNameVisible:
		return s.staffNameVisible, nil
	default:
		return false, nil
	}
}

func (s stubSettings) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s stubSettings) ResolveString(_ context.Context, _ string) (string, error) {
	return "", nil
}

// parentEventCount counts recorded BroadcastParentMessage calls (the only
// broadcast method the messaging service ever calls) whose event carries the
// given type — the shared RecordingBroadcaster equivalent of the old
// captureBroadcaster.countOf.
func parentEventCount(bc *testpkg.RecordingBroadcaster, t realtime.EventType) int {
	n := 0
	for _, c := range bc.CallsByMethod("parent") {
		if c.Event.Type == t {
			n++
		}
	}
	return n
}

// --- harness ------------------------------------------------------------

type fixture struct {
	db           *bun.DB
	svc          *messaging.Service
	bc           *testpkg.RecordingBroadcaster
	chain        testpkg.ParentChain
	staffAccount int64
}

func newPersons(repos *repositories.Factory, db *bun.DB) usersService.PersonService {
	return usersService.NewPersonService(usersService.PersonServiceDependencies{
		PersonRepo:  repos.Person,
		AccountRepo: repos.Account,
		StudentRepo: repos.Student,
		StaffRepo:   repos.Staff,
		DB:          db,
		Logger:      slog.Default(),
	})
}

func buildMessagingWithSettings(t *testing.T, settings stubSettings) (*messaging.Service, *testpkg.RecordingBroadcaster, *repositories.Factory, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	svc := messaging.NewService(messaging.Config{
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		ReadRepo:    repos.ParentMessageRead,
		Persons:     newPersons(repos, db),
		Settings:    settings,
		Broadcaster: bc,
		DB:          db,
		Logger:      slog.Default(),
	})
	return svc, bc, repos, db
}

// adminCtx returns a context in this test's tenant carrying admin permissions
// and the given staff account id (the inbox reader / message sender).
func adminCtx(tb testing.TB, accountID int64) context.Context {
	return claimsCtx(tb, accountID, []string{"admin:*"})
}

func claimsCtx(tb testing.TB, accountID int64, perms []string) context.Context {
	ctx := testpkg.Ctx(tb)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
	return ctx
}

// newFixture wires the full messaging stack plus a parent→child chain and a
// distinct staff reader account, with cleanup registered.
func newFixture(t *testing.T, enabled bool) *fixture {
	return newFixtureWithSettings(t, stubSettings{messagingEnabled: enabled})
}

func newFixtureWithSettings(t *testing.T, settings stubSettings) *fixture {
	t.Helper()
	svc, bc, _, db := buildMessagingWithSettings(t, settings)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	return &fixture{db: db, svc: svc, bc: bc, chain: chain, staffAccount: staffAccount.ID}
}

// createGuardianMessage inserts a guardian-authored message straight through the
// repo, simulating the parent side so a staff-facing unread exists.
func createGuardianMessage(t *testing.T, db *bun.DB, chain testpkg.ParentChain, threadID int64, body string) *usersModels.ParentMessage {
	t.Helper()
	gm := &usersModels.ParentMessage{
		ThreadID:        threadID,
		StudentID:       chain.StudentID,
		SenderAccountID: chain.AccountID,
		SenderKind:      usersModels.ParentMessageSenderGuardian,
		SenderName:      "Sabine Schneider",
		Body:            body,
	}
	gm.SetTenantID(chain.TenantID)
	require.NoError(t, repositories.NewFactory(db).ParentMessage.Create(testpkg.Ctx(t), gm))
	return gm
}

// --- StartThread / PostMessage / GetThread ------------------------------

// TestStartThread_CreatesConversationAndBroadcasts is the staff "Neue Nachricht"
// happy path: the OGS opens a conversation with a guardian, the message persists,
// the chat detail carries the resolved header (child + guardian + relationship)
// and the staff sender name, and a new-message SSE fan-out fires to the guardian.
func TestStartThread_CreatesConversationAndBroadcasts(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)

	detail, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Guten Tag, bitte um Rückruf")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Positive(t, detail.ThreadID)
	assert.Equal(t, f.chain.StudentID, detail.StudentID)
	assert.Equal(t, f.chain.AccountID, detail.GuardianAccountID)
	assert.Equal(t, "Felix Schneider", detail.StudentName)
	assert.Equal(t, "Sabine Schneider", detail.GuardianName)
	assert.Equal(t, "parent", detail.RelationshipType)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, detail.Messages[0].SenderKind)
	assert.Equal(t, "Olivia Berg", detail.Messages[0].SenderName, "staff display name is denormalized onto the message")
	assert.Equal(t, "Guten Tag, bitte um Rückruf", detail.Messages[0].Body)

	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessage), "a staff send wakes the guardian with a new-message event")
}

// TestStartThread_StampsStaffNameVisibleFromSetting proves the send path FREEZES
// the operations.parent_message_staff_name_visible setting onto each staff
// message at write time (users.parent_messages.staff_name_visible). That frozen
// column — not a live re-read — is what the parent view later uses, so a message
// keeps the visibility it was sent with even if the school toggles the setting.
func TestStartThread_StampsStaffNameVisibleFromSetting(t *testing.T) {
	t.Parallel()

	t.Run("setting on -> stamped visible", func(t *testing.T) {
		f := newFixtureWithSettings(t, stubSettings{messagingEnabled: true, staffNameVisible: true})
		detail, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
		require.NoError(t, err)
		require.Len(t, detail.Messages, 1)
		assert.True(t, detail.Messages[0].StaffNameVisible, "reply sent while the setting is on must be stamped visible")
	})

	t.Run("setting off -> stamped hidden", func(t *testing.T) {
		f := newFixtureWithSettings(t, stubSettings{messagingEnabled: true, staffNameVisible: false})
		detail, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
		require.NoError(t, err)
		require.Len(t, detail.Messages, 1)
		assert.False(t, detail.Messages[0].StaffNameVisible, "reply sent while the setting is off stays anonymous")
	})
}

// TestStartThread_IsGetOrCreate verifies the chat model: a second StartThread for
// the same (child, guardian) appends to the existing conversation instead of
// opening a duplicate.
func TestStartThread_IsGetOrCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)

	first, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Erste")
	require.NoError(t, err)
	second, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Zweite")
	require.NoError(t, err)
	assert.Equal(t, first.ThreadID, second.ThreadID, "same pair must resolve to one conversation")
	require.Len(t, second.Messages, 2)
}

// TestPostMessage_AppendsAndBroadcasts replies into an existing thread and checks
// the message is appended in chat order and a fan-out fires.
func TestPostMessage_AppendsAndBroadcasts(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)

	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	f.bc.Reset() // reset to isolate the reply's broadcast

	msgs, err := f.svc.PostMessage(ctx, started.ThreadID, "Noch eine Info", 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "Noch eine Info", msgs[1].Body)
	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessage))
}

// TestPostMessage_ClearsTeamUnreadForColleagues covers the shared-inbox contract:
// a reply handles the guardian activity for every authorized staff member without
// forging a personal read cursor for colleagues. Later guardian activity becomes
// unread for the whole team again.
func TestPostMessage_ClearsTeamUnreadForColleagues(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, colleagueAccount := testpkg.CreateTestStaffWithAccount(t, f.db, "Miriam", "Klein")

	started, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	firstGuardianMessage := createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Erste Frage")

	assertStaffUnread := func(accountID int64, expected int) {
		t.Helper()
		count, countErr := f.svc.UnreadMessageCount(adminCtx(t, accountID))
		require.NoError(t, countErr)
		assert.Equal(t, expected, count)

		inbox, inboxErr := f.svc.ListInbox(adminCtx(t, accountID), true)
		require.NoError(t, inboxErr)
		if expected == 0 {
			assert.Empty(t, inbox)
		} else {
			require.Len(t, inbox, 1)
			assert.Equal(t, expected, inbox[0].UnreadCount)
		}

		threads, threadsErr := f.svc.ListStudentThreads(adminCtx(t, accountID), f.chain.StudentID)
		require.NoError(t, threadsErr)
		require.Len(t, threads, 1)
		assert.Equal(t, expected, threads[0].UnreadCount)
	}

	assertStaffUnread(f.staffAccount, 1)
	assertStaffUnread(colleagueAccount.ID, 1)

	_, err = f.svc.PostMessage(adminCtx(t, f.staffAccount), started.ThreadID, "Team-Antwort", firstGuardianMessage.ID)
	require.NoError(t, err)
	assertStaffUnread(f.staffAccount, 0)
	assertStaffUnread(colleagueAccount.ID, 0)

	var colleagueReadRows int
	require.NoError(t, f.db.NewRaw(`
		SELECT COUNT(*) FROM users.parent_message_reads
		WHERE thread_id = ? AND account_id = ?
	`, started.ThreadID, colleagueAccount.ID).Scan(context.Background(), &colleagueReadRows))
	assert.Zero(t, colleagueReadRows, "team handling must not forge a colleague's personal read receipt")

	allMessages, err := repositories.NewFactory(f.db).ParentMessage.ListByThread(adminCtx(t, f.staffAccount), started.ThreadID, 0)
	require.NoError(t, err)
	parentmessaging.DecorateReadReceipts(adminCtx(t, f.chain.AccountID), repositories.NewFactory(f.db).ParentMessageRead, nil, started.ThreadID, f.chain.AccountID, allMessages)
	assert.True(t, allMessages[1].ReadByStaff, "the guardian receipt remains tied to the responding staff account's personal cursor")

	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Neue Frage")
	assertStaffUnread(f.staffAccount, 1)
	assertStaffUnread(colleagueAccount.ID, 1)

	readRepo := repositories.NewFactory(f.db).ParentMessageRead
	count, err := readRepo.UnreadMessageCountForStaff(adminCtx(t, colleagueAccount.ID), colleagueAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "authorized staff must see the new open message")
	count, err = readRepo.UnreadMessageCountForStaff(adminCtx(t, colleagueAccount.ID), colleagueAccount.ID, false)
	require.NoError(t, err)
	assert.Zero(t, count, "the team boundary must not broaden student access beyond the caller's own scope")
}

func TestPostMessage_RejectsLegacyReplyWithoutVisibleBoundary(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	started, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage")

	_, err = f.svc.PostMessage(adminCtx(t, f.staffAccount), started.ThreadID, "Antwort", 0)
	require.ErrorIs(t, err, messaging.ErrHandledBoundaryRequired)

	messages, err := repositories.NewFactory(f.db).ParentMessage.ListByThread(adminCtx(t, f.staffAccount), started.ThreadID, 0)
	require.NoError(t, err)
	assert.Len(t, messages, 2, "the rejected legacy reply must not be inserted")

	_, err = f.svc.PostMessage(adminCtx(t, f.staffAccount), started.ThreadID, "Antwort", 999999999)
	require.ErrorIs(t, err, messaging.ErrHandledBoundaryRequired)
	messages, err = repositories.NewFactory(f.db).ParentMessage.ListByThread(adminCtx(t, f.staffAccount), started.ThreadID, 0)
	require.NoError(t, err)
	assert.Len(t, messages, 2, "a reply with a foreign boundary must not be inserted")
}

// TestPostMessage_EmptyAndTooLong pins the body guards: blank → ErrEmptyBody,
// over the 2000-rune cap → ErrBodyTooLong, both BEFORE any insert.
func TestPostMessage_EmptyAndTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)

	_, err = f.svc.PostMessage(ctx, started.ThreadID, "   \n\t ", 0)
	require.ErrorIs(t, err, messaging.ErrEmptyBody)

	_, err = f.svc.PostMessage(ctx, started.ThreadID, strings.Repeat("x", 2001), 0)
	require.ErrorIs(t, err, messaging.ErrBodyTooLong)
}

// TestGetThread_MarksReadAndBroadcastsReceipt: a guardian message is unread to
// staff until GetThread reads it; opening the thread advances the staff cursor
// (clearing the unread badge) and fires a read-receipt SSE so the guardian's open
// chat shows "Gelesen" live. A second GetThread that marks nothing must NOT fire
// another receipt (the anti-ping-pong gate).
func TestGetThread_MarksReadAndBroadcastsReceipt(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	staffCtx := adminCtx(t, f.staffAccount)

	started, err := f.svc.StartThread(staffCtx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Danke!")

	before, err := f.svc.UnreadMessageCount(staffCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, before, "guardian reply is unread to staff before opening")

	f.bc.Reset()
	detail, err := f.svc.GetThread(staffCtx, started.ThreadID)
	require.NoError(t, err)
	require.Len(t, detail.Messages, 2)

	after, err := f.svc.UnreadMessageCount(staffCtx)
	require.NoError(t, err)
	assert.Equal(t, 0, after, "GetThread advances the staff read cursor")
	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessageRead), "advancing the cursor wakes the guardian's receipts")

	// Re-open: nothing new to read, so no second receipt event.
	f.bc.Reset()
	_, err = f.svc.GetThread(staffCtx, started.ThreadID)
	require.NoError(t, err)
	assert.Equal(t, 0, parentEventCount(f.bc, realtime.EventParentMessageRead), "a no-op read must not ping-pong a receipt event")
}

// TestGetThread_NotFound: a thread id that does not exist in the tenant is
// ErrThreadNotFound, not a 500.
func TestGetThread_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	_, err := f.svc.GetThread(adminCtx(t, f.staffAccount), 999999999)
	require.ErrorIs(t, err, messaging.ErrThreadNotFound)
}

// --- OpenThread ---------------------------------------------------------

// TestOpenThread_EmptyConversationStaysHiddenFromInbox: opening a chat
// get-or-creates the thread but sends no message, and an empty conversation must
// not appear in the inbox until its first message — so opening never litters the
// list.
func TestOpenThread_EmptyConversationStaysHiddenFromInbox(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)

	detail, err := f.svc.OpenThread(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assert.Positive(t, detail.ThreadID)
	assert.Empty(t, detail.Messages)

	inbox, err := f.svc.ListInbox(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, inbox, "an opened-but-unwritten conversation must stay out of the inbox")
}

// TestOpenThread_ExistingWithUnreadMarksReadAndBroadcasts: opening an existing
// conversation that has an unread guardian message reads it (clearing the badge)
// and fires a read-receipt SSE so the guardian's open chat updates live.
func TestOpenThread_ExistingWithUnreadMarksReadAndBroadcasts(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Antwort")

	f.bc.Reset()
	detail, err := f.svc.OpenThread(ctx, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, started.ThreadID, detail.ThreadID, "OpenThread resolves the existing conversation")
	require.Len(t, detail.Messages, 2)
	assert.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessageRead), "opening with unread fires a read receipt")

	cnt, err := f.svc.UnreadMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt, "opening the thread cleared the unread badge")
}

// --- ListInbox / ListGuardians / ListStudentThreads ---------------------

// TestListInbox_ShowsConversationWithUnread: after two guardian messages the
// inbox row carries names, relationship, and the unread MESSAGE count; the
// onlyUnread filter keeps it while there is unread and drops it once read.
func TestListInbox_ShowsConversationWithUnread(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)

	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage eins")
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage zwei")

	inbox, err := f.svc.ListInbox(ctx, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, "Felix Schneider", inbox[0].StudentName)
	assert.Equal(t, "Sabine Schneider", inbox[0].GuardianName)
	assert.Equal(t, 2, inbox[0].UnreadCount, "badge counts unread messages, not threads")

	onlyUnread, err := f.svc.ListInbox(ctx, true)
	require.NoError(t, err)
	require.Len(t, onlyUnread, 1)

	// Read it: onlyUnread now drops the row.
	_, err = f.svc.GetThread(ctx, started.ThreadID)
	require.NoError(t, err)
	onlyUnread, err = f.svc.ListInbox(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread, "a fully-read conversation leaves the onlyUnread filter")
}

// TestListGuardians_ReturnsAccountHoldingGuardian: the staff recipient picker
// offers the child's account-holding guardian with their relationship + primary
// flag.
func TestListGuardians_ReturnsAccountHoldingGuardian(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	guardians, err := f.svc.ListGuardians(adminCtx(t, f.staffAccount), f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	assert.Equal(t, f.chain.AccountID, guardians[0].AccountID)
	assert.Equal(t, "Sabine Schneider", guardians[0].Name)
	assert.True(t, guardians[0].IsPrimary)
}

// TestListStudentThreads_FiltersToChild: the student-detail card sees the child's
// conversation(s) after authorizing read access to that child.
func TestListStudentThreads_FiltersToChild(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	_, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)

	rows, err := f.svc.ListStudentThreads(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, f.chain.StudentID, rows[0].StudentID)
}

// --- authorization & feature gates --------------------------------------

// TestStartThread_InvalidGuardian: starting a thread to an account that is NOT a
// messageable guardian of the child is ErrInvalidGuardian — staff cannot message
// an arbitrary account about a child.
func TestStartThread_InvalidGuardian(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	stranger := testpkg.CreateTestAccount(t, f.db, "stranger")
	t.Cleanup(func() {
		_, _ = f.db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, stranger.ID)
	})

	_, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, stranger.ID, "Hallo")
	require.ErrorIs(t, err, messaging.ErrInvalidGuardian)
}

// TestMessaging_Forbidden: a caller with no admin permission and no staff record
// in the tenant (guest, guardian) may not read a child's threads or guardians —
// CanReadStudent denies, and the service maps that to ErrForbidden (→ 403),
// never leaking the conversation. users:read alone is not enough.
func TestMessaging_Forbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	started, err := f.svc.StartThread(adminCtx(t, f.staffAccount), f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)

	outsider := testpkg.CreateTestAccount(t, f.db, "outsider")
	t.Cleanup(func() {
		_, _ = f.db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, outsider.ID)
	})
	ctx := claimsCtx(t, outsider.ID, []string{"users:read"})

	_, err = f.svc.GetThread(ctx, started.ThreadID)
	require.ErrorIs(t, err, messaging.ErrForbidden)
	_, err = f.svc.ListGuardians(ctx, f.chain.StudentID)
	require.ErrorIs(t, err, messaging.ErrForbidden)
}

// TestMessaging_MissingStudentIsForbidden: a studentID that does not resolve to a
// row in the tenant (stale search result, hand-crafted id) is an AUTHORIZATION
// decision — GetStudentByID returns a wrapped sql.ErrNoRows — so the service maps
// it to ErrForbidden (→ 403), never a 500 that would tell staff the child exists.
func TestMessaging_MissingStudentIsForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	_, err := f.svc.ListGuardians(ctx, 999999999)
	require.ErrorIs(t, err, messaging.ErrForbidden)
	_, err = f.svc.ListStudentThreads(ctx, 999999999)
	require.ErrorIs(t, err, messaging.ErrForbidden)
}

func TestMessaging_GraduatedStudentIsForbidden(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)

	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.students SET status = ? WHERE id = ?
	`, usersModels.StudentStatusAlumnus, f.chain.StudentID)
	require.NoError(t, err)

	_, err = f.svc.GetThread(ctx, started.ThreadID)
	require.ErrorIs(t, err, messaging.ErrForbidden)
	_, err = f.svc.PostMessage(ctx, started.ThreadID, "Reply", 0)
	require.ErrorIs(t, err, messaging.ErrForbidden)
	_, err = f.svc.ListStudentThreads(ctx, f.chain.StudentID)
	require.ErrorIs(t, err, messaging.ErrForbidden)

	inbox, err := f.svc.ListInbox(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, inbox)
}

// TestPostMessage_GuardianAccessRevoked: staff keep READ access to a historical
// thread after the guardian loses parent_portal.access, but may not WRITE to it
// (the parent APIs now hide it, so a reply would be unreadable and the SSE would
// leak to a revoked account).
func TestPostMessage_GuardianAccessRevoked(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)

	// Downgrade the guardian relationship to read-only (no parent_portal.access).
	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, f.chain.TenantID, f.chain.StudentID, f.chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = f.svc.PostMessage(ctx, started.ThreadID, "Reply", 0)
	require.ErrorIs(t, err, messaging.ErrGuardianAccessRevoked)

	// Read still works — history stays visible.
	_, err = f.svc.GetThread(ctx, started.ThreadID)
	require.NoError(t, err)
}

// TestMessaging_DisabledFeature: with operations.parent_notes_enabled off, writes
// are refused (ErrMessagingDisabled) and the unread badge goes dark (0), while the
// inbox history stays readable but with its unread pills suppressed so the badge
// and the rows agree.
func TestMessaging_DisabledFeature(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	ctx := adminCtx(t, f.staffAccount)
	started, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, started.ThreadID, "Frage")

	// Rebuild the service with the feature OFF, same DB.
	repos := repositories.NewFactory(f.db)
	disabled := messaging.NewService(messaging.Config{
		ThreadRepo: repos.ParentMessageThread, MessageRepo: repos.ParentMessage, ReadRepo: repos.ParentMessageRead,
		Persons:     newPersons(repos, f.db),
		Settings:    stubSettings{messagingEnabled: false},
		Broadcaster: f.bc, DB: f.db, Logger: slog.Default(),
	})

	_, err = disabled.PostMessage(ctx, started.ThreadID, "Reply", 0)
	require.ErrorIs(t, err, messaging.ErrMessagingDisabled)

	count, err := disabled.UnreadMessageCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "disabled feature darkens the staff badge")

	inbox, err := disabled.ListInbox(ctx, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, 0, inbox[0].UnreadCount, "disabled feature suppresses the row unread pill")

	onlyUnread, err := disabled.ListInbox(ctx, true)
	require.NoError(t, err)
	assert.Empty(t, onlyUnread)
}
