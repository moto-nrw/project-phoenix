package parentportal_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// The answer path of the group threads (#4784): a guardian answers the OGS
// in the 1:1 conversation of their child, with a reference to the group
// thread. No other family ever sees the answer.

func buildGroupThreadPortal(t *testing.T) (*parentService.Portal, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	bc := testpkg.NewRecordingBroadcaster()
	settings := parentSettingsStub{boolValues: map[string]bool{parentService.KeyParentNotesEnabled: true}}
	portal := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:   owners.ParentGuardianChanges(),
		DeparturePolicy:   owners.ParentDeparturePolicy(),
		GuardianPolicy:    owners.NewParentGuardianPolicy(),
		Fingerprints:      owners.SecurityFingerprints{},
		Declarations:      owners.ParentDeclarationPolicy(),
		Locales:           owners.ParentPortalLocales(),
		Runtime:           owners.ParentPortalRuntime{},
		MessagingEnabled:  owners.BindParentMessagingGate(settings, slog.Default()),
		CarePlan:          owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:            owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:      careProfiles(t, db),
		ChildRepo:         repos.ParentChildReads(),
		StatusDayRepo:     repos.StudentStatusDay,
		StudentRepo:       owners.NewParentStudentReads(reposPeople.Student),
		CareExceptions:    owners.NewParentCareExceptions(repos.CarePlan()),
		Settings:          settings,
		StudentUpdates:    owners.BindParentStudentUpdates(bc),
		MessageThreadRepo: owners.BindParentMessageThreadAccess(reposMessages.ParentMessageThread),
		MessageRepo:       owners.BindParentMessageReads(communicationtest.NewParentMessageRepository(db)),
		MessageReadRepo:   owners.BindParentMessageInboxReads(reposMessages.ParentMessageRead),
		Conversations:     api.NewParentConversationCore(reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db), reposMessages.ParentMessageRead, bc, slog.Default()),
		GroupThreads:      api.NewParentGroupThreadTestPort(testpkg.NewDatabase(t, db)),
		Logger:            slog.Default(),
		DataRequests:      owners.NewParentDataRequestCommands(repos.CarePlan()),
		CarePlanPolicy:    owners.NewParentCarePlanPolicy(),
	})
	return portal, bc, db
}

func TestGroupThreadAnswer_GoesOnlyToTheOGS(t *testing.T) {
	t.Parallel()
	portal, bc, db := buildGroupThreadPortal(t)
	a := testpkg.CreateTestParentGuardianChain(t, db)
	b := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	staffCtx := context.WithValue(testpkg.Ctx(t), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(staff.ID)})
	staffCtx = context.WithValue(staffCtx, sessiontoken.CtxPermissions, []string{"admin:*"})
	var threadID int64
	require.NoError(t, testpkg.WithinTenantContext(t, staffCtx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		var err error
		threadID, err = api.CreateParentGroupClassThreadForTest(ctx, testpkg.NewDatabase(t, db), staff.ID, "1a", "Sommerfest", "Wer bringt einen Kuchen mit?")
		return err
	}))

	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	// Both families see the thread; it counts toward each unread badge.
	for _, chain := range []testpkg.ParentChain{a, b} {
		threads, err := portal.ListGroupThreads(ctx, chain.AccountID)
		require.NoError(t, err)
		require.Len(t, threads, 1)
		require.Len(t, threads[0].Children, 1)
		assert.Equal(t, chain.StudentID, threads[0].Children[0].StudentID, "a family sees only its own child")
		unread, err := portal.UnreadMessageCount(ctx, chain.AccountID)
		require.NoError(t, err)
		assert.Equal(t, 1, unread)
	}

	// Family A answers. The answer lands in A's 1:1 conversation with the
	// reference to the thread.
	view, err := portal.PostChildReply(ctx, a.AccountID, a.StudentID, "Wir bringen Muffins.", threadID)
	require.NoError(t, err)
	require.Len(t, view.Messages, 1)
	require.NotNil(t, view.Messages[0].GroupThreadID)
	assert.Equal(t, threadID, *view.Messages[0].GroupThreadID)
	assert.Equal(t, "Sommerfest", view.Messages[0].GroupThreadTitle)

	// Family B sees neither the answer in its conversation nor in the thread.
	bConversation, err := portal.GetChildConversation(ctx, b.AccountID, b.StudentID)
	require.NoError(t, err)
	assert.Empty(t, bConversation.Messages, "another family's answer never reaches this conversation")
	bc.Reset()
	bThread, err := portal.GetGroupThread(ctx, b.AccountID, threadID)
	require.NoError(t, err)
	require.Len(t, bThread.Messages, 1, "the group thread holds only the OGS messages")
	assert.Equal(t, "Wer bringt einen Kuchen mit?", bThread.Messages[0].Body)
	readEvents := bc.CallsByMethod("parent")
	require.Len(t, readEvents, 1, "advancing a group-thread read cursor wakes open staff details")
	assert.Equal(t, b.TenantID, readEvents[0].TenantID)
	assert.Equal(t, b.AccountID, readEvents[0].GuardianID)
	assert.Equal(t, realtime.EventParentMessageRead, readEvents[0].Event.Type)
	assert.Nil(t, readEvents[0].Event.Data.ThreadID)
	assert.Nil(t, readEvents[0].Event.Data.StudentID)

	// Family B cannot answer in family A's name or for A's child.
	_, err = portal.PostChildReply(ctx, b.AccountID, a.StudentID, "Hallo", threadID)
	require.Error(t, err)

	// Once closed, the thread takes no further answers.
	require.NoError(t, testpkg.WithinTenantContext(t, staffCtx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		return api.CloseParentGroupThreadForTest(ctx, testpkg.NewDatabase(t, db), staff.ID, threadID)
	}))
	_, err = portal.PostChildReply(ctx, b.AccountID, b.StudentID, "Zu spät", threadID)
	require.ErrorIs(t, err, parentService.ErrGroupThreadClosed)
}

