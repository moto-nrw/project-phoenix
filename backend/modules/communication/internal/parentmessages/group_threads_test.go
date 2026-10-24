package messaging_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	composition "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Integration tests for the group threads (#4784): the OGS writes to a class,
// the recipients are frozen per message, and a guardian reads only what
// reached them, never another family.

type groupFixture struct {
	svc         *messaging.Service
	portal      communication.PortalGroupThreads
	broadcaster *testpkg.RecordingBroadcaster
	staff       int64
	a, b        testpkg.ParentChain
}

func newGroupFixture(t *testing.T) (*groupFixture, func(context.Context, func(context.Context) error) error) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposAccounts := api.NewAccountSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := messaging.NewService(messaging.Config{
		UserContext:   owners.StaffStudentReads{},
		Session:       api.CommunicationSession{},
		ThreadRepo:    messaging.NewThreadRepository(reposMessages.ParentMessageThread),
		MessageRepo:   messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
		ReadRepo:      messaging.NewReadRepository(reposMessages.ParentMessageRead),
		Persons:       newPersons(reposPeople, reposAccounts, db),
		Settings:      configModels.NewParentMessagingValues(stubSettings{messagingEnabled: true}),
		Broadcaster:   broadcaster,
		Logger:        slog.Default(),
		GroupThreads:  parentStore.NewGroupThreadStore(db),
		GroupAudience: owners.NewParentGroupAudience(testpkg.NewDatabase(t, db)),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	// Both families' children are in class 1a.
	a := testpkg.CreateTestParentGuardianChain(t, db)
	b := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	run := func(ctx context.Context, fn func(context.Context) error) error {
		return testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), fn)
	}
	return &groupFixture{svc: svc, portal: parentStore.NewGroupThreadPortal(db), broadcaster: broadcaster, staff: staff.ID, a: a, b: b}, run
}

func classTarget(name string) communication.ParentGroupTarget {
	return communication.ParentGroupTarget{Type: communication.ParentGroupTargetClass, RefText: &name, Label: "Klasse " + name}
}

func TestGroupThread_ReachesEachFamilyAndKeepsThemApart(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)

	var detail *communication.ParentGroupThreadDetail
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		var err error
		detail, err = f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Ausflug", Body: "Am Freitag gehen wir in den Zoo.",
		})
		return err
	}))
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, "Olivia Berg", detail.Messages[0].SenderName)
	require.NotNil(t, detail.Thread.LastMessageAt)
	assert.Equal(t, detail.Messages[0].CreatedAt, *detail.Thread.LastMessageAt)
	require.Len(t, detail.Children, 2, "both families of class 1a are recipients")
	for _, child := range detail.Children {
		assert.False(t, child.Read)
	}
	threadID := detail.Thread.ID

	// Guardian A reads the thread: only A's child, only A's messages.
	var view *communication.PortalGuardianGroupThreadView
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		var err error
		view, _, err = f.portal.PortalReadGroupThread(ctx, threadID, f.a.AccountID, []int64{f.a.StudentID})
		return err
	}))
	require.Len(t, view.Messages, 1)
	assert.Equal(t, []int64{f.a.StudentID}, view.StudentIDs, "a guardian sees only their own child")

	// Guardian A cannot read the thread through family B's child.
	require.ErrorIs(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		_, _, err := f.portal.PortalReadGroupThread(ctx, threadID, f.a.AccountID, []int64{f.b.StudentID})
		return err
	}), communication.ErrParentGroupThreadNotFound)
	// Nor answer for family B's child.
	require.ErrorIs(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		_, err := f.portal.PortalGroupReplyTarget(ctx, threadID, f.a.AccountID, f.b.StudentID)
		return err
	}), communication.ErrParentGroupThreadNotFound)

	// The OGS sees that one of two families has read it.
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		rows, err := f.svc.ListParentGroupThreads(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, 2, rows[0].FamilyCount)
		assert.Equal(t, 1, rows[0].ReadCount)
		return nil
	}))

	// Guardian B's list carries only B's child and one unread message.
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		rows, err := f.portal.PortalListGroupThreads(ctx, f.b.AccountID, []int64{f.b.StudentID}, []int64{f.b.TenantID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, 1, rows[0].UnreadCount)
		assert.Equal(t, []int64{f.b.StudentID}, rows[0].StudentIDs)
		count, err := f.portal.PortalCountUnreadGroupThreads(ctx, f.b.AccountID, []int64{f.b.StudentID}, []int64{f.b.TenantID})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		return nil
	}))
}

