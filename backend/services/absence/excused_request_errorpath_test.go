package absence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	notificationsSvc "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// errBoom is the injected repository failure used across the error-path tests.
var errBoom = errors.New("boom")

// --- fake repositories (embed the interface, override only what a test drives)

type fakeReqRepo struct {
	activeModels.ExcusedAbsenceRequestRepository
	create             func(context.Context, *activeModels.ExcusedAbsenceRequest) error
	lockStudent        func(context.Context, int64) error
	listPendingTenant  func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error)
	listPendingStudent func(context.Context, int64) ([]*activeModels.ExcusedAbsenceRequest, error)
	listRecent         func(context.Context, int64, time.Time) ([]*activeModels.ExcusedAbsenceRequest, error)
	findByIDForUpdate  func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error)
	findPending        func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error)
	decide             func(context.Context, int64, string, *string, *int64, bool) error
	findByID           func(context.Context, any) (*activeModels.ExcusedAbsenceRequest, error)
}

func (r *fakeReqRepo) Create(ctx context.Context, req *activeModels.ExcusedAbsenceRequest) error {
	return r.create(ctx, req)
}

// LockStudentRequests is a no-op unless a test drives it — the create-path tests
// only care that the per-student lock is attempted, not its result.
func (r *fakeReqRepo) LockStudentRequests(ctx context.Context, id int64) error {
	if r.lockStudent == nil {
		return nil
	}
	return r.lockStudent(ctx, id)
}
func (r *fakeReqRepo) ListPendingForTenant(ctx context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) {
	return r.listPendingTenant(ctx)
}

// ListPendingForStudent defaults to "no existing pending requests" so the
// create-path overlap/idempotency check runs and falls through to Create.
func (r *fakeReqRepo) ListPendingForStudent(ctx context.Context, id int64) ([]*activeModels.ExcusedAbsenceRequest, error) {
	if r.listPendingStudent == nil {
		return nil, nil
	}
	return r.listPendingStudent(ctx, id)
}
func (r *fakeReqRepo) ListRecentForStudent(ctx context.Context, id int64, since time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
	return r.listRecent(ctx, id, since)
}
func (r *fakeReqRepo) FindByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	return r.findByIDForUpdate(ctx, id)
}
func (r *fakeReqRepo) FindPendingByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	return r.findPending(ctx, id)
}
func (r *fakeReqRepo) Decide(ctx context.Context, id int64, s string, reason *string, by *int64, applied bool) error {
	return r.decide(ctx, id, s, reason, by, applied)
}
func (r *fakeReqRepo) FindByID(ctx context.Context, id any) (*activeModels.ExcusedAbsenceRequest, error) {
	return r.findByID(ctx, id)
}

type fakeStudentRepo struct {
	usersModels.StudentRepository
	findByIDs         func(context.Context, []int64) (map[int64]*usersModels.Student, error)
	findByID          func(context.Context, any) (*usersModels.Student, error)
	findByIDForUpdate func(context.Context, int64) (*usersModels.Student, error)
	update            func(context.Context, *usersModels.Student) error
}

func (r *fakeStudentRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Student, error) {
	return r.findByIDs(ctx, ids)
}
func (r *fakeStudentRepo) FindByID(ctx context.Context, id any) (*usersModels.Student, error) {
	return r.findByID(ctx, id)
}

// FindByIDForUpdate falls back to findByID when no locking stub is configured.
// Decide reads the child under a row lock (the alumnus gate must not decide on
// a state a concurrent graduation can change underneath it), so the plain and
// the locking read are the same fixture for every test that does not care about
// the difference.
func (r *fakeStudentRepo) FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.Student, error) {
	if r.findByIDForUpdate == nil {
		return r.findByID(ctx, id)
	}
	return r.findByIDForUpdate(ctx, id)
}
func (r *fakeStudentRepo) Update(ctx context.Context, s *usersModels.Student) error {
	return r.update(ctx, s)
}

type fakePersonRepo struct {
	usersModels.PersonRepository
	findByIDs func(context.Context, []int64) (map[int64]*usersModels.Person, error)
	findByID  func(context.Context, any) (*usersModels.Person, error)
}