func TestGroupThreadAnswerRetryAfterCloseReturnsStoredMessage(t *testing.T) {
	t.Parallel()
	portal, _, db := buildGroupThreadPortal(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	staffCtx := context.WithValue(testpkg.Ctx(t), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(staff.ID)})
	staffCtx = context.WithValue(staffCtx, sessiontoken.CtxPermissions, []string{"admin:*"})
	var threadID int64
	require.NoError(t, testpkg.WithinTenantContext(t, staffCtx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		var err error
		threadID, err = api.CreateParentGroupClassThreadForTest(ctx, testpkg.NewDatabase(t, db), staff.ID, "1a", "Sommerfest", "Wer bringt Kuchen?")
		return err
	}))
	ctx := parentService.WithMessageSendID(testpkg.WithPackageTenantRuntime(context.Background()), "guardian-group-retry")
	first, err := portal.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Wir bringen Muffins.", threadID)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTenantContext(t, staffCtx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		return api.CloseParentGroupThreadForTest(ctx, testpkg.NewDatabase(t, db), staff.ID, threadID)
	}))
	retry, err := portal.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Wir bringen Muffins.", threadID)
	require.NoError(t, err)
	require.Len(t, retry.Messages, 1)
	require.Equal(t, first.Messages[0].ID, retry.Messages[0].ID)
	require.Equal(t, first.Messages[0].CreatedAt, retry.Messages[0].CreatedAt)
	require.Equal(t, "Sommerfest", retry.Messages[0].GroupThreadTitle)
	_, err = portal.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Anderer Text", threadID)
	require.ErrorIs(t, err, parentService.ErrMessageSendConflict)
	_, err = portal.PostChildReply(parentService.WithMessageSendID(ctx, "new-send-after-close"), chain.AccountID, chain.StudentID, "Wir bringen Muffins.", threadID)
	require.ErrorIs(t, err, parentService.ErrGroupThreadClosed)
	_, err = portal.PostChildReply(ctx, other.AccountID, other.StudentID, "Wir bringen Muffins.", threadID)
	require.ErrorIs(t, err, parentService.ErrGroupThreadClosed)
	_, err = portal.PostChildReply(ctx, other.AccountID, chain.StudentID, "Wir bringen Muffins.", threadID)
	require.Error(t, err)
}
