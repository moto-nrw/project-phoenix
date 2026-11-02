package messaging_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/moto-nrw/project-phoenix/api"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Fault-injection tests: every repository call the staff messaging service makes
	// must propagate a transient failure as a wrapped server error (→ 500, retryable),
	// NOT silently swallow it and hand the client an empty/200 result. These use fake
	// repos that return errors, so they pin the error-mapping branches the happy-path
	// DB tests cannot reach. The forbidden/not-found/disabled sentinels stay distinct
	// from these server faults — that distinction is the whole point of the mapping.

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"

	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var errBoom = errors.New("transient DB failure")

// --- fake repos (only the called methods are overridden) ----------------

type fakeThreadRepo struct {
	messaging.ParentMessageThreadRepository
	thread           *messaging.ParentMessageThread
	findErr          error
	guardians        []*messaging.MessageableGuardian
	listGuardiansErr error
	getOrCreateErr   error
}

func (f *fakeThreadRepo) LockForMessageAppend(context.Context, int64) error { return nil }

func (f *fakeThreadRepo) FindByID(context.Context, int64) (*messaging.ParentMessageThread, error) {
	return f.thread, f.findErr
}

func (f *fakeThreadRepo) ListGuardiansForStudent(context.Context, int64) ([]*messaging.MessageableGuardian, error) {
	return f.guardians, f.listGuardiansErr
}

func (f *fakeThreadRepo) GetOrCreate(context.Context, int64, int64, int64, string) (*messaging.ParentMessageThread, error) {
	if f.getOrCreateErr != nil {
		return nil, f.getOrCreateErr
	}
	return f.thread, nil
}

type fakeMessageRepo struct {
	messaging.ParentMessageRepository
	messages   []*messaging.ParentMessage
	listErr    error
	createErr  error
	createSeen bool
}

func (f *fakeMessageRepo) Create(_ context.Context, m *messaging.ParentMessage) error {
	f.createSeen = true
	if f.createErr != nil {
		return f.createErr
	}
	m.ID = 1
	return nil
}

func (f *fakeMessageRepo) ListByThread(context.Context, int64, int) ([]*messaging.ParentMessage, error) {
	return f.messages, f.listErr
}

type fakeReadRepo struct {
	messaging.ParentMessageReadRepository
	inbox       []*messaging.InboxThread
	inboxErr    error
	unread      int
	unreadErr   error
	studentRows []*messaging.InboxThread
	studentErr  error
}

func (f *fakeReadRepo) ListInboxForStaff(context.Context, int64, messaging.StaffReach, bool) ([]*messaging.InboxThread, error) {
	return f.inbox, f.inboxErr
}

func (f *fakeReadRepo) UnreadMessageCountForStaff(context.Context, int64, messaging.StaffReach) (int, error) {
	return f.unread, f.unreadErr
}

func (f *fakeReadRepo) ListThreadsForStudent(context.Context, int64, int64, messaging.StaffReach) ([]*messaging.InboxThread, error) {
	return f.studentRows, f.studentErr
}

// fakePersons satisfies PersonLookup for the read-access load + staff-name
// resolution. GetStudentByID returns a real student so the admin read check
// passes; FindByAccountID returns nil so resolveStaffName uses its default.
type fakePersons struct {
	messaging.PersonLookup
}

func (fakePersons) FindParentMessageStudent(context.Context, int64) (*communication.ParentMessageStudent, error) {
	return &communication.ParentMessageStudent{AuthorizationStudent: true}, nil
}
func (fakePersons) FindStaffSenderName(context.Context, int64) (*communication.StaffSenderName, error) {
	return nil, nil
}

func errSvc(tr messaging.ParentMessageThreadRepository, mr messaging.ParentMessageRepository, rr messaging.ParentMessageReadRepository) *messaging.Service {
	return messaging.NewService(messaging.Config{
		Leaders:     communicationtest.Leaders{},
		UserContext: owners.StaffStudentReads{},
		Session:     api.CommunicationSession{},
		ThreadRepo:  tr,
		MessageRepo: mr,
		ReadRepo:    rr,
		Persons:     fakePersons{},
		Settings:    settingsValues.NewParentMessagingValues(stubSettings{messagingEnabled: true}),
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		Logger:      slog.Default(),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
}

// errCtx is an admin context (the read scope is satisfied without a userContext).
func errCtx(tb testing.TB) context.Context { return adminCtx(tb, 1) }

func TestListInbox_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{inboxErr: errBoom})
	_, err := testpkg.TenantQuery1(t, errCtx(t), testpkg.SetupTestDB(t), svc.ListInbox, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list inbox")
}

func TestUnreadMessageCount_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{unreadErr: errBoom})
	_, err := testpkg.TenantQuery0(t, errCtx(t), testpkg.SetupTestDB(t), svc.UnreadMessageCount)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unread count")
}

