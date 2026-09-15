package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// errBoom is the injected failure used across the error-path tests.
var errBoom = errors.New("boom")

const (
	fakeTenantID   int64 = 31
	fakeStudentID  int64 = 9
	fakePersonID   int64 = 99
	fakeGuardian   int64 = 88
	fakeReviewer   int64 = 77
	otherStudentID int64 = 10
	endedStudentID int64 = 11
	fakeGroupID    int64 = 5
)

var (
	fakeToday    = careplan.Date("2026-08-24")
	fakeTomorrow = careplan.Date("2026-08-25")
	fakeNextWeek = careplan.Date("2026-08-31")
	fakePast     = careplan.Date("2026-08-20")
)

// --- fake ports (embed the interface, override only what a test drives)

type fakeCarePlan struct {
	careplan.Capability
	findRequest    func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error)
	findPending    func(context.Context, int64) (careplan.ExcusedAbsenceRequest, error)
	listRequests   func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error)
	createRequest  func(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error)
	decide         func(context.Context, careplan.ExcusedAbsenceDecision) error
	redecide       func(context.Context, careplan.ExcusedAbsenceDecision) error
	updatePending  func(context.Context, int64, []careplan.Date, string, string) error
	listStatusDays func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
	clearDays      func(context.Context, int64, string, []careplan.Date, time.Time, string) error
	upsertDay      func(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, error)
	listPickups    func(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
}

func (f *fakeCarePlan) LockStudentAndExceptionDay(context.Context, int64, string) error { return nil }
func (f *fakeCarePlan) LockExcusedAbsenceRequests(context.Context, int64) error         { return nil }

func (f *fakeCarePlan) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, error) {
	if f.findRequest == nil {
		return careplan.ExcusedAbsenceRequest{}, careplan.ErrExcusedRequestNotFound
	}
	return f.findRequest(ctx, id, lock)
}

func (f *fakeCarePlan) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (careplan.ExcusedAbsenceRequest, error) {
	if f.findPending == nil {
		return careplan.ExcusedAbsenceRequest{}, careplan.ErrExcusedRequestNotFound
	}
	return f.findPending(ctx, id)
}

func (f *fakeCarePlan) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	if f.listRequests == nil {
		return nil, nil
	}
	return f.listRequests(ctx, filter)
}

func (f *fakeCarePlan) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
	if f.createRequest == nil {
		value.ID, value.TenantID = 1, fakeTenantID
		return value, nil
	}
	return f.createRequest(ctx, value)
}

func (f *fakeCarePlan) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	if f.decide == nil {
		return nil
	}
	return f.decide(ctx, value)
}

func (f *fakeCarePlan) RedecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision) error {
	if f.redecide == nil {
		return nil
	}
	return f.redecide(ctx, value)
}

func (f *fakeCarePlan) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, status string) error {
	if f.updatePending == nil {
		return nil
	}
	return f.updatePending(ctx, id, dates, note, status)
}

func (f *fakeCarePlan) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
	if f.listStatusDays == nil {
		return nil, nil
	}
	return f.listStatusDays(ctx, filter)
}

func (f *fakeCarePlan) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, at time.Time, source string) error {
	if f.clearDays == nil {
		return nil
	}
	return f.clearDays(ctx, studentID, status, dates, at, source)
}

func (f *fakeCarePlan) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
	if f.upsertDay == nil {
		return value, nil
	}
	return f.upsertDay(ctx, value)
}

func (f *fakeCarePlan) ListPickupExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	if f.listPickups == nil {
		return nil, nil
	}
	return f.listPickups(ctx, filter)
}

type fakeStudents struct {
	students      func(context.Context, []int64) (map[int64]ports.ReviewStudent, error)
	lock          func(context.Context, int64) (ports.ReviewStudent, error)
	persons       func(context.Context, []int64) (map[int64]ports.PersonName, error)
	reviewers     func(context.Context, []int64) (map[int64]ports.PersonName, error)
	liveFlags     func(context.Context, int64, string, time.Time) error
	liveFlagCalls int
}

func activeStudent() ports.ReviewStudent {
	return ports.ReviewStudent{ID: fakeStudentID, PersonID: fakePersonID}
}

func (f *fakeStudents) FindStudents(ctx context.Context, ids []int64) (map[int64]ports.ReviewStudent, error) {
	if f.students == nil {
		return map[int64]ports.ReviewStudent{fakeStudentID: activeStudent()}, nil
	}
	return f.students(ctx, ids)
}

func (f *fakeStudents) LockStudent(ctx context.Context, id int64) (ports.ReviewStudent, error) {
	if f.lock == nil {
		return activeStudent(), nil
	}
	return f.lock(ctx, id)
}

func (f *fakeStudents) PersonNames(ctx context.Context, ids []int64) (map[int64]ports.PersonName, error) {
	if f.persons == nil {
		return map[int64]ports.PersonName{fakePersonID: {FirstName: "Felix", LastName: "Schneider"}}, nil
	}
	return f.persons(ctx, ids)
}

func (f *fakeStudents) ReviewerNames(ctx context.Context, ids []int64) (map[int64]ports.PersonName, error) {
	if f.reviewers == nil {
		return map[int64]ports.PersonName{}, nil
	}
	return f.reviewers(ctx, ids)
}

func (f *fakeStudents) SetLiveAbsenceFlags(ctx context.Context, studentID int64, status string, at time.Time) error {
	f.liveFlagCalls++
	if f.liveFlags == nil {
		return nil
	}
	return f.liveFlags(ctx, studentID, status, at)
}

