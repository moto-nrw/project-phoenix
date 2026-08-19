package messaging_test

// Fault-injection tests: every repository call the staff messaging service makes
// must propagate a transient failure as a wrapped server error (→ 500, retryable),
// NOT silently swallow it and hand the client an empty/200 result. These use fake
// repos that return errors, so they pin the error-mapping branches the happy-path
// DB tests cannot reach. The forbidden/not-found/disabled sentinels stay distinct
// from these server faults — that distinction is the whole point of the mapping.

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var errBoom = errors.New("transient DB failure")

// --- fake repos (only the called methods are overridden) ----------------

type fakeThreadRepo struct {
	usersModels.ParentMessageThreadRepository
	thread           *usersModels.ParentMessageThread
	findErr          error
	guardians        []*usersModels.MessageableGuardian
	listGuardiansErr error
	getOrCreateErr   error
}

func (f *fakeThreadRepo) LockForMessageAppend(context.Context, int64) error { return nil }

func (f *fakeThreadRepo) FindByID(context.Context, int64) (*usersModels.ParentMessageThread, error) {
	return f.thread, f.findErr
}

func (f *fakeThreadRepo) ListGuardiansForStudent(context.Context, int64) ([]*usersModels.MessageableGuardian, error) {
	return f.guardians, f.listGuardiansErr
}

func (f *fakeThreadRepo) GetOrCreate(context.Context, int64, int64, int64) (*usersModels.ParentMessageThread, error) {
	if f.getOrCreateErr != nil {
		return nil, f.getOrCreateErr
	}
	return f.thread, nil
}

type fakeMessageRepo struct {
	usersModels.ParentMessageRepository
	messages   []*usersModels.ParentMessage
	listErr    error
	createErr  error
	createSeen bool
}

func (f *fakeMessageRepo) Create(_ context.Context, m *usersModels.ParentMessage) error {
	f.createSeen = true
	if f.createErr != nil {
		return f.createErr
	}
	m.ID = 1
	return nil
}

func (f *fakeMessageRepo) ListByThread(context.Context, int64, int) ([]*usersModels.ParentMessage, error) {
	return f.messages, f.listErr
}

type fakeReadRepo struct {
	usersModels.ParentMessageReadRepository
	inbox       []*usersModels.InboxThread
	inboxErr    error
	unread      int
	unreadErr   error
	studentRows []*usersModels.InboxThread
	studentErr  error
}

func (f *fakeReadRepo) ListInboxForStaff(context.Context, int64, bool, bool) ([]*usersModels.InboxThread, error) {
	return f.inbox, f.inboxErr
}

func (f *fakeReadRepo) UnreadMessageCountForStaff(context.Context, int64, bool) (int, error) {
	return f.unread, f.unreadErr
}

func (f *fakeReadRepo) ListThreadsForStudent(context.Context, int64, int64) ([]*usersModels.InboxThread, error) {
	return f.studentRows, f.studentErr
}

// fakePersons satisfies PersonService for the read-access load + staff-name
// resolution. GetStudentByID returns a real student so the admin read check
// passes; FindByAccountID returns nil so resolveStaffName uses its default.
type fakePersons struct {
	usersService.PersonService
}

func (fakePersons) GetStudentByID(context.Context, int64) (*usersModels.Student, error) {
	var gid int64 = 1 // fake in-memory student, not a DB row
	return &usersModels.Student{GroupID: &gid}, nil
}

func (fakePersons) FindByAccountID(context.Context, int64) (*usersModels.Person, error) {
	return nil, nil
}

func errSvc(tr usersModels.ParentMessageThreadRepository, mr usersModels.ParentMessageRepository, rr usersModels.ParentMessageReadRepository) *messaging.Service {
	return messaging.NewService(messaging.Config{
		ThreadRepo:  tr,
		MessageRepo: mr,
		ReadRepo:    rr,
		Persons:     fakePersons{},
		Settings:    stubSettings{messagingEnabled: true},
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		Logger:      slog.Default(),
	})
}