func (r *fakePersonRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Person, error) {
	return r.findByIDs(ctx, ids)
}
func (r *fakePersonRepo) FindByID(ctx context.Context, id any) (*usersModels.Person, error) {
	if r.findByID == nil {
		return nil, nil
	}
	return r.findByID(ctx, id)
}

type fakeStatusRepo struct {
	activeModels.StudentStatusDayRepository
	markCleared func(context.Context, int64, string, []timezone.Date, time.Time, string) error
	upsert      func(context.Context, *activeModels.StudentStatusDay) error
	findActive  func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StudentStatusDay, error)
}

func (r *fakeStatusRepo) MarkClearedForDates(ctx context.Context, id int64, status string, dates []timezone.Date, at time.Time, src string) error {
	return r.markCleared(ctx, id, status, dates, at, src)
}

// FindActiveByStudentAndDateRange backs the approval-time conflict check. It
// defaults to "no active status days" so the approve-path error tests reach the
// injected markCleared/upsert failure instead of a spurious conflict.
func (r *fakeStatusRepo) FindActiveByStudentAndDateRange(ctx context.Context, id int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if r.findActive == nil {
		return nil, nil
	}
	return r.findActive(ctx, id, from, to)
}
func (r *fakeStatusRepo) UpsertReported(ctx context.Context, e *activeModels.StudentStatusDay) error {
	return r.upsert(ctx, e)
}

// newFakeService builds the service over the supplied fakes with no emitter,
// broadcaster or DB — every path is deterministic.
func newFakeService(req *fakeReqRepo, status *fakeStatusRepo, student *fakeStudentRepo, person *fakePersonRepo) absenceSvc.ExcusedAbsenceRequestService {
	return absenceSvc.NewExcusedAbsenceRequestService(req, status, student, person, nil, nil, nil, nil)
}

// okStatusRepo returns a status repo whose writes all succeed.
func okStatusRepo() *fakeStatusRepo {
	return &fakeStatusRepo{
		markCleared: func(context.Context, int64, string, []timezone.Date, time.Time, string) error { return nil },
		upsert:      func(context.Context, *activeModels.StudentStatusDay) error { return nil },
	}
}

func pendingRow(id, studentID int64, dates ...timezone.Date) *activeModels.ExcusedAbsenceRequest {
	r := &activeModels.ExcusedAbsenceRequest{StudentID: studentID, Dates: dates, Status: activeModels.ExcusedRequestStatusPending}
	r.ID = id
	return r
}