func TestGroupThread_StaffListCountsMultiChildFamilyOnce(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)
	sibling := testpkg.CreateTestStudent(t, testpkg.SetupTestDB(t), "Lena", "Schneider", "1a")
	testpkg.CreateTestStudentGuardianLink(t, testpkg.SetupTestDB(t), sibling.ID, f.a.GuardianProfileID, testpkg.GuardianRolePrimaryGuardian)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Ausflug", Body: "Am Freitag gehen wir in den Zoo.",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))

	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		_, _, err := f.portal.PortalReadGroupThread(ctx, threadID, f.a.AccountID, []int64{f.a.StudentID})
		return err
	}))

	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.GetParentGroupThread(ctx, threadID)
		require.NoError(t, err)
		assert.Len(t, detail.Children, 3)
		assert.Equal(t, 2, detail.FamilyCount, "one guardian account reaches both siblings")
		assert.Equal(t, 1, detail.ReadCount)

		rows, err := f.svc.ListParentGroupThreads(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, 2, rows[0].FamilyCount, "one guardian account reaches both siblings")
		assert.Equal(t, 1, rows[0].ReadCount)
		return nil
	}))
}

func TestGroupThread_RecipientsAreFrozenPerMessage(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Info", Body: "Erste Nachricht",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))

	// A third family joins class 1a after the first message.
	late := testpkg.CreateTestParentGuardianChain(t, testpkg.SetupTestDB(t))
	var second *communication.ParentGroupThreadDetail
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		var err error
		second, err = f.svc.PostParentGroupMessage(ctx, threadID, "Zweite Nachricht")
		return err
	}))
	require.Len(t, second.Messages, 2)
	require.NotNil(t, second.Thread.LastMessageAt)
	assert.Equal(t, second.Messages[1].CreatedAt, *second.Thread.LastMessageAt)

	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		view, _, err := f.portal.PortalReadGroupThread(ctx, threadID, late.AccountID, []int64{late.StudentID})
		require.NoError(t, err)
		require.Len(t, view.Messages, 1, "a family that joined later sees only the messages sent since")
		assert.Equal(t, "Zweite Nachricht", view.Messages[0].Body)

		view, _, err = f.portal.PortalReadGroupThread(ctx, threadID, f.a.AccountID, []int64{f.a.StudentID})
		require.NoError(t, err)
		assert.Len(t, view.Messages, 2)
		return nil
	}))
}

func TestGroupThread_BroadcastsUpdatesToReachedGuardians(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Info", Body: "Erste Nachricht",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))
	assertGroupGuardianBroadcasts(t, f, "creating a thread")

	f.broadcaster.Reset()
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		_, err := f.svc.PostParentGroupMessage(ctx, threadID, "Zweite Nachricht")
		return err
	}))
	assertGroupGuardianBroadcasts(t, f, "adding a message")

	f.broadcaster.Reset()
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		_, err := f.svc.CloseParentGroupThread(ctx, threadID)
		return err
	}))
	assertGroupGuardianBroadcasts(t, f, "closing a thread")
}

func assertGroupGuardianBroadcasts(t *testing.T, f *groupFixture, action string) {
	t.Helper()
	calls := f.broadcaster.CallsByMethod("guardian")
	require.Len(t, calls, 2, "%s must wake every reached guardian", action)
	assert.ElementsMatch(t, []int64{f.a.AccountID, f.b.AccountID}, []int64{calls[0].GuardianID, calls[1].GuardianID})
	for _, call := range calls {
		assert.Equal(t, realtime.EventParentMessage, call.Event.Type)
		assert.Nil(t, call.Event.Data.StudentID, "group updates must not expose a child through SSE")
		assert.Nil(t, call.Event.Data.ThreadID, "group updates must not expose a conversation through SSE")
	}
}

func TestGroupThread_ClosedThreadTakesNoAnswersOrMessages(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Info", Body: "Hallo",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		_, err := f.portal.PortalGroupReplyTarget(ctx, threadID, f.a.AccountID, f.a.StudentID)
		return err
	}), "an open thread takes answers")

	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CloseParentGroupThread(ctx, threadID)
		require.NoError(t, err)
		assert.NotNil(t, detail.Thread.ClosedAt)
		return nil
	}))
	require.ErrorIs(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		_, err := f.portal.PortalGroupReplyTarget(ctx, threadID, f.a.AccountID, f.a.StudentID)
		return err
	}), communication.ErrParentGroupThreadClosed)
	require.ErrorIs(t, run(staffCtx, func(ctx context.Context) error {
		_, err := f.svc.PostParentGroupMessage(ctx, threadID, "Noch etwas")
		return err
	}), communication.ErrParentGroupThreadClosed)
}