// fakeHooks queues after-commit callbacks so a test can prove what ran inside
// the transaction and what waited for the commit.
// The queue is a channel, so the fake never rewires a dependency field after
// construction.
type fakeHooks struct{ queued chan func() }

func newFakeHooks() *fakeHooks { return &fakeHooks{queued: make(chan func(), 64)} }

func (h *fakeHooks) TenantID(context.Context) int64           { return fakeTenantID }
func (h *fakeHooks) AfterCommit(_ context.Context, fn func()) { h.queued <- fn }
func (h *fakeHooks) pending() int                             { return len(h.queued) }
func (h *fakeHooks) commit() {
	for {
		select {
		case fn := <-h.queued:
			fn()
		default:
			return
		}
	}
}

type fakeBroadcaster struct {
	studentUpdated, queueChanged int
}

func (b *fakeBroadcaster) StudentUpdated(int64) error        { b.studentUpdated++; return nil }
func (b *fakeBroadcaster) ChangeRequestsChanged(int64) error { b.queueChanged++; return nil }

type fakeMessenger struct {
	hasAccess     bool
	accessErr     error
	enqueued      []ports.RequestEvent
	emitted       []ports.RequestEvent
	guardianWakes int
	audience      ports.DecisionAudience
	audienceFull  []ports.RequestEvent
	audienceCalls int
}

func (m *fakeMessenger) EnqueueRequestDecision(_ context.Context, _, _, _ int64, event ports.RequestEvent) error {
	m.enqueued = append(m.enqueued, event)
	return nil
}
func (m *fakeMessenger) EmitChildEvent(_, _, _ int64, event ports.RequestEvent) {
	m.emitted = append(m.emitted, event)
}
func (m *fakeMessenger) BroadcastChildUpdateToGuardians(_, _ int64) { m.guardianWakes++ }
func (m *fakeMessenger) GuardianHasChildAccess(context.Context, int64, int64) (bool, error) {
	return m.hasAccess, m.accessErr
}
func (m *fakeMessenger) ResolveDecisionAudience(context.Context, int64, int64, []int64) (ports.DecisionAudience, error) {
	return m.audience, nil
}
func (m *fakeMessenger) EmitDecisionAudience(_, _ int64, _ ports.DecisionAudience, full, _ ports.RequestEvent) {
	m.audienceCalls++
	m.audienceFull = append(m.audienceFull, full)
}

type fakeNotifier struct{ reports []ports.AbsenceReport }

func (n *fakeNotifier) NotifyAbsenceReported(_ context.Context, report ports.AbsenceReport) error {
	n.reports = append(n.reports, report)
	return nil
}

type fakeLedger struct{ entries []ports.RequestLedgerEntry }

func (l *fakeLedger) Record(_ context.Context, entry ports.RequestLedgerEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

type harness struct {
	carePlan    *fakeCarePlan
	students    *fakeStudents
	hooks       *fakeHooks
	broadcaster *fakeBroadcaster
	messenger   *fakeMessenger
	notifier    *fakeNotifier
	ledger      *fakeLedger
	scope       ports.ReviewScope
	scopeErr    error
	workflow    *ExcusedRequests
}

func newHarness(t *testing.T, carePlan *fakeCarePlan, students *fakeStudents) *harness {
	t.Helper()
	h := &harness{
		carePlan: carePlan, students: students, hooks: newFakeHooks(),
		broadcaster: &fakeBroadcaster{}, messenger: &fakeMessenger{hasAccess: true},
		notifier: &fakeNotifier{}, ledger: &fakeLedger{}, scope: ports.ReviewScope{SchoolWide: true},
	}
	workflow, err := NewExcusedRequests(ExcusedRequestDependencies{
		CarePlan: carePlan, Students: students, Hooks: h.hooks, Today: func() careplan.Date { return fakeToday },
		Scope:     func(context.Context) (ports.ReviewScope, error) { return h.scope, h.scopeErr },
		Messenger: h.messenger, Broadcaster: h.broadcaster, Notifier: h.notifier, Ledger: h.ledger,
	})
	require.NoError(t, err)
	h.workflow = workflow
	return h
}

func pendingRow(id int64, dates ...careplan.Date) careplan.ExcusedAbsenceRequest {
	return careplan.ExcusedAbsenceRequest{
		ID: id, TenantID: fakeTenantID, StudentID: fakeStudentID, SubmittedBy: fakeGuardian,
		Dates: dates, Note: "Arzttermin", AbsenceStatus: careplan.StudentStatusDayExcused,
		Status: careplan.ExcusedRequestStatusPending, CreatedAt: time.Now().Add(-time.Hour),
	}
}

func decidablePlan(row careplan.ExcusedAbsenceRequest) *fakeCarePlan {
	return &fakeCarePlan{
		findPending: func(context.Context, int64) (careplan.ExcusedAbsenceRequest, error) { return row, nil },
		findRequest: func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error) {
			decided := row
			decided.Status = careplan.ExcusedRequestStatusApproved
			return decided, nil
		},
	}
}

// --- construction and validation

func TestNewExcusedRequestsRequiresCoreDependencies(t *testing.T) {
	t.Parallel()
	_, err := NewExcusedRequests(ExcusedRequestDependencies{})
	require.Error(t, err)
}

func TestSubmitValidatesBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeCarePlan{createRequest: func(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
		t.Fatal("validation must refuse before the store is touched")
		return careplan.ExcusedAbsenceRequest{}, nil
	}}, &fakeStudents{})
	ctx := context.Background()

	_, err := h.workflow.CreateRequest(ctx, fakeStudentID, fakeGuardian, nil, "note")
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNoDates)
	_, err = h.workflow.CreateRequest(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, "   ")
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestEmptyNote)
	_, err = h.workflow.CreateRequest(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, strings.Repeat("x", 2001))
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNoteTooLong)
	_, err = h.workflow.CreateRequestForStatus(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, "note", careplan.StudentStatusDayClassTrip)
	assert.ErrorIs(t, err, careplan.ErrAbsenceRequestInvalidStatus)

	// A school that asks nobody for a reason accepts a blank note.
	h = newHarness(t, &fakeCarePlan{}, &fakeStudents{})
	created, err := h.workflow.Submit(ctx, careplan.ExcusedRequestCreateInput{
		StudentID: fakeStudentID, GuardianAccountID: fakeGuardian, Dates: []careplan.Date{fakeNextWeek, fakeTomorrow, fakeTomorrow},
		AbsenceStatus: careplan.StudentStatusDaySick, NoteRequired: false,
	})
	require.NoError(t, err)
	assert.Equal(t, []careplan.Date{fakeTomorrow, fakeNextWeek}, created.Dates, "dates are stored deduplicated and ascending")
	assert.Empty(t, created.Note)
}

func TestDecideValidatesBeforeTheLock(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeCarePlan{}, &fakeStudents{})
	ctx := context.Background()

	_, err := h.workflow.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: 0, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound)
	_, err = h.workflow.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "  "})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestRejectReasonRequired)
	_, err = h.workflow.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true, ReasonRequired: true})
	assert.ErrorIs(t, err, careplan.ErrParentRequestReasonRequired)
	_, err = h.workflow.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: strings.Repeat("x", 2001)})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestRejectReasonTooLong)
}

// --- create path guards

func TestSubmitIsIdempotentForIdenticalAndRefusesOverlap(t *testing.T) {
	t.Parallel()
	existing := pendingRow(4, fakeTomorrow, fakeNextWeek)
	h := newHarness(t, &fakeCarePlan{
		listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
			return []careplan.ExcusedAbsenceRequest{existing}, nil
		},
		createRequest: func(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
			t.Fatal("an identical or overlapping submission must not insert")
			return careplan.ExcusedAbsenceRequest{}, nil
		},
	}, &fakeStudents{})
	ctx := context.Background()

	same, err := h.workflow.CreateRequest(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeNextWeek, fakeTomorrow}, "nochmal")
	require.NoError(t, err)
	assert.Equal(t, existing.ID, same.ID, "an identical resubmit is an idempotent retry")
	assert.Empty(t, h.ledger.entries, "a retry records nothing")

	_, err = h.workflow.CreateRequest(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow, fakePast}, "teilweise")
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestOverlap)

	_, err = h.workflow.CreateRequestForStatus(ctx, fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow, fakeNextWeek}, "krank", careplan.StudentStatusDaySick)
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestOverlap, "the same days cannot carry a pending sick and a pending excused request")
}

func TestSubmitRefusedWhenManualPartialAbsenceOwnsADay(t *testing.T) {
	t.Parallel()
	from := time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC)
	h := newHarness(t, &fakeCarePlan{
		listPickups: func(_ context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
			assert.Equal(t, []int64{fakeStudentID}, filter.StudentIDs)
			return []careplan.PickupException{
				{StudentID: fakeStudentID, ExceptionDate: fakeTomorrow, ExcusedFrom: &from, ExcusedAuto: true},
				{StudentID: fakeStudentID, ExceptionDate: fakeNextWeek, ExcusedFrom: &from},
			}, nil
		},
	}, &fakeStudents{})

	_, err := h.workflow.CreateRequest(context.Background(), fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, "auto excusal coexists")
	require.NoError(t, err, "an auto-derived excusal does not block a full-day request")
	_, err = h.workflow.CreateRequest(context.Background(), fakeStudentID, fakeGuardian, []careplan.Date{fakeNextWeek}, "manual excusal conflicts")
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestStatusConflict)
}

func TestSubmitRecordsLedgerAndWakesBothAudiencesAfterCommit(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeCarePlan{}, &fakeStudents{})

	created, err := h.workflow.CreateRequest(context.Background(), fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, "Arzttermin")
	require.NoError(t, err)
	require.Len(t, h.ledger.entries, 1)
	assert.Equal(t, careplan.ParentRequestEventSubmitted, h.ledger.entries[0].EventType)
	assert.Equal(t, created.ID, h.ledger.entries[0].RequestID)
	require.Len(t, h.messenger.enqueued, 1, "the durable pill intent is written inside the transaction")
	assert.Equal(t, "active.excused_absence_requests", h.messenger.enqueued[0].RefTable)
	assert.Equal(t, created.ID, h.messenger.enqueued[0].RefID)

	assert.Zero(t, h.broadcaster.studentUpdated, "nothing is broadcast before the commit")
	assert.Zero(t, h.messenger.guardianWakes)
	assert.Empty(t, h.messenger.emitted)
	h.hooks.commit()
	assert.Equal(t, 1, h.broadcaster.studentUpdated)
	assert.Equal(t, 1, h.broadcaster.queueChanged, "the staff queue is woken independently of parent messaging")
	assert.Equal(t, 1, h.messenger.guardianWakes, "every guardian is woken independently of parent messaging")
	require.Len(t, h.messenger.emitted, 1)
	assert.Equal(t, "Anfrage: Entschuldigte Abmeldung", h.messenger.emitted[0].Body)
}

func TestSubmitWrapsStoreFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeCarePlan{createRequest: func(context.Context, careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, error) {
		return careplan.ExcusedAbsenceRequest{}, errBoom
	}}, &fakeStudents{})
	_, err := h.workflow.CreateRequest(context.Background(), fakeStudentID, fakeGuardian, []careplan.Date{fakeTomorrow}, "note")
	require.ErrorIs(t, err, errBoom)
	assert.Zero(t, h.hooks.pending(), "a failed write schedules no after-commit effect")
}