func TestErrorPath_CreateRequest_RepoError(t *testing.T) {
	t.Parallel()

	svc := newFakeService(&fakeReqRepo{
		create: func(context.Context, *activeModels.ExcusedAbsenceRequest) error { return errBoom },
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})

	_, err := svc.CreateRequest(adminCtx(), 7, 9, []timezone.Date{timezone.TodayDate()}, "note")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestErrorPath_ListForStudent_RepoErrors(t *testing.T) {
	t.Parallel()

	// Pending list fails.
	svc := newFakeService(&fakeReqRepo{
		listPendingStudent: func(context.Context, int64) ([]*activeModels.ExcusedAbsenceRequest, error) { return nil, errBoom },
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})
	_, err := svc.ListForStudent(adminCtx(), 9, time.Now())
	require.ErrorIs(t, err, errBoom)

	// Pending ok, recent list fails.
	svc = newFakeService(&fakeReqRepo{
		listPendingStudent: func(context.Context, int64) ([]*activeModels.ExcusedAbsenceRequest, error) { return nil, nil },
		listRecent: func(context.Context, int64, time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
			return nil, errBoom
		},
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})
	_, err = svc.ListForStudent(adminCtx(), 9, time.Now())
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_ListPending_RepoErrors(t *testing.T) {
	t.Parallel()

	row := pendingRow(1, 9, timezone.TodayDate())

	// ListPendingForTenant fails.
	svc := newFakeService(&fakeReqRepo{
		listPendingTenant: func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) { return nil, errBoom },
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})
	_, err := svc.ListPending(adminCtx())
	require.Error(t, err)

	// studentRepo.FindByIDs fails.
	svc = newFakeService(&fakeReqRepo{
		listPendingTenant: func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) {
			return []*activeModels.ExcusedAbsenceRequest{row}, nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByIDs: func(context.Context, []int64) (map[int64]*usersModels.Student, error) { return nil, errBoom },
	}, &fakePersonRepo{})
	_, err = svc.ListPending(adminCtx())
	require.Error(t, err)

	// personRepo.FindByIDs fails.
	student := &usersModels.Student{PersonID: 99}
	student.ID = 9
	svc = newFakeService(&fakeReqRepo{
		listPendingTenant: func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) {
			return []*activeModels.ExcusedAbsenceRequest{row}, nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByIDs: func(context.Context, []int64) (map[int64]*usersModels.Student, error) {
			return map[int64]*usersModels.Student{9: student}, nil
		},
	}, &fakePersonRepo{
		findByIDs: func(context.Context, []int64) (map[int64]*usersModels.Person, error) { return nil, errBoom },
	})
	_, err = svc.ListPending(adminCtx())
	require.Error(t, err)
}

func TestErrorPath_PendingByStudentForDate_RepoErrors(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	row := pendingRow(1, 9, today)

	// ListPendingForTenant fails.
	svc := newFakeService(&fakeReqRepo{
		listPendingTenant: func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) { return nil, errBoom },
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})
	_, err := svc.PendingByStudentForDate(adminCtx(), today)
	require.ErrorIs(t, err, errBoom)

	// studentRepo.FindByIDs fails after a covering request is found.
	svc = newFakeService(&fakeReqRepo{
		listPendingTenant: func(context.Context) ([]*activeModels.ExcusedAbsenceRequest, error) {
			return []*activeModels.ExcusedAbsenceRequest{row}, nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByIDs: func(context.Context, []int64) (map[int64]*usersModels.Student, error) { return nil, errBoom },
	}, &fakePersonRepo{})
	_, err = svc.PendingByStudentForDate(adminCtx(), today)
	require.Error(t, err)
}

func TestErrorPath_WithdrawRequest_DecideError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate())
	row.SubmittedBy = 3
	svc := newFakeService(&fakeReqRepo{
		findByIDForUpdate: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
		decide:            func(context.Context, int64, string, *string, *int64, bool) error { return errBoom },
	}, okStatusRepo(), &fakeStudentRepo{}, &fakePersonRepo{})

	_, err := svc.WithdrawRequest(adminCtx(), 5, 9, 3)
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_Decide_StudentLoadError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate().AddDays(4))
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
	}, okStatusRepo(), &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return nil, errBoom },
	}, &fakePersonRepo{})

	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

func TestErrorPath_Decide_ApplyError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate().AddDays(4))
	student := &usersModels.Student{}
	student.ID = 9
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
	}, &fakeStatusRepo{
		markCleared: func(context.Context, int64, string, []timezone.Date, time.Time, string) error { return errBoom },
	}, &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return student, nil },
	}, &fakePersonRepo{})

	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_Decide_ApplyUpsertError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate().AddDays(4))
	student := &usersModels.Student{}
	student.ID = 9
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
	}, &fakeStatusRepo{
		markCleared: func(context.Context, int64, string, []timezone.Date, time.Time, string) error { return nil },
		upsert:      func(context.Context, *activeModels.StudentStatusDay) error { return errBoom },
	}, &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return student, nil },
	}, &fakePersonRepo{})

	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_Decide_ApplyTodayBranchErrors(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	student := &usersModels.Student{}
	student.ID = 9

	// FindByIDForUpdate (today branch) fails. The authorization gate takes the
	// same locked read first, so only the SECOND call is the today branch's.
	locked := 0
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) {
			return pendingRow(5, 9, today), nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return student, nil },
		findByIDForUpdate: func(context.Context, int64) (*usersModels.Student, error) {
			locked++
			if locked == 1 {
				return student, nil
			}
			return nil, errBoom
		},
	}, &fakePersonRepo{})
	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.ErrorIs(t, err, errBoom)

	// Update (today branch) fails.
	svc = newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) {
			return pendingRow(5, 9, today), nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByID:          func(context.Context, any) (*usersModels.Student, error) { return student, nil },
		findByIDForUpdate: func(context.Context, int64) (*usersModels.Student, error) { return student, nil },
		update:            func(context.Context, *usersModels.Student) error { return errBoom },
	}, &fakePersonRepo{})
	_, err = svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_Decide_RepoDecideError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate().AddDays(4))
	student := &usersModels.Student{}
	student.ID = 9
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
		decide:      func(context.Context, int64, string, *string, *int64, bool) error { return errBoom },
	}, okStatusRepo(), &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return student, nil },
	}, &fakePersonRepo{})

	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.ErrorIs(t, err, errBoom)
}