func TestGetThread_FindThreadErrorIsServerFaultNotNotFound(t *testing.T) {
	t.Parallel()

	// A transient FindByID failure must be a server error, NOT ErrThreadNotFound
	// (which would wrongly tell the client the thread does not exist).
	svc := errSvc(&fakeThreadRepo{findErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := testpkg.TenantQuery1(t, errCtx(t), testpkg.SetupTestDB(t), svc.GetThread, 5)
	require.Error(t, err)
	assert.NotErrorIs(t, err, messaging.ErrThreadNotFound)
}

func TestGetThread_ListMessagesErrorPropagates(t *testing.T) {
	t.Parallel()

	tr := &fakeThreadRepo{thread: &messaging.ParentMessageThread{StudentID: 7}}
	tr.thread.ID = 5
	svc := errSvc(tr, &fakeMessageRepo{listErr: errBoom}, &fakeReadRepo{})
	_, err := testpkg.TenantQuery1(t, errCtx(t), testpkg.SetupTestDB(t), svc.GetThread, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list messages")
}

func TestListStudentThreads_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{studentErr: errBoom})
	_, err := testpkg.TenantQuery1(t, errCtx(t), testpkg.SetupTestDB(t), svc.ListStudentThreads, 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list student threads")
}

func TestListGuardians_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := errSvc(&fakeThreadRepo{listGuardiansErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := testpkg.TenantQuery1(t, errCtx(t), testpkg.SetupTestDB(t), teamGuardians(svc), 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list guardians")
}

func TestStartThread_GuardianLookupErrorPropagates(t *testing.T) {
	t.Parallel()

	// authorizeThreadParticipants → ListGuardiansForStudent fails → server error,
	// not ErrInvalidGuardian (which would wrongly imply the recipient is invalid).
	svc := errSvc(&fakeThreadRepo{listGuardiansErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := testpkg.TenantQuery4(t, errCtx(t), testpkg.SetupTestDB(t), svc.StartThread, 7, 2, "Hallo", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, messaging.ErrInvalidGuardian)
}

func TestStartThread_GetOrCreateErrorPropagates(t *testing.T) {
	t.Parallel()

	tr := &fakeThreadRepo{
		guardians:      []*messaging.MessageableGuardian{{AccountID: 2}},
		getOrCreateErr: errBoom,
	}
	svc := errSvc(tr, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := testpkg.TenantQuery4(t, errCtx(t), testpkg.SetupTestDB(t), svc.StartThread, 7, 2, "Hallo", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get-or-create thread")
}

func TestPostMessage_NoBroadcastOnAppendFailure(t *testing.T) {
	t.Parallel()

	// A failed message insert must surface an error AND must not have fired the SSE
	// fan-out (which would wake clients for a message that never persisted).
	tr := &fakeThreadRepo{
		thread:    &messaging.ParentMessageThread{StudentID: 7},
		guardians: []*messaging.MessageableGuardian{{AccountID: 2}},
	}
	tr.thread.ID = 5
	tr.thread.GuardianAccountID = 2
	bc := testpkg.NewRecordingBroadcaster()
	svc := messaging.NewService(messaging.Config{
		Leaders:     communicationtest.Leaders{},
		UserContext: owners.StaffStudentReads{},
		Session:     api.CommunicationSession{},
		ThreadRepo:  tr, MessageRepo: &fakeMessageRepo{createErr: errBoom}, ReadRepo: &fakeReadRepo{},
		Persons: fakePersons{}, Settings: settingsValues.NewParentMessagingValues(stubSettings{messagingEnabled: true}), Broadcaster: bc, Logger: slog.Default(),
	}, testpkg.TenantTransactionRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	_, err := testpkg.TenantQuery3(t, errCtx(t), testpkg.SetupTestDB(t), svc.PostMessage, 5, "Hallo", 0)
	require.Error(t, err)
	assert.Equal(t, 0, parentEventCount(bc, realtime.EventParentMessage), "a failed append must not broadcast")
}