// --- reads

func TestListForStudentPropagatesStoreFailures(t *testing.T) {
	t.Parallel()
	calls := 0
	h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		calls++
		if calls == 2 {
			return nil, errBoom
		}
		return nil, nil
	}}, &fakeStudents{})
	_, err := h.workflow.ListForStudent(context.Background(), fakeStudentID, time.Now())
	require.ErrorIs(t, err, errBoom, "the recent-decisions read fails after the pending read")
}

func TestListForStudentHidesWithdrawnAndDedupes(t *testing.T) {
	t.Parallel()
	pending := pendingRow(1, fakeTomorrow)
	rejected := pendingRow(2, fakeNextWeek)
	rejected.Status = careplan.ExcusedRequestStatusRejected
	withdrawn := pendingRow(3, fakePast)
	withdrawn.Status = careplan.ExcusedRequestStatusWithdrawn
	h := newHarness(t, &fakeCarePlan{listRequests: func(_ context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		if len(filter.Statuses) == 1 {
			return []careplan.ExcusedAbsenceRequest{pending}, nil
		}
		return []careplan.ExcusedAbsenceRequest{pending, rejected, withdrawn}, nil
	}}, &fakeStudents{})
	rows, err := h.workflow.ListForStudent(context.Background(), fakeStudentID, time.Now())
	require.NoError(t, err)
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	assert.Equal(t, []int64{1, 2}, ids, "pending first, decided once, withdrawn hidden")
}

func TestListPendingPropagatesDirectoryFailures(t *testing.T) {
	t.Parallel()
	row := pendingRow(1, fakeTomorrow)
	listOne := func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return []careplan.ExcusedAbsenceRequest{row}, nil
	}

	h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return nil, errBoom
	}}, &fakeStudents{})
	_, _, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.ErrorIs(t, err, errBoom)

	h = newHarness(t, &fakeCarePlan{listRequests: listOne}, &fakeStudents{students: func(context.Context, []int64) (map[int64]ports.ReviewStudent, error) {
		return nil, errBoom
	}})
	_, _, err = h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.ErrorIs(t, err, errBoom)

	h = newHarness(t, &fakeCarePlan{listRequests: listOne}, &fakeStudents{persons: func(context.Context, []int64) (map[int64]ports.PersonName, error) {
		return nil, errBoom
	}})
	_, _, err = h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.ErrorIs(t, err, errBoom)

	h = newHarness(t, &fakeCarePlan{listRequests: listOne}, &fakeStudents{})
	h.scopeErr = errBoom
	_, _, err = h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.ErrorIs(t, err, errBoom, "a scope resolution failure is not an empty queue")
}

func TestListPendingScopesEnrichesAndProbesTheNextPage(t *testing.T) {
	t.Parallel()
	other := otherStudentID
	rows := []careplan.ExcusedAbsenceRequest{pendingRow(3, fakeTomorrow), pendingRow(2, fakeTomorrow), pendingRow(1, fakeTomorrow)}
	rows[1].StudentID = other
	var seenLimit int
	group := fakeGroupID
	h := newHarness(t, &fakeCarePlan{listRequests: func(_ context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		seenLimit = filter.Queue.Limit
		return rows[:filter.Queue.Limit], nil
	}}, &fakeStudents{students: func(context.Context, []int64) (map[int64]ports.ReviewStudent, error) {
		return map[int64]ports.ReviewStudent{
			fakeStudentID: {ID: fakeStudentID, PersonID: fakePersonID, GroupID: &group},
			other:         {ID: other, PersonID: 100},
		}, nil
	}})
	h.scope = ports.ReviewScope{GroupIDs: []int64{group}}

	items, next, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, seenLimit, "the store is asked for one probe row beyond the page")
	require.NotNil(t, next, "the probe row proves an older page exists")
	assert.Equal(t, rows[1].ID, next.ID, "the cursor points at the last store row, not the last visible item")
	require.Len(t, items, 1, "the child outside the reviewer's groups is filtered after the page")
	assert.Equal(t, rows[0].ID, items[0].Request.ID)
	assert.Equal(t, "Felix", items[0].FirstName)
	assert.True(t, items[0].BulkEligible)
	assert.Equal(t, map[string]string{fakeTomorrow.String(): careplan.StudentStatusDayPresent}, items[0].CurrentStatusByDate)
}

func TestListPendingUsesSharedDayForCareEndAndEligibility(t *testing.T) {
	t.Parallel()
	for _, end := range []careplan.Date{"", fakePast} {
		h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
			return []careplan.ExcusedAbsenceRequest{pendingRow(1, fakePast)}, nil
		}}, &fakeStudents{students: func(context.Context, []int64) (map[int64]ports.ReviewStudent, error) {
			return map[int64]ports.ReviewStudent{fakeStudentID: {ID: fakeStudentID, PersonID: fakePersonID, EnrolledUntil: end}}, nil
		}})
		items, _, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{UrgentDate: fakePast.String()})
		require.NoError(t, err)
		require.Len(t, items, 1, "the request started before the owner's clock crossed midnight")
		assert.True(t, items[0].BulkEligible, "eligibility uses the same day as visibility")
	}
}