func TestErrorPath_Decide_ReloadError(t *testing.T) {
	t.Parallel()

	row := pendingRow(5, 9, timezone.TodayDate().AddDays(4))
	student := &usersModels.Student{}
	student.ID = 9
	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) { return row, nil },
		decide:      func(context.Context, int64, string, *string, *int64, bool) error { return nil },
		findByID:    func(context.Context, any) (*activeModels.ExcusedAbsenceRequest, error) { return nil, errBoom },
	}, okStatusRepo(), &fakeStudentRepo{
		findByID: func(context.Context, any) (*usersModels.Student, error) { return student, nil },
	}, &fakePersonRepo{})

	// Reject path (no apply, no today branch) so the failure is isolated to the reload.
	_, err := svc.Decide(adminCtx(), absenceSvc.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "abgelehnt"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

type recordingAbsenceNotifier struct {
	reports []notificationsSvc.AbsenceReport
}

func (n *recordingAbsenceNotifier) NotifyAbsenceReported(_ context.Context, report notificationsSvc.AbsenceReport) {
	n.reports = append(n.reports, report)
}

func TestDecide_ApprovalNotifiesAfterCommit(t *testing.T) {
	t.Parallel()

	const (
		tenantID  int64 = 31
		studentID int64 = 9
		reviewer  int64 = 77
		submitter int64 = 88
	)
	today := timezone.TodayDate()
	row := pendingRow(5, studentID, today)
	row.TenantID = tenantID
	row.SubmittedBy = submitter
	student := &usersModels.Student{}
	student.ID = studentID

	svc := newFakeService(&fakeReqRepo{
		findPending: func(context.Context, int64) (*activeModels.ExcusedAbsenceRequest, error) {
			return row, nil
		},
		decide: func(context.Context, int64, string, *string, *int64, bool) error {
			return nil
		},
		findByID: func(context.Context, any) (*activeModels.ExcusedAbsenceRequest, error) {
			return row, nil
		},
	}, okStatusRepo(), &fakeStudentRepo{
		findByID:          func(context.Context, any) (*usersModels.Student, error) { return student, nil },
		findByIDForUpdate: func(context.Context, int64) (*usersModels.Student, error) { return student, nil },
		update:            func(context.Context, *usersModels.Student) error { return nil },
	}, &fakePersonRepo{})
	notifier := &recordingAbsenceNotifier{}
	svc.(absenceSvc.AbsenceNotifierSetter).SetAbsenceNotifier(notifier)

	baseCtx := tenant.WithTenantID(adminCtx(), tenantID)
	ctx, commit := tenant.WithAfterCommitHooksForTest(baseCtx)
	_, err := svc.Decide(ctx, absenceSvc.ExcusedRequestDecideInput{
		RequestID:  5,
		Approve:    true,
		ReviewedBy: reviewer,
	})
	require.NoError(t, err)
	assert.Empty(t, notifier.reports, "the notification must not run before the decision commits")

	commit()
	require.Len(t, notifier.reports, 1)
	report := notifier.reports[0]
	assert.Equal(t, tenantID, report.TenantID)
	assert.Equal(t, []int64{studentID}, report.StudentIDs)
	assert.Equal(t, activeModels.StudentStatusDayExcused, report.Status)
	assert.Equal(t, []timezone.Date{today}, report.Dates)
	assert.True(t, report.FromParent)
	assert.Equal(t, reviewer, report.ActorAccountID)
	assert.Equal(t, []int64{submitter}, report.ExcludedAccountIDs)
}
