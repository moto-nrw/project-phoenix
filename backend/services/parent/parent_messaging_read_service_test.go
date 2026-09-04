package parent_test

// Integration tests for the parent-side messaging READ/LIST/COUNT paths
// (parent_messaging_service.go): the cross-tenant inbox, the per-child thread
// list, the unread badge, and the single-conversation view that marks read. The
// write-path guards (empty/too-long/ownership/permission/disabled) are covered in
// parent_write_service_test.go; this file pins the happy paths, the empty view,
// the disabled-school unread suppression, and guardian-name resolution.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	timetabletest "github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildReadService wires the full parent messaging stack including the guardian
// profile repo (so resolveGuardianName resolves a real name) and returns the
// service, broadcaster, db, and repo factory.
func buildReadService(t *testing.T, enabled bool) (parentService.Service, *testpkg.RecordingBroadcaster, *bun.DB, *repositories.Factory) {
	return buildReadServiceWithNotifier(t, enabled, nil)
}

func buildReadServiceWithNotifier(t *testing.T, enabled bool, notifier notificationsSvc.StaffParentMessageNotifier) (parentService.Service, *testpkg.RecordingBroadcaster, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// Child names come from the People Directory composition (#2661); bind
	// it before the school projections, as the service graph does.
	repos, err := repositories.NewFactoryWithPeopleDirectory(db, timetabletest.New(t, db))
	require.NoError(t, err)
	organizationTenancy, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	repos.BindOrganizationTenancy(organizationTenancy)
	bc := testpkg.NewRecordingBroadcaster()
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentSickNoteEnabled: true,
				configModels.KeyParentNotesEnabled:    enabled,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		Broadcaster:           bc,
		MessageThreadRepo:     repos.ParentMessageThread,
		MessageRepo:           repos.ParentMessage,
		MessageReadRepo:       repos.ParentMessageRead,
		ParentMessageNotifier: notifier,
		DB:                    db,
		Logger:                slog.Default(),
	})
	return svc, bc, db, repos
}

type recordingStaffParentMessageNotifier struct {
	reports []notificationsSvc.StaffParentMessageReport
}

func (n *recordingStaffParentMessageNotifier) NotifyStaffParentMessage(_ context.Context, report notificationsSvc.StaffParentMessageReport) error {
	n.reports = append(n.reports, report)
	return nil
}

// seedStaffReply inserts a staff-authored message into the guardian's child
// thread (creating the thread), simulating an OGS reply that is unread to the
// guardian. Returns the thread id and the staff account id (caller cleans it up).
func seedStaffReply(t *testing.T, db *bun.DB, repos *repositories.Factory, chain testpkg.ParentChain, body string) (int64, int64) {
	t.Helper()
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	ctx := testpkg.Ctx(t)
	thread, err := repos.ParentMessageThread.GetOrCreate(ctx, chain.TenantID, chain.StudentID, chain.AccountID)
	require.NoError(t, err)
	m := &usersModels.ParentMessage{
		ThreadID:        thread.ID,
		StudentID:       chain.StudentID,
		SenderAccountID: staffAccount.ID,
		SenderKind:      usersModels.ParentMessageSenderStaff,
		SenderName:      "Olivia Berg",
		Body:            body,
	}
	m.SetTenantID(chain.TenantID)
	// AppendMessage also touches the thread's last-activity, so it surfaces in the
	// guardian's thread list (empty conversations stay hidden).
	require.NoError(t, parentmessaging.AppendMessage(ctx, repos.ParentMessage, repos.ParentMessageThread, m))
	return thread.ID, staffAccount.ID
}

// --- GetChildConversation -----------------------------------------------

func TestGetChildConversation_EmptyViewWhenNoThread(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.GetChildConversation(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Zero(t, view.ThreadID, "no conversation yet → ThreadID 0 so the chat opens ready to write")
	assert.Empty(t, view.Messages)
	assert.Equal(t, chain.StudentID, view.StudentID)
	assert.Equal(t, "Felix Schneider", view.StudentName)
	assert.NotEmpty(t, view.SchoolName, "the header still carries the school name")
}

func TestGetChildConversation_ReturnsHistoryMarksReadAndBroadcasts(t *testing.T) {
	t.Parallel()

	svc, bc, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, _ = seedStaffReply(t, db, repos, chain, "Bitte Jacke mitgeben")

	// Before reading, the staff reply is unread to the guardian.
	before, err := svc.UnreadMessageCount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, before)

	bc.Reset()
	view, err := svc.GetChildConversation(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, view.Messages, 1)
	assert.Equal(t, "Bitte Jacke mitgeben", view.Messages[0].Body)
	assert.Positive(t, view.ThreadID)

	// Reading it advances the guardian cursor → unread clears.
	after, err := svc.UnreadMessageCount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, after, "GetChildConversation marks the staff reply read")
}