func TestListPendingHidesAlumniAndEndedCare(t *testing.T) {
	t.Parallel()
	ended := endedStudentID
	rows := []careplan.ExcusedAbsenceRequest{pendingRow(1, fakeTomorrow), pendingRow(2, fakeTomorrow)}
	rows[1].StudentID = ended
	h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return rows, nil
	}}, &fakeStudents{students: func(context.Context, []int64) (map[int64]ports.ReviewStudent, error) {
		return map[int64]ports.ReviewStudent{
			fakeStudentID: {ID: fakeStudentID, PersonID: fakePersonID, Alumnus: true},
			ended:         {ID: ended, PersonID: 100, EnrolledUntil: fakePast},
		}, nil
	}})
	items, _, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.NoError(t, err)
	assert.Empty(t, items)
	badges, err := h.workflow.PendingByStudentForDate(context.Background(), fakeTomorrow)
	require.NoError(t, err)
	assert.Empty(t, badges)
}

func TestPendingByStudentForDateKeepsNewestPerStudent(t *testing.T) {
	t.Parallel()
	rows := []careplan.ExcusedAbsenceRequest{pendingRow(9, fakeTomorrow), pendingRow(8, fakeTomorrow), pendingRow(7, fakeNextWeek)}
	h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return rows, nil
	}}, &fakeStudents{})
	badges, err := h.workflow.PendingByStudentForDate(context.Background(), fakeTomorrow)
	require.NoError(t, err)
	require.Len(t, badges, 1)
	assert.Equal(t, rows[0].ID, badges[fakeStudentID].ID, "the newest pending request wins")
	miss, err := h.workflow.PendingByStudentForDate(context.Background(), fakePast)
	require.NoError(t, err)
	assert.Empty(t, miss)
}

func TestPendingByStudentForDatePropagatesFailures(t *testing.T) {
	t.Parallel()
	h := newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return nil, errBoom
	}}, &fakeStudents{})
	_, err := h.workflow.PendingByStudentForDate(context.Background(), fakeTomorrow)
	require.ErrorIs(t, err, errBoom)

	h = newHarness(t, &fakeCarePlan{listRequests: func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return []careplan.ExcusedAbsenceRequest{pendingRow(1, fakeTomorrow)}, nil
	}}, &fakeStudents{students: func(context.Context, []int64) (map[int64]ports.ReviewStudent, error) { return nil, errBoom }})
	_, err = h.workflow.PendingByStudentForDate(context.Background(), fakeTomorrow)
	require.ErrorIs(t, err, errBoom)
}

// --- decisions

func TestDecideForbiddenOutsideTheReviewScope(t *testing.T) {
	t.Parallel()
	h := newHarness(t, decidablePlan(pendingRow(5, fakeTomorrow)), &fakeStudents{})
	h.scope = ports.ReviewScope{}
	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestForbidden)
	assert.Zero(t, h.hooks.pending())
}

func TestDecideRejectsStaleVersionAfterTheLock(t *testing.T) {
	t.Parallel()
	row := pendingRow(5, fakeTomorrow)
	row.UpdatedAt = time.Now()
	h := newHarness(t, decidablePlan(row), &fakeStudents{})
	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true, ExpectedVersion: "stale"})
	assert.ErrorIs(t, err, careplan.ErrParentRequestStale)
	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{
		RequestID: 5, Approve: true, ExpectedVersion: careplan.ParentRequestVersion(row.UpdatedAt),
	})
	require.NoError(t, err)
}

func TestDecideRefusesAlumniEndedCareAndDaysBeyondCare(t *testing.T) {
	t.Parallel()
	alumnus := activeStudent()
	alumnus.Alumnus = true
	h := newHarness(t, decidablePlan(pendingRow(5, fakeTomorrow)), &fakeStudents{lock: func(context.Context, int64) (ports.ReviewStudent, error) { return alumnus, nil }})
	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "weg"})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound, "neither verdict may act on an alumnus")

	beyond := activeStudent()
	beyond.EnrolledUntil = fakeTomorrow
	h = newHarness(t, decidablePlan(pendingRow(5, fakeTomorrow, fakeNextWeek)), &fakeStudents{lock: func(context.Context, int64) (ports.ReviewStudent, error) { return beyond, nil }})
	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestNotFound, "approval must not create an absence after care ends")
	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "zu spät"})
	require.NoError(t, err, "rejecting stays possible while care still runs")
}

func TestDecideApproveRefusedWhenAllDaysArePast(t *testing.T) {
	t.Parallel()
	h := newHarness(t, decidablePlan(pendingRow(5, fakePast)), &fakeStudents{})
	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrParentRequestPast)
	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "Zu spät eingereicht"})
	require.NoError(t, err, "rejecting is how staff wind a stale request down")
}

func TestDecideApproveRefusedWhenGuardianLostAccess(t *testing.T) {
	t.Parallel()
	h := newHarness(t, decidablePlan(pendingRow(5, fakeTomorrow)), &fakeStudents{})
	h.messenger.hasAccess = false
	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestGuardianAccessRevoked)

	h.carePlan.listRequests = func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return []careplan.ExcusedAbsenceRequest{pendingRow(5, fakeTomorrow)}, nil
	}
	items, _, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.False(t, items[0].BulkEligible)
	assert.Equal(t, careplan.BulkIneligibleAccessRevoked, items[0].BulkIneligibleReason)

	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: "nicht mehr berechtigt"})
	require.NoError(t, err, "rejecting is deliberately not gated on the guardian link")
}

