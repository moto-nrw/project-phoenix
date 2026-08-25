package staffmessaging_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repoUsers "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/staffmessaging"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newService wires a service against the real repositories with messaging
// switched ON, which is the state every test below assumes unless it says
// otherwise.
func newService(t *testing.T, db *bun.DB) *staffmessaging.Service {
	t.Helper()
	return newServiceWithEnabled(t, db, true, 365)
}

func newServiceWithEnabled(t *testing.T, db *bun.DB, enabled bool, retentionDays int) *staffmessaging.Service {
	t.Helper()

	settings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			if key == configModels.KeyStaffMessagingEnabled {
				return enabled, nil
			}
			return false, nil
		},
		ResolveIntFn: func(_ context.Context, key string) (int, error) {
			if key == configModels.KeyGDPRStaffMessageRetentionDays {
				return retentionDays, nil
			}
			return 0, nil
		},
	}

	// The REAL person service, not a mock: freezing the sender's display name
	// onto the message is part of what these tests verify, so the lookup must
	// go through the same path production uses. FindByAccountID only touches
	// PersonRepo, so the rest of the DI bundle stays empty on purpose.
	persons := userService.NewPersonService(userService.PersonServiceDependencies{
		PersonRepo: repoUsers.NewPersonRepository(db),
	})

	return staffmessaging.NewService(staffmessaging.Config{
		ThreadRepo:  repoUsers.NewStaffMessageThreadRepository(db),
		MessageRepo: repoUsers.NewStaffMessageRepository(db),
		ReadRepo:    repoUsers.NewStaffMessageReadRepository(db),
		Persons:     persons,
		Settings:    settings,
		DB:          db,
	})
}

// asAccount returns a context that authenticates as the given account inside
// this test's tenant.
func asAccount(t *testing.T, accountID int64) context.Context {
	t.Helper()
	return context.WithValue(testpkg.Ctx(t), jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
}

// twoColleagues creates two staff accounts that are active members of this
// test's school.
func twoColleagues(t *testing.T, db *bun.DB) (anna, ben int64) {
	t.Helper()
	_, annaAccount := testpkg.CreateTestStaffWithAccount(t, db, "Anna", "Mustermann")
	_, benAccount := testpkg.CreateTestStaffWithAccount(t, db, "Ben", "Beispiel")
	return annaAccount.ID, benAccount.ID
}

func TestOpenThreadIsGetOrCreate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	first, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)
	require.NotZero(t, first.ThreadID)

	// The same pair from the OTHER side must land in the SAME conversation —
	// that is the entire point of the sorted participant key.
	second, err := svc.OpenThread(asAccount(t, ben), anna)
	require.NoError(t, err)
	assert.Equal(t, first.ThreadID, second.ThreadID)

	// And each side sees the other as the counterpart.
	assert.Equal(t, ben, first.CounterpartAccountID)
	assert.Equal(t, anna, second.CounterpartAccountID)
}

func TestPostMessageAndReadBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	sent, err := svc.PostMessage(asAccount(t, anna), thread.ThreadID, "  Kannst du heute die Gruppe 2a übernehmen?  ")
	require.NoError(t, err)
	assert.Equal(t, "Kannst du heute die Gruppe 2a übernehmen?", sent.Body, "body must be trimmed")
	assert.Equal(t, "Anna Mustermann", sent.SenderName, "sender name is frozen at send time")
	require.False(t, sent.CreatedAt.IsZero(), "created_at must come back from the database")

	detail, err := svc.GetThread(asAccount(t, ben), thread.ThreadID)
	require.NoError(t, err)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, "Anna Mustermann", detail.Messages[0].SenderName)
}

func TestUnreadCountLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Erste Nachricht")
	require.NoError(t, err)
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Zweite Nachricht")
	require.NoError(t, err)

	// The recipient has two unread…
	benCount, err := svc.UnreadMessageCount(asAccount(t, ben))
	require.NoError(t, err)
	assert.Equal(t, 2, benCount)

	// …and the SENDER has none. A just-sent message must never count against
	// its own author, which is what the cursor advance in PostMessage buys.
	annaCount, err := svc.UnreadMessageCount(asAccount(t, anna))
	require.NoError(t, err)
	assert.Equal(t, 0, annaCount)

	// Opening the thread clears it.
	_, err = svc.GetThread(asAccount(t, ben), thread.ThreadID)
	require.NoError(t, err)
	benCount, err = svc.UnreadMessageCount(asAccount(t, ben))
	require.NoError(t, err)
	assert.Equal(t, 0, benCount)
}