func TestGroupThread_DisabledFeatureRefusesClose(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	staffCtx := adminCtx(t, f.staff)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Info", Body: "Hallo",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))

	f.svc.Settings = configModels.NewParentMessagingValues(stubSettings{messagingEnabled: false})
	require.ErrorIs(t, run(staffCtx, func(ctx context.Context) error {
		_, err := f.svc.CloseParentGroupThread(ctx, threadID)
		return err
	}), communication.ErrParentMessagingDisabled)

	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.GetParentGroupThread(ctx, threadID)
		require.NoError(t, err)
		assert.Nil(t, detail.Thread.ClosedAt, "disabled messaging must not close the thread")
		return nil
	}))
}

func TestGroupThread_PreviewMarksChildrenWithoutReachableParents(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	db := testpkg.SetupTestDB(t)
	testpkg.CreateTestStudent(t, db, "Mia", "Ohne", "1a")

	require.NoError(t, run(adminCtx(t, f.staff), func(ctx context.Context) error {
		children, err := f.svc.PreviewParentGroupAudience(ctx, classTarget("1a"))
		require.NoError(t, err)
		require.Len(t, children, 3)
		reachable := 0
		for _, child := range children {
			if child.Reachable {
				reachable++
			}
		}
		assert.Equal(t, 2, reachable, "the child without parents in moto is listed but not reachable")
		return nil
	}))
}

func TestGroupThread_RefusesATargetWithoutReachableParents(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	err := run(adminCtx(t, f.staff), func(ctx context.Context) error {
		_, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("9z"), Title: "Info", Body: "Hallo",
		})
		return err
	})
	require.ErrorIs(t, err, communication.ErrParentGroupThreadNoRecipients)
}

func TestGroupThread_RequiresStaffWhoReadEveryChild(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	err := run(claimsCtx(t, f.staff, nil), func(ctx context.Context) error {
		_, err := f.svc.ListParentGroupThreads(ctx)
		return err
	})
	require.ErrorIs(t, err, communication.ErrParentMessagingForbidden)
}

func createClassThread(t *testing.T, f *groupFixture, run func(context.Context, func(context.Context) error) error, title string) {
	t.Helper()
	require.NoError(t, run(adminCtx(t, f.staff), func(ctx context.Context) error {
		_, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: title, Body: "Hallo",
		})
		return err
	}))
}

func TestGroupThreadListsQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	db := testpkg.SetupTestDB(t)
	for _, title := range []string{"Eins", "Zwei", "Drei"} {
		createClassThread(t, f, run, title)
	}

	counter := testpkg.CaptureQueriesForContext(t, db)
	rows, err := testpkg.MeasuredTenantQuery0(t, counter.Context(adminCtx(t, f.staff)), db, counter, f.svc.ListParentGroupThreads)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	testpkg.AssertQueryBudget(t, "modules.communication.parent_group_threads.list", counter.Queries())

	guardianCounter := testpkg.CaptureQueriesForContext(t, db)
	guardianRows, err := testpkg.MeasuredTenantQuery0(t, guardianCounter.Context(testpkg.Ctx(t)), db, guardianCounter,
		func(ctx context.Context) ([]communication.PortalGuardianGroupThread, error) {
			return f.portal.PortalListGroupThreads(ctx, f.a.AccountID, []int64{f.a.StudentID}, []int64{f.a.TenantID})
		})
	require.NoError(t, err)
	require.Len(t, guardianRows, 3)
	testpkg.AssertQueryBudget(t, "modules.communication.parent_group_threads.list_for_guardian", guardianCounter.Queries())
}

type groupRetentionDays int

func (d groupRetentionDays) ParentMessageRetentionDays(context.Context) (int, error) {
	return int(d), nil
}