func TestDecideApproveRefusedWhenANewerStatusExists(t *testing.T) {
	t.Parallel()
	row := pendingRow(5, fakeTomorrow)
	plan := decidablePlan(row)
	plan.listStatusDays = func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
		return []careplan.StudentStatusDay{{StudentID: fakeStudentID, Date: fakeTomorrow, Status: careplan.StudentStatusDaySick, ReportedAt: row.CreatedAt.Add(time.Minute)}}, nil
	}
	plan.listRequests = func(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		return []careplan.ExcusedAbsenceRequest{row}, nil
	}
	h := newHarness(t, plan, &fakeStudents{})

	items, _, err := h.workflow.ListPending(context.Background(), careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, careplan.BulkIneligibleStale, items[0].BulkIneligibleReason)
	assert.Equal(t, careplan.StudentStatusDaySick, items[0].CurrentStatusByDate[fakeTomorrow.String()])
	require.NotNil(t, items[0].CurrentValueChanged)
	assert.True(t, *items[0].CurrentValueChanged)

	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	assert.ErrorIs(t, err, careplan.ErrExcusedRequestStatusConflict)

	// An older status is exactly what the request supersedes.
	plan.listStatusDays = func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
		return []careplan.StudentStatusDay{{StudentID: fakeStudentID, Date: fakeTomorrow, Status: careplan.StudentStatusDaySick, ReportedAt: row.CreatedAt.Add(-time.Minute)}}, nil
	}
	_, err = h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
	require.NoError(t, err)
}

func TestDecideApproveAppliesDaysNotifiesInsideAndBroadcastsAfterCommit(t *testing.T) {
	t.Parallel()
	row := pendingRow(5, fakeToday, fakeTomorrow)
	row.AbsenceStatus = careplan.StudentStatusDaySick
	plan := decidablePlan(row)
	var cleared []string
	var upserted []careplan.StudentStatusDay
	plan.clearDays = func(_ context.Context, _ int64, status string, _ []careplan.Date, _ time.Time, source string) error {
		cleared = append(cleared, status+":"+source)
		return nil
	}
	plan.upsertDay = func(_ context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
		upserted = append(upserted, value)
		return value, nil
	}
	h := newHarness(t, plan, &fakeStudents{})
	h.messenger.audience = ports.DecisionAudience{Neutral: []int64{4}}

	item, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true, Reason: "Passt", ReviewedBy: fakeReviewer})
	require.NoError(t, err)
	assert.Equal(t, careplan.ExcusedRequestStatusApproved, item.Request.Status)
	assert.Equal(t, "Felix", item.FirstName)
	assert.Equal(t, []string{"excused:parent", "class_trip:parent"}, cleared, "every competing status is cleared before the requested one is written")
	require.Len(t, upserted, 2)
	assert.Equal(t, careplan.StudentStatusSourceParent, upserted[0].Source)
	require.NotNil(t, upserted[0].GuardianAccountID)
	assert.Equal(t, fakeGuardian, *upserted[0].GuardianAccountID)
	assert.Equal(t, 1, h.students.liveFlagCalls, "a sick day covering today raises the live flag")

	require.Len(t, h.notifier.reports, 1, "the durable notification intent is enqueued in the decision transaction")
	assert.Equal(t, fakeTenantID, h.notifier.reports[0].TenantID)
	assert.Equal(t, []int64{fakeGuardian}, h.notifier.reports[0].ExcludedAccountIDs)
	assert.Equal(t, fakeReviewer, h.notifier.reports[0].ActorAccountID)
	require.Len(t, h.ledger.entries, 1)
	assert.Equal(t, careplan.ParentRequestEventDecided, h.ledger.entries[0].EventType)
	assert.Equal(t, map[string]any{"approve": true, "reason": "Passt"}, h.ledger.entries[0].Payload)

	assert.Zero(t, h.broadcaster.queueChanged)
	assert.Zero(t, h.messenger.audienceCalls)
	h.hooks.commit()
	assert.Equal(t, 1, h.broadcaster.queueChanged)
	assert.Equal(t, 1, h.messenger.guardianWakes)
	assert.Equal(t, 1, h.messenger.audienceCalls, "co-guardians hear about the decision after commit")
	require.Len(t, h.messenger.emitted, 1)
	assert.Equal(t, "Krankmeldung bestätigt", h.messenger.emitted[0].Body)
}

func TestDecideRejectWritesNoDaysAndCarriesTheReason(t *testing.T) {
	t.Parallel()
	plan := decidablePlan(pendingRow(5, fakeTomorrow))
	plan.upsertDay = func(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
		t.Fatal("a rejection writes no status day")
		return careplan.StudentStatusDay{}, nil
	}
	var decision careplan.ExcusedAbsenceDecision
	plan.decide = func(_ context.Context, value careplan.ExcusedAbsenceDecision) error { decision = value; return nil }
	h := newHarness(t, plan, &fakeStudents{})

	_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: false, Reason: " telefonisch klären ", ReviewedBy: fakeReviewer})
	require.NoError(t, err)
	assert.Equal(t, careplan.ExcusedRequestStatusRejected, decision.Status)
	require.NotNil(t, decision.Reason)
	assert.Equal(t, "telefonisch klären", *decision.Reason)
	assert.False(t, decision.Applied)
	assert.Empty(t, h.notifier.reports)
	h.hooks.commit()
	require.Len(t, h.messenger.emitted, 1)
	assert.Equal(t, "Abmeldung abgelehnt: telefonisch klären", h.messenger.emitted[0].Body)
}