// TestUnreadCountSurvivesTimestampTie is the regression guard for the composite
// cursor. Two messages inserted in the same transaction can share a
// clock_timestamp() value; a timestamp-only cursor would swallow the second one.
func TestUnreadCountSurvivesTimestampTie(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	first, err := svc.PostMessage(asAccount(t, anna), thread.ThreadID, "A")
	require.NoError(t, err)
	second, err := svc.PostMessage(asAccount(t, anna), thread.ThreadID, "B")
	require.NoError(t, err)

	// Force the tie the composite cursor exists for.
	ctx := testpkg.Ctx(t)
	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", first.CreatedAt).
		Where("id = ?", second.ID).
		Exec(ctx)
	require.NoError(t, err)

	count, err := svc.UnreadMessageCount(asAccount(t, ben))
	require.NoError(t, err)
	assert.Equal(t, 2, count, "a message tied on created_at must still count as unread")
}

func TestNonParticipantCannotReadThread(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)
	_, outsiderAccount := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "Colleague")

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Vertraulich")
	require.NoError(t, err)

	// A colleague at the SAME school who is not in the conversation gets the
	// same answer as for a thread that does not exist — no 403, so thread ids
	// cannot be probed.
	_, err = svc.GetThread(asAccount(t, outsiderAccount.ID), thread.ThreadID)
	require.ErrorIs(t, err, staffmessaging.ErrNotParticipant)

	_, err = svc.PostMessage(asAccount(t, outsiderAccount.ID), thread.ThreadID, "Ich lese mit")
	require.ErrorIs(t, err, staffmessaging.ErrNotParticipant)

	// And the outsider's inbox stays empty.
	inbox, err := svc.ListInbox(asAccount(t, outsiderAccount.ID), false)
	require.NoError(t, err)
	assert.Empty(t, inbox)
}

func TestInactiveMemberIsNotAddressable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	ctx := testpkg.Ctx(t)
	_, err := db.NewUpdate().
		Table("auth.account_tenants").
		Set("status = ?", authModels.AccountTenantStatusInactive).
		Where("account_id = ? AND tenant_id = ?", ben, testpkg.Tenant(t)).
		Exec(ctx)
	require.NoError(t, err)

	_, err = svc.OpenThread(asAccount(t, anna), ben)
	require.ErrorIs(t, err, staffmessaging.ErrRecipientNotAvailable)

	// …and they disappear from the picker.
	recipients, err := svc.ListMessageableStaff(asAccount(t, anna))
	require.NoError(t, err)
	for _, r := range recipients {
		assert.NotEqual(t, ben, r.AccountID, "an inactive member must not be offered as a recipient")
	}
}

func TestSelfConversationRejected(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, _ := twoColleagues(t, db)

	_, err := svc.OpenThread(asAccount(t, anna), anna)
	require.ErrorIs(t, err, staffmessaging.ErrSelfConversation)
}

func TestRecipientPickerExcludesSelf(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	recipients, err := svc.ListMessageableStaff(asAccount(t, anna))
	require.NoError(t, err)

	ids := make([]int64, 0, len(recipients))
	for _, r := range recipients {
		ids = append(ids, r.AccountID)
	}
	assert.NotContains(t, ids, anna, "the caller must not be able to write to themselves")
	assert.Contains(t, ids, ben)
}

func TestBodyValidation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "   ")
	require.ErrorIs(t, err, staffmessaging.ErrEmptyMessage)

	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, strings.Repeat("ü", 2001))
	require.ErrorIs(t, err, staffmessaging.ErrMessageTooLong, "the limit counts runes, not bytes")

	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, strings.Repeat("ü", 2000))
	require.NoError(t, err, "exactly at the limit must still pass")
}