// TestGroupThread_RetentionDeletesExpiredGroupMessages: the nightly parent
// message retention removes group messages past the window, their recipient
// rows, and the thread left empty.
func TestGroupThread_RetentionDeletesExpiredGroupMessages(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	db := testpkg.SetupTestDB(t)
	createClassThread(t, f, run, "Alt")

	_, err := db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.parent_group_messages SET created_at = NOW() - INTERVAL '400 days' WHERE tenant_id = ?;
		UPDATE users.parent_group_threads SET created_at = NOW() - INTERVAL '400 days' WHERE tenant_id = ?;`,
		testpkg.Tenant(t), testpkg.Tenant(t))
	require.NoError(t, err)

	cleanup := composition.NewParentMessageRetention(db, groupRetentionDays(365))
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		result, err := cleanup.CleanupExpiredParentMessages(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesDeleted)
		assert.Equal(t, 1, result.ThreadsDeleted)
		return nil
	}))

	var remaining int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT COUNT(*) FROM users.parent_group_threads WHERE tenant_id = ?) +
		(SELECT COUNT(*) FROM users.parent_group_messages WHERE tenant_id = ?) +
		(SELECT COUNT(*) FROM users.parent_group_message_recipients WHERE tenant_id = ?)`,
		testpkg.Tenant(t), testpkg.Tenant(t), testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &remaining))
	assert.Zero(t, remaining)
}

func TestGroupThread_RetentionRepairsLatestActivity(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	db := testpkg.SetupTestDB(t)
	staffCtx := adminCtx(t, f.staff)

	var detail *communication.ParentGroupThreadDetail
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		var err error
		detail, err = f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Aktivität", Body: "Erste Nachricht",
		})
		return err
	}))
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		var err error
		detail, err = f.svc.PostParentGroupMessage(ctx, detail.Thread.ID, "Zweite Nachricht")
		return err
	}))
	require.Len(t, detail.Messages, 2)

	latestID := detail.Messages[1].ID
	_, err := db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.parent_group_messages
		SET created_at = NOW() - INTERVAL '400 days'
		WHERE id = ? AND tenant_id = ?`, latestID, testpkg.Tenant(t))
	require.NoError(t, err)

	cleanup := composition.NewParentMessageRetention(db, groupRetentionDays(365))
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		result, err := cleanup.CleanupExpiredParentMessages(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesDeleted)
		assert.Zero(t, result.ThreadsDeleted)
		return nil
	}))

	var lastMessageAt, remainingMessageAt time.Time
	require.NoError(t, db.NewRaw(`
		SELECT last_message_at FROM users.parent_group_threads
		WHERE id = ? AND tenant_id = ?`, detail.Thread.ID, testpkg.Tenant(t)).
		Scan(testpkg.Ctx(t), &lastMessageAt))
	require.NoError(t, db.NewRaw(`
		SELECT created_at FROM users.parent_group_messages
		WHERE thread_id = ? AND tenant_id = ?`, detail.Thread.ID, testpkg.Tenant(t)).
		Scan(testpkg.Ctx(t), &remainingMessageAt))
	assert.Equal(t, remainingMessageAt, lastMessageAt)
}

func TestGroupThread_RetentionClearsActivityOfRecentEmptyThread(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	db := testpkg.SetupTestDB(t)
	staffCtx := adminCtx(t, f.staff)

	var threadID int64
	require.NoError(t, run(staffCtx, func(ctx context.Context) error {
		detail, err := f.svc.CreateParentGroupThread(ctx, communication.ParentGroupThreadInput{
			Target: classTarget("1a"), Title: "Neu", Body: "Alte Nachricht",
		})
		if err == nil {
			threadID = detail.Thread.ID
		}
		return err
	}))

	// Keep the just-created thread, but expire its only message. The activity
	// timestamp must not keep pointing at the deleted message.
	_, err := db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.parent_group_messages
		SET created_at = NOW() - INTERVAL '400 days'
		WHERE thread_id = ? AND tenant_id = ?`, threadID, testpkg.Tenant(t))
	require.NoError(t, err)

	cleanup := composition.NewParentMessageRetention(db, groupRetentionDays(365))
	require.NoError(t, run(testpkg.Ctx(t), func(ctx context.Context) error {
		result, err := cleanup.CleanupExpiredParentMessages(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesDeleted)
		assert.Zero(t, result.ThreadsDeleted)
		return nil
	}))

	var lastMessageAt *time.Time
	require.NoError(t, db.NewRaw(`
		SELECT last_message_at FROM users.parent_group_threads
		WHERE id = ? AND tenant_id = ?`, threadID, testpkg.Tenant(t)).
		Scan(testpkg.Ctx(t), &lastMessageAt))
	assert.Nil(t, lastMessageAt)
}
