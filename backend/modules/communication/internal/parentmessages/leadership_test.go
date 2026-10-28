package messaging_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The leadership conversations of #5200: a staff member without
// parent_messages:leadership must not see them anywhere, while a custom role
// holding the permission and admin:* both do.

// verifiedStaff is the caller context of a verified staff member, who reads
// every child of the school (#2329) without being an administrator.
type verifiedStaff struct{}

func (verifiedStaff) HasCurrentStaff(context.Context) (bool, error) { return true, nil }
func (verifiedStaff) MyGroupIDs(context.Context) ([]int64, error)   { return nil, nil }

var (
	plainStaffPermissions = []string{"users:read"}
	customRolePermissions = []string{"users:read", "parent_messages:leadership"}
	adminPermissions      = []string{"admin:*"}
)

type leadershipFixture struct {
	*fixture
	teamThreadID       int64
	leadershipThreadID int64
}

func newLeadershipFixture(t *testing.T, leadershipEnabled bool) *leadershipFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	reposAccounts := api.NewAccountSuiteRepositories(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	bc := testpkg.NewRecordingBroadcaster()
	threads := messaging.NewThreadRepository(reposMessages.ParentMessageThread)
	svc := messaging.NewService(messaging.Config{
		Leaders:     communicationtest.Leaders{{AccountID: 1, FirstName: "Swantje", LastName: "Meier"}},
		UserContext: owners.StaffStudentReads{Caller: verifiedStaff{}},
		Session:     api.CommunicationSession{},
		ThreadRepo:  threads,
		MessageRepo: messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
		ReadRepo:    messaging.NewReadRepository(reposMessages.ParentMessageRead),
		Persons:     newPersons(reposPeople, reposAccounts, db),
		Settings:    configModels.NewParentMessagingValues(stubSettings{messagingEnabled: true, leadershipEnabled: leadershipEnabled}),
		Broadcaster: bc,
		Logger:      slog.Default(),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	f := &leadershipFixture{fixture: &fixture{db: db, svc: svc, bc: bc, chain: chain, staffAccount: staffAccount.ID}}

	ctx := testpkg.Ctx(t)
	team, err := testpkg.TenantQuery4(t, ctx, db, threads.GetOrCreate, chain.TenantID, chain.StudentID, chain.AccountID, communication.ParentMessageAudienceTeam)
	require.NoError(t, err)
	leadership, err := testpkg.TenantQuery4(t, ctx, db, threads.GetOrCreate, chain.TenantID, chain.StudentID, chain.AccountID, communication.ParentMessageAudienceLeadership)
	require.NoError(t, err)
	require.NotEqual(t, team.ID, leadership.ID, "a child has one conversation per audience")
	createGuardianMessage(t, db, chain, team.ID, "Frage an das Team")
	createGuardianMessage(t, db, chain, leadership.ID, "Gesprächstermin wegen einer Mitarbeiterin")
	f.teamThreadID, f.leadershipThreadID = team.ID, leadership.ID
	return f
}

func (f *leadershipFixture) ctx(t *testing.T, permissions []string) context.Context {
	return claimsCtx(t, f.staffAccount, permissions)
}

func inboxThreadIDs(rows []*messaging.InboxThread) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ThreadID)
	}
	return ids
}