// TestDisabledSchoolIsBlocked pins the fail-CLOSED contract: with the feature
// switch off, nothing works — including reads.
func TestDisabledSchoolIsBlocked(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	enabled := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := enabled.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	disabled := newServiceWithEnabled(t, db, false, 365)
	_, err = disabled.OpenThread(asAccount(t, anna), ben)
	require.ErrorIs(t, err, staffmessaging.ErrMessagingDisabled)
	_, err = disabled.PostMessage(asAccount(t, anna), thread.ThreadID, "Hallo")
	require.ErrorIs(t, err, staffmessaging.ErrMessagingDisabled)
	_, err = disabled.ListInbox(asAccount(t, anna), false)
	require.ErrorIs(t, err, staffmessaging.ErrMessagingDisabled)
	_, err = disabled.UnreadMessageCount(asAccount(t, anna))
	require.ErrorIs(t, err, staffmessaging.ErrMessagingDisabled)
}

func TestInboxShowsCounterpartAndPreview(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	// An empty conversation must NOT appear — a get-or-create that was never
	// followed by a send is not a conversation yet.
	inbox, err := svc.ListInbox(asAccount(t, anna), false)
	require.NoError(t, err)
	assert.Empty(t, inbox, "a thread without messages must not clutter the inbox")

	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Bis gleich")
	require.NoError(t, err)

	inbox, err = svc.ListInbox(asAccount(t, anna), false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, ben, inbox[0].CounterpartAccountID)
	assert.Equal(t, "Ben Beispiel", inbox[0].CounterpartName)
	assert.Equal(t, "Bis gleich", inbox[0].LastMessageBody)
	assert.Equal(t, 0, inbox[0].UnreadCount, "own message is not unread to the sender")

	benInbox, err := svc.ListInbox(asAccount(t, ben), false)
	require.NoError(t, err)
	require.Len(t, benInbox, 1)
	assert.Equal(t, anna, benInbox[0].CounterpartAccountID)
	assert.Equal(t, 1, benInbox[0].UnreadCount)
}

func TestInboxOnlyUnreadFilter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Neu")
	require.NoError(t, err)

	unread, err := svc.ListInbox(asAccount(t, ben), true)
	require.NoError(t, err)
	assert.Len(t, unread, 1)

	// The sender has nothing unread, so their filtered inbox is empty.
	unread, err = svc.ListInbox(asAccount(t, anna), true)
	require.NoError(t, err)
	assert.Empty(t, unread)
}

// TestSendingDoesNotSwallowIncomingUnread is the regression guard for the
// interleaved-send case: while Anna composes a reply, Ben writes two messages.
// Anna's own send must not mark those two as read - she never saw them.
//
// The trap it guards: advancing the sender's cursor to their just-sent message
// looks harmless ("you have read what you wrote"), but the cursor is a
// thread-wide watermark, so it drags past everything the counterpart sent in
// between. It is also unnecessary - the unread predicate already excludes the
// reader's own messages - which is why the fix was to stop advancing at all.
func TestSendingDoesNotSwallowIncomingUnread(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, ben := twoColleagues(t, db)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	// Anna opens the conversation, so her cursor sits at the start.
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Kurze Frage")
	require.NoError(t, err)
	_, err = svc.GetThread(asAccount(t, ben), thread.ThreadID)
	require.NoError(t, err)

	// Ben answers twice while Anna is typing.
	_, err = svc.PostMessage(asAccount(t, ben), thread.ThreadID, "Antwort eins")
	require.NoError(t, err)
	_, err = svc.PostMessage(asAccount(t, ben), thread.ThreadID, "Antwort zwei")
	require.NoError(t, err)

	before, err := svc.UnreadMessageCount(asAccount(t, anna))
	require.NoError(t, err)
	require.Equal(t, 2, before, "Ben's two messages are unread to Anna")

	// Anna sends her own message without having opened the thread.
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Ach, hat sich erledigt")
	require.NoError(t, err)

	after, err := svc.UnreadMessageCount(asAccount(t, anna))
	require.NoError(t, err)
	assert.Equal(t, 2, after,
		"sending must not mark the counterpart's unseen messages as read")

	// And her own message still is not unread to her - the predicate excludes it.
	inbox, err := svc.ListInbox(asAccount(t, anna), false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, 2, inbox[0].UnreadCount)
}