// errCtx is an admin context (the read scope is satisfied without a userContext).
func errCtx(tb testing.TB) context.Context { return adminCtx(tb, 1) }

func TestListInbox_RepoErrorPropagates(t *testing.T) {
	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{inboxErr: errBoom})
	_, err := svc.ListInbox(errCtx(t), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list inbox")
}

func TestUnreadMessageCount_RepoErrorPropagates(t *testing.T) {
	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{unreadErr: errBoom})
	_, err := svc.UnreadMessageCount(errCtx(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unread count")
}

func TestGetThread_FindThreadErrorIsServerFaultNotNotFound(t *testing.T) {
	// A transient FindByID failure must be a server error, NOT ErrThreadNotFound
	// (which would wrongly tell the client the thread does not exist).
	svc := errSvc(&fakeThreadRepo{findErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := svc.GetThread(errCtx(t), 5)
	require.Error(t, err)
	assert.NotErrorIs(t, err, messaging.ErrThreadNotFound)
}

func TestGetThread_ListMessagesErrorPropagates(t *testing.T) {
	tr := &fakeThreadRepo{thread: &usersModels.ParentMessageThread{StudentID: 7}}
	tr.thread.ID = 5
	svc := errSvc(tr, &fakeMessageRepo{listErr: errBoom}, &fakeReadRepo{})
	_, err := svc.GetThread(errCtx(t), 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list messages")
}

func TestListStudentThreads_RepoErrorPropagates(t *testing.T) {
	svc := errSvc(&fakeThreadRepo{}, &fakeMessageRepo{}, &fakeReadRepo{studentErr: errBoom})
	_, err := svc.ListStudentThreads(errCtx(t), 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list student threads")
}

func TestListGuardians_RepoErrorPropagates(t *testing.T) {
	svc := errSvc(&fakeThreadRepo{listGuardiansErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := svc.ListGuardians(errCtx(t), 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list guardians")
}

func TestStartThread_GuardianLookupErrorPropagates(t *testing.T) {
	// authorizeThreadParticipants → ListGuardiansForStudent fails → server error,
	// not ErrInvalidGuardian (which would wrongly imply the recipient is invalid).
	svc := errSvc(&fakeThreadRepo{listGuardiansErr: errBoom}, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := svc.StartThread(errCtx(t), 7, 2, "Hallo")
	require.Error(t, err)
	assert.NotErrorIs(t, err, messaging.ErrInvalidGuardian)
}

func TestStartThread_GetOrCreateErrorPropagates(t *testing.T) {
	tr := &fakeThreadRepo{
		guardians:      []*usersModels.MessageableGuardian{{AccountID: 2}},
		getOrCreateErr: errBoom,
	}
	svc := errSvc(tr, &fakeMessageRepo{}, &fakeReadRepo{})
	_, err := svc.StartThread(errCtx(t), 7, 2, "Hallo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get-or-create thread")
}

func TestPostMessage_NoBroadcastOnAppendFailure(t *testing.T) {
	// A failed message insert must surface an error AND must not have fired the SSE
	// fan-out (which would wake clients for a message that never persisted).
	tr := &fakeThreadRepo{
		thread:    &usersModels.ParentMessageThread{StudentID: 7},
		guardians: []*usersModels.MessageableGuardian{{AccountID: 2}},
	}
	tr.thread.ID = 5
	tr.thread.GuardianAccountID = 2
	bc := testpkg.NewRecordingBroadcaster()
	svc := messaging.NewService(messaging.Config{
		ThreadRepo: tr, MessageRepo: &fakeMessageRepo{createErr: errBoom}, ReadRepo: &fakeReadRepo{},
		Persons: fakePersons{}, Settings: stubSettings{messagingEnabled: true}, Broadcaster: bc, Logger: slog.Default(),
	})
	_, err := svc.PostMessage(errCtx(t), 5, "Hallo", 0)
	require.Error(t, err)
	assert.Equal(t, 0, parentEventCount(bc, realtime.EventParentMessage), "a failed append must not broadcast")
}