// --- ListMessageThreads / ListChildThreads ------------------------------

func TestListMessageThreads_ReturnsConversationAfterFirstMessage(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// No message yet → no thread in the list (empty conversations stay hidden).
	threads, err := svc.ListMessageThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	assert.Empty(t, threads)

	// Guardian writes → the conversation appears with the child + school header.
	_, err = svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "Hallo OGS")
	require.NoError(t, err)

	threads, err = svc.ListMessageThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.Equal(t, chain.StudentID, threads[0].StudentID)
	assert.Equal(t, "Felix Schneider", threads[0].StudentName)
	assert.NotEmpty(t, threads[0].SchoolName)
}

func TestListMessageThreads_ReportsWhetherStaffReadTheLastGuardianMessage(t *testing.T) {
	t.Parallel()

	svc, _, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "Hallo OGS")
	require.NoError(t, err)
	require.Len(t, view.Messages, 1)

	threads, err := svc.ListMessageThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.False(t, threads[0].LastMessageReadByStaff)

	_, staffAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, chain.TenantID, "Olivia", "Berg")

	message := view.Messages[0]
	tenantCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), chain.TenantID)
	advanced, err := repos.ParentMessageRead.MarkReadUpTo(
		tenantCtx,
		chain.TenantID,
		view.ThreadID,
		staffAccount.ID,
		message.CreatedAt,
		message.ID,
	)
	require.NoError(t, err)
	require.True(t, advanced)

	threads, err = svc.ListMessageThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	assert.True(t, threads[0].LastMessageReadByStaff)
}

func TestListChildThreads_FiltersToOneChild(t *testing.T) {
	t.Parallel()

	svc, _, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedStaffReply(t, db, repos, chain, "Hallo")

	rows, err := svc.ListChildThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, chain.StudentID, rows[0].StudentID)
	assert.Equal(t, 1, rows[0].UnreadCount, "the staff reply is unread to the guardian")
}

func TestListChildThreads_NotOwnedDenied(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.ListChildThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// --- UnreadMessageCount -------------------------------------------------

func TestUnreadMessageCount_ZeroWhenSchoolDisabled(t *testing.T) {
	t.Parallel()

	// Feature OFF: the guardian can still read history, but the badge goes dark.
	svc, _, db, repos := buildReadService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedStaffReply(t, db, repos, chain, "Hallo")

	count, err := svc.UnreadMessageCount(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a disabled school's unread badge is suppressed")

	// The per-row pill is suppressed too, so the child-thread row agrees with the badge.
	rows, err := svc.ListChildThreads(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 0, rows[0].UnreadCount, "disabled school suppresses the per-row unread pill")
}

func TestParentMessaging_AccountIDMustBePositive(t *testing.T) {
	t.Parallel()

	svc, _, _, _ := buildReadService(t, true)
	_, err := svc.ListMessageThreads(testpkg.WithPackageTenantRuntime(context.Background()), 0)
	require.Error(t, err)
	_, err = svc.UnreadMessageCount(testpkg.WithPackageTenantRuntime(context.Background()), -1)
	require.Error(t, err)
}

// TestPostChildMessage_DenormalizesGuardianName: with the guardian profile repo
// wired, the persisted message carries the guardian's real display name (resolved
// inside the child's tenant tx), not the "Elternteil" fallback.
func TestPostChildMessage_DenormalizesGuardianName(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "Hallo OGS")
	require.NoError(t, err)
	require.Len(t, view.Messages, 1)
	assert.Equal(t, "Sabine Schneider", view.Messages[0].SenderName,
		"the guardian's tenant-scoped profile name is denormalized onto the message")
}

func TestPostChildMessage_NotifiesStaffAfterCommit(t *testing.T) {
	t.Parallel()

	notifier := &recordingStaffParentMessageNotifier{}
	svc, _, db, _ := buildReadServiceWithNotifier(t, true, notifier)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "Hallo OGS")
	require.NoError(t, err)
	require.Len(t, notifier.reports, 1)
	assert.Equal(t, notificationsSvc.StaffParentMessageReport{
		TenantID:       chain.TenantID,
		ThreadID:       view.ThreadID,
		MessageID:      view.Messages[0].ID,
		StudentID:      chain.StudentID,
		ActorAccountID: chain.AccountID,
	}, notifier.reports[0])
}