func TestDecidePropagatesEveryWriteFailure(t *testing.T) {
	t.Parallel()
	row := pendingRow(5, fakeToday)
	cases := map[string]func(*fakeCarePlan, *fakeStudents){
		"student lock": func(_ *fakeCarePlan, s *fakeStudents) {
			s.lock = func(context.Context, int64) (ports.ReviewStudent, error) { return ports.ReviewStudent{}, errBoom }
		},
		"clear competing days": func(p *fakeCarePlan, _ *fakeStudents) {
			p.clearDays = func(context.Context, int64, string, []careplan.Date, time.Time, string) error { return errBoom }
		},
		"upsert requested day": func(p *fakeCarePlan, _ *fakeStudents) {
			p.upsertDay = func(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
				return careplan.StudentStatusDay{}, errBoom
			}
		},
		"live flag": func(_ *fakeCarePlan, s *fakeStudents) {
			s.liveFlags = func(context.Context, int64, string, time.Time) error { return errBoom }
		},
		"decide row": func(p *fakeCarePlan, _ *fakeStudents) {
			p.decide = func(context.Context, careplan.ExcusedAbsenceDecision) error { return errBoom }
		},
		"reload": func(p *fakeCarePlan, _ *fakeStudents) {
			p.findRequest = func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error) {
				return careplan.ExcusedAbsenceRequest{}, errBoom
			}
		},
	}
	for name, inject := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan, students := decidablePlan(row), &fakeStudents{}
			inject(plan, students)
			h := newHarness(t, plan, students)
			_, err := h.workflow.Decide(context.Background(), careplan.ExcusedRequestDecideInput{RequestID: 5, Approve: true})
			require.ErrorIs(t, err, errBoom)
		})
	}
}

// --- mark done and correct

func TestMarkDoneRequiresAPastRequestAndAppliesNothing(t *testing.T) {
	t.Parallel()
	plan := decidablePlan(pendingRow(5, fakeTomorrow))
	plan.upsertDay = func(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
		t.Fatal("mark-done never writes a status day")
		return careplan.StudentStatusDay{}, nil
	}
	h := newHarness(t, plan, &fakeStudents{})
	err := h.workflow.MarkDone(context.Background(), 5, "", "", fakeReviewer)
	assert.ErrorIs(t, err, careplan.ErrParentRequestNotPast)

	past := pendingRow(6, fakePast)
	past.UpdatedAt = time.Now()
	plan.findPending = func(context.Context, int64) (careplan.ExcusedAbsenceRequest, error) { return past, nil }
	var decision careplan.ExcusedAbsenceDecision
	plan.decide = func(_ context.Context, value careplan.ExcusedAbsenceDecision) error { decision = value; return nil }
	assert.ErrorIs(t, h.workflow.MarkDone(context.Background(), 6, "stale", "", fakeReviewer), careplan.ErrParentRequestStale)
	require.NoError(t, h.workflow.MarkDone(context.Background(), 6, "", "Tage sind vorbei", fakeReviewer))
	assert.Equal(t, careplan.ExcusedRequestStatusDone, decision.Status)
	require.Len(t, h.ledger.entries, 1)
	assert.Equal(t, careplan.ParentRequestEventMarkedDone, h.ledger.entries[0].EventType)
	h.hooks.commit()
	require.Len(t, h.messenger.emitted, 1)
	assert.Equal(t, "Anfrage abgeschlossen", h.messenger.emitted[0].Body)
}

func TestCorrectRefusesUndecidedAndRevertsOnlyItsOwnDays(t *testing.T) {
	t.Parallel()
	pending := pendingRow(5, fakeTomorrow)
	plan := &fakeCarePlan{findRequest: func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error) { return pending, nil }}
	h := newHarness(t, plan, &fakeStudents{})
	assert.ErrorIs(t, h.workflow.Correct(context.Background(), 5, false, "", "Korrektur", fakeReviewer), careplan.ErrParentRequestNotDecided)

	reviewedAt := time.Now().Add(-time.Hour)
	approved := pendingRow(5, fakeTomorrow)
	approved.Status = careplan.ExcusedRequestStatusApproved
	approved.ReviewedAt = &reviewedAt
	plan.findRequest = func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error) { return approved, nil }
	guardian := fakeGuardian
	plan.listStatusDays = func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
		return []careplan.StudentStatusDay{{
			StudentID: fakeStudentID, Date: fakeTomorrow, Status: careplan.StudentStatusDayExcused,
			Source: careplan.StudentStatusSourceManual, GuardianAccountID: &guardian, ReportedAt: reviewedAt.Add(time.Minute),
		}}, nil
	}
	err := h.workflow.Correct(context.Background(), 5, false, "", "Doch nicht", fakeReviewer)
	assert.ErrorIs(t, err, careplan.ErrParentRequestCorrectionUnsupported, "a day re-entered by staff after the approval is never cleared")

	cleared := 0
	plan.listStatusDays = func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
		return []careplan.StudentStatusDay{{
			StudentID: fakeStudentID, Date: fakeTomorrow, Status: careplan.StudentStatusDayExcused,
			Source: careplan.StudentStatusSourceParent, GuardianAccountID: &guardian, ReportedAt: reviewedAt.Add(-time.Minute),
		}}, nil
	}
	plan.clearDays = func(context.Context, int64, string, []careplan.Date, time.Time, string) error { cleared++; return nil }
	var redecision careplan.ExcusedAbsenceDecision
	plan.redecide = func(_ context.Context, value careplan.ExcusedAbsenceDecision) error { redecision = value; return nil }
	require.NoError(t, h.workflow.Correct(context.Background(), 5, false, "", "Doch nicht genehmigt", fakeReviewer))
	assert.Equal(t, 1, cleared, "the approval's own days are cleared")
	assert.Equal(t, careplan.ExcusedRequestStatusRejected, redecision.Status)
	require.Len(t, h.ledger.entries, 1)
	assert.Equal(t, careplan.ParentRequestEventCorrected, h.ledger.entries[0].EventType)
	assert.Equal(t, careplan.ExcusedRequestStatusApproved, h.ledger.entries[0].Payload["from"])
	h.hooks.commit()
	require.Len(t, h.messenger.emitted, 1)
	assert.Equal(t, "Entscheidung geändert: Abmeldung abgelehnt", h.messenger.emitted[0].Body)
}