func TestLeadershipConversation_InvisibleWithoutPermission(t *testing.T) {
	t.Parallel()
	f := newLeadershipFixture(t, true)
	ctx := f.ctx(t, plainStaffPermissions)

	inbox, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListInbox, false)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.teamThreadID}, inboxThreadIDs(inbox), "the inbox lists only the team conversation")

	unreadOnly, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListInbox, true)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.teamThreadID}, inboxThreadIDs(unreadOnly))

	count, err := testpkg.TenantQuery0(t, ctx, f.db, f.svc.UnreadMessageCount)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "the leadership message must not light the badge")

	studentRows, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListStudentThreads, f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.teamThreadID}, inboxThreadIDs(studentRows), "the child profile card shows only the team conversation")

	_, err = testpkg.TenantQuery1(t, ctx, f.db, f.svc.GetThread, f.leadershipThreadID)
	require.ErrorIs(t, err, messaging.ErrThreadNotFound, "opening the id answers as for a missing thread (404)")
	_, err = testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, f.leadershipThreadID, "Antwort", int64(0))
	require.ErrorIs(t, err, messaging.ErrThreadNotFound)
	err = testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, f.leadershipThreadID)
	require.ErrorIs(t, err, messaging.ErrThreadNotFound)

	_, err = testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Hallo", communication.ParentMessageAudienceLeadership)
	require.ErrorIs(t, err, messaging.ErrForbidden, "only the leadership may start a leadership conversation")
	_, _, err = listGuardians(t, ctx, f, communication.ParentMessageAudienceLeadership)
	require.ErrorIs(t, err, messaging.ErrForbidden)

	// Mark-all-read clears only what the caller sees: the leadership
	// conversation stays unread for the same account once it holds the
	// permission.
	_, err = testpkg.TenantQuery0(t, ctx, f.db, f.svc.MarkAllRead)
	require.NoError(t, err)
	leaderCount, err := testpkg.TenantQuery0(t, f.ctx(t, customRolePermissions), f.db, f.svc.UnreadMessageCount)
	require.NoError(t, err)
	assert.Equal(t, 1, leaderCount, "only the team message was cleared")
}

func TestLeadershipConversation_VisibleToCustomRoleAndAdmin(t *testing.T) {
	t.Parallel()
	for name, permissions := range map[string][]string{"custom role": customRolePermissions, "admin": adminPermissions} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			t.Parallel()
			f := newLeadershipFixture(t, true)
			ctx := f.ctx(t, permissions)

			inbox, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListInbox, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []int64{f.teamThreadID, f.leadershipThreadID}, inboxThreadIDs(inbox))
			for _, row := range inbox {
				want := communication.ParentMessageAudienceTeam
				if row.ThreadID == f.leadershipThreadID {
					want = communication.ParentMessageAudienceLeadership
				}
				assert.Equal(t, want, row.Audience, "the inbox marks the leadership conversation")
			}

			count, err := testpkg.TenantQuery0(t, ctx, f.db, f.svc.UnreadMessageCount)
			require.NoError(t, err)
			assert.Equal(t, 2, count)

			studentRows, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.ListStudentThreads, f.chain.StudentID)
			require.NoError(t, err)
			assert.Len(t, studentRows, 2)

			detail, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.GetThread, f.leadershipThreadID)
			require.NoError(t, err)
			assert.Equal(t, communication.ParentMessageAudienceLeadership, detail.Audience)
			assert.Equal(t, []string{"Swantje M."}, detail.LeadershipReaders, "the readers are named when the conversation is read")
			assert.Equal(t, "Sabine Schneider", detail.GuardianName, "the one guardian the conversation belongs to")
		})
	}
}

func TestLeadershipConversation_WakesOnlyLeadershipTabs(t *testing.T) {
	t.Parallel()
	f := newLeadershipFixture(t, true)
	ctx := f.ctx(t, customRolePermissions)
	detail, err := testpkg.TenantQuery1(t, ctx, f.db, f.svc.GetThread, f.leadershipThreadID)
	require.NoError(t, err)

	f.bc.Reset()
	_, err = testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, f.leadershipThreadID, "Gern, am Montag?", detail.Messages[0].ID)
	require.NoError(t, err)
	assert.Empty(t, f.bc.CallsByMethod("parent"), "no tenant-wide staff wake for a leadership conversation")
	leadership := f.bc.CallsByMethod("leadership")
	require.NotEmpty(t, leadership)
	assert.Equal(t, realtime.EventParentMessage, leadership[0].Event.Type)
	assert.Equal(t, f.chain.AccountID, leadership[0].GuardianID)

	f.bc.Reset()
	require.NoError(t, testpkg.TenantCommand1(t, ctx, f.db, f.svc.MarkUnread, f.leadershipThreadID))
	assert.Empty(t, f.bc.CallsByMethod("parent"), "marking a leadership conversation unread stays with the leadership")
	assert.NotEmpty(t, f.bc.CallsByMethod("leadership"))
}