func TestCorrectApprovalKeepsDecideGuards(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		plan     func(careplan.ExcusedAbsenceRequest) *fakeCarePlan
		students *fakeStudents
		access   bool
		want     error
	}{
		{
			name: "newer status",
			plan: func(row careplan.ExcusedAbsenceRequest) *fakeCarePlan {
				plan := decidablePlan(row)
				plan.listStatusDays = func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
					return []careplan.StudentStatusDay{{Date: fakeTomorrow, ReportedAt: row.CreatedAt.Add(time.Minute)}}, nil
				}
				return plan
			},
			students: &fakeStudents{}, access: true, want: careplan.ErrExcusedRequestStatusConflict,
		},
		{
			name: "manual partial absence",
			plan: func(row careplan.ExcusedAbsenceRequest) *fakeCarePlan {
				plan := decidablePlan(row)
				excusedFrom := time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC)
				plan.listPickups = func(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
					return []careplan.PickupException{{ExceptionDate: fakeTomorrow, ExcusedFrom: &excusedFrom}}, nil
				}
				return plan
			},
			students: &fakeStudents{}, access: true, want: careplan.ErrExcusedRequestStatusConflict,
		},
		{
			name:     "guardian access revoked",
			plan:     func(row careplan.ExcusedAbsenceRequest) *fakeCarePlan { return decidablePlan(row) },
			students: &fakeStudents{}, access: false, want: careplan.ErrExcusedRequestGuardianAccessRevoked,
		},
		{
			name: "alumnus",
			plan: func(row careplan.ExcusedAbsenceRequest) *fakeCarePlan { return decidablePlan(row) },
			students: &fakeStudents{lock: func(context.Context, int64) (ports.ReviewStudent, error) {
				return ports.ReviewStudent{ID: fakeStudentID, PersonID: fakePersonID, Alumnus: true}, nil
			}},
			access: true, want: careplan.ErrExcusedRequestNotFound,
		},
		{
			name: "care ended",
			plan: func(row careplan.ExcusedAbsenceRequest) *fakeCarePlan { return decidablePlan(row) },
			students: &fakeStudents{lock: func(context.Context, int64) (ports.ReviewStudent, error) {
				return ports.ReviewStudent{ID: fakeStudentID, PersonID: fakePersonID, EnrolledUntil: fakePast}, nil
			}},
			access: true, want: careplan.ErrExcusedRequestNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := pendingRow(5, fakeTomorrow)
			h := newHarness(t, tc.plan(row), tc.students)
			h.messenger.hasAccess = tc.access

			err := h.workflow.Correct(context.Background(), row.ID, true, "", "Doch genehmigt", fakeReviewer)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// --- coordinator ports

func TestCoordinatorPortsTranslateTheWorkflowOutcomes(t *testing.T) {
	t.Parallel()
	row := pendingRow(5, fakeTomorrow)
	plan := decidablePlan(row)
	plan.findPending = func(context.Context, int64) (careplan.ExcusedAbsenceRequest, error) {
		return careplan.ExcusedAbsenceRequest{}, careplan.ErrExcusedRequestNotPending
	}
	h := newHarness(t, plan, &fakeStudents{})
	assert.ErrorIs(t, h.workflow.ApproveExcusedBulk(context.Background(), 5, "", fakeReviewer, ""), careplan.ErrParentRequestDecisionRace)
	assert.ErrorIs(t, h.workflow.LockConflictRequest(context.Background(), 5), careplan.ErrExcusedRequestNotPending)

	candidate, err := h.workflow.ConflictCandidate(context.Background(), 5)
	require.ErrorIs(t, err, careplan.ErrExcusedRequestNotPending, "a decided request is no conflict candidate")
	assert.Nil(t, candidate)

	plan.findRequest = func(context.Context, int64, bool) (careplan.ExcusedAbsenceRequest, error) { return row, nil }
	bulk, err := h.workflow.GetExcusedBulkCandidate(context.Background(), 5)
	require.NoError(t, err)
	require.NotNil(t, bulk)
	assert.True(t, bulk.Eligible)

	assert.ErrorIs(t, h.workflow.WriteStaffValue(context.Background(), careplan.ExcusedStaffValueWrite{Status: "unknown"}), careplan.ErrAbsenceRequestInvalidStatus)
	assert.ErrorIs(t, h.workflow.WriteStaffValue(context.Background(), careplan.ExcusedStaffValueWrite{Status: careplan.StudentStatusDaySick}), careplan.ErrStaffValueUnsupported, "no request ids means no days to write")

	plan.listRequests = func(_ context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error) {
		assert.Equal(t, []int64{5, 6}, filter.IDs)
		return []careplan.ExcusedAbsenceRequest{pendingRow(5, fakeTomorrow), pendingRow(6, fakeNextWeek, fakeTomorrow)}, nil
	}
	var written []careplan.Date
	plan.upsertDay = func(_ context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
		assert.Equal(t, careplan.StudentStatusSourceManual, value.Source)
		written = append(written, value.Date)
		return value, nil
	}
	require.NoError(t, h.workflow.WriteStaffValue(context.Background(), careplan.ExcusedStaffValueWrite{
		StudentID: fakeStudentID, RequestIDs: []int64{5, 6}, Reason: "Klassenfahrt", Status: careplan.StudentStatusDayClassTrip,
	}))
	assert.Equal(t, []careplan.Date{fakeTomorrow, fakeNextWeek}, written, "the days come from the requests, deduplicated and sorted")
}