func TestLeadershipConversation_StartNeedsTheSchoolsChoice(t *testing.T) {
	t.Parallel()

	off := newLeadershipFixture(t, false)
	ctx := off.ctx(t, customRolePermissions)
	// The existing conversation stays open after the school switched it off.
	detail, err := testpkg.TenantQuery4(t, ctx, off.db, off.svc.StartThread, off.chain.StudentID, off.chain.AccountID, "Antwort der Leitung", communication.ParentMessageAudienceLeadership)
	require.NoError(t, err)
	assert.Equal(t, off.leadershipThreadID, detail.ThreadID)

	// A guardian without a leadership conversation gets no new one.
	other := testpkg.CreateTestParentGuardianChain(t, off.db)
	_, err = testpkg.TenantQuery4(t, ctx, off.db, off.svc.StartThread, other.StudentID, other.AccountID, "Hallo", communication.ParentMessageAudienceLeadership)
	require.ErrorIs(t, err, communication.ErrParentMessageLeadershipUnavailable)
	_, err = testpkg.TenantQuery3(t, ctx, off.db, off.svc.OpenThread, other.StudentID, other.AccountID, communication.ParentMessageAudienceLeadership)
	require.ErrorIs(t, err, communication.ErrParentMessageLeadershipUnavailable)

	on := newLeadershipFixture(t, true)
	onCtx := on.ctx(t, adminPermissions)
	fresh := testpkg.CreateTestParentGuardianChain(t, on.db)
	started, err := testpkg.TenantQuery4(t, onCtx, on.db, on.svc.StartThread, fresh.StudentID, fresh.AccountID, "Hallo", communication.ParentMessageAudienceLeadership)
	require.NoError(t, err)
	assert.Equal(t, communication.ParentMessageAudienceLeadership, started.Audience)

	_, err = testpkg.TenantQuery4(t, onCtx, on.db, on.svc.StartThread, fresh.StudentID, fresh.AccountID, "Hallo", "everyone")
	require.ErrorIs(t, err, communication.ErrParentMessageInvalidAudience)
}

func TestLeadershipReaders_PreviewOnRecipientList(t *testing.T) {
	t.Parallel()
	f := newLeadershipFixture(t, true)

	guardians, preview, err := listGuardians(t, f.ctx(t, customRolePermissions), f, communication.ParentMessageAudienceLeadership)
	require.NoError(t, err)
	require.NotEmpty(t, guardians)
	assert.Equal(t, communication.ParentMessageReadersLeadership, preview.School.Kind)
	assert.Equal(t, []string{"Swantje M."}, preview.School.Names)
	assert.True(t, preview.LeadershipAvailable)

	_, team, err := listGuardians(t, f.ctx(t, plainStaffPermissions), f, "")
	require.NoError(t, err)
	assert.Equal(t, communication.ParentMessageReadersAllStaff, team.School.Kind)
	assert.Empty(t, team.School.Names)
	assert.False(t, team.LeadershipAvailable, "no leadership names or option for a caller without the permission")
}

func listGuardians(t *testing.T, ctx context.Context, f *leadershipFixture, audience string) ([]*messaging.MessageableGuardian, communication.ParentMessageReaderPreview, error) {
	t.Helper()
	var preview communication.ParentMessageReaderPreview
	guardians, err := testpkg.TenantQuery2(t, ctx, f.db, func(ctx context.Context, studentID int64, audience string) ([]*messaging.MessageableGuardian, error) {
		rows, p, err := f.svc.ListGuardians(ctx, studentID, audience)
		preview = p
		return rows, err
	}, f.chain.StudentID, audience)
	return guardians, preview, err
}
