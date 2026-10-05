// Coverage tests for the parent-enrollment activate-students scheduler task.
// SetStudentLifecycleRepo wiring, scheduleActivateStudentsTask nil-vs-registered
// branches, and runActivateStudentsForTenant transition behaviour (pending →
// active, active → inactive, idempotency).
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStudentLifecycleRepo is a deterministic test double for
// StudentLifecycleRepository. Per-call lists drive the Find* methods so tests
// can stage exactly the rows the tick should see; TransitionStatus appends to
// a slice the test inspects to verify transitions ran in order.
type fakeStudentLifecycleRepo struct {
	mu              sync.Mutex
	pendingDue      []int64
	activeDue       []int64
	pendingErr      error
	activeErr       error
	updateErr       error
	updateErrForID  int64 // if > 0, only fail TransitionStatus when called with this ID
	updates         []update
	currentStatuses map[int64]string
	pendingCalls    int
	activeCalls     int
}

type update struct {
	studentID int64
	to        string
}

func (f *fakeStudentLifecycleRepo) FindPendingDueForActivation(_ context.Context, _ calendar.Date) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pendingCalls++
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return f.pendingDue, nil
}

func (f *fakeStudentLifecycleRepo) FindActiveDueForDeactivation(_ context.Context, _ calendar.Date) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activeCalls++
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	return f.activeDue, nil
}

// TransitionStatus records the transition like the old unconditional
// UpdateStatus did; currentStatuses stages the compare-and-set MISS (the row's
// status changed since it was selected — e.g. a grade transition graduated the
// child) so the tick's skip path is exercised.
func (f *fakeStudentLifecycleRepo) TransitionStatus(
	_ context.Context,
	studentID int64,
	expected string,
	next string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		if f.updateErrForID == 0 || f.updateErrForID == studentID {
			return false, f.updateErr
		}
	}
	if f.currentStatuses != nil {
		if f.currentStatuses[studentID] != expected {
			return false, nil
		}
		f.currentStatuses[studentID] = next
	}
	f.updates = append(f.updates, update{studentID: studentID, to: next})
	return true, nil
}

// -----------------------------------------------------------------------------
// scheduleActivateStudentsTask — nil repo vs. registered
// -----------------------------------------------------------------------------

func TestScheduleActivateStudentsTask_NilRepo(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		done:   make(chan struct{}),
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask)})

	s.scheduleActivateStudentsTask()
	assert.Empty(t, s.tasks, "nil repo → no task registered")
}

func TestScheduleActivateStudentsTask_RegistersTask(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		done:                 make(chan struct{}),
		logger:               slog.Default(),
		tasks:                make(map[string]*ScheduledTask),
		studentLifecycleRepo: &fakeStudentLifecycleRepo{}})

	// Pre-close done so the spawned goroutine exits during the startup sleep.
	close(s.done)

	s.scheduleActivateStudentsTask()
	s.wg.Wait()

	s.mu.RLock()
	task, ok := s.tasks["activate-students"]
	s.mu.RUnlock()
	require.True(t, ok, "activate-students task must be registered")
	assert.Equal(t, "interval-poll", task.Schedule)
}

// -----------------------------------------------------------------------------
// runActivateStudentsForTenant — transition behaviour
// -----------------------------------------------------------------------------

func TestRunActivateStudentsForTenant_PendingToActive(t *testing.T) {
	t.Parallel()

	pending := []int64{101, 102}

	repo := &fakeStudentLifecycleRepo{pendingDue: pending}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.updates, 2, "both pending students should be activated")
	for _, u := range repo.updates {
		assert.Equal(t, studentStatusActive, u.to)
	}
}

func TestRunActivateStudentsForTenant_ActiveToInactive(t *testing.T) {
	t.Parallel()

	due := []int64{201}

	repo := &fakeStudentLifecycleRepo{activeDue: due}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.updates, 1)
	assert.Equal(t, int64(201), repo.updates[0].studentID)
	assert.Equal(t, studentStatusInactive, repo.updates[0].to)
}

func TestRunActivateStudentsForTenant_BothDirections(t *testing.T) {
	t.Parallel()

	pending := []int64{301}
	due := []int64{302}

	repo := &fakeStudentLifecycleRepo{
		pendingDue: pending,
		activeDue:  due,
	}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.updates, 2)
	// Order: pending→active runs first, then active→inactive.
	assert.Equal(t, int64(301), repo.updates[0].studentID)
	assert.Equal(t, studentStatusActive, repo.updates[0].to)
	assert.Equal(t, int64(302), repo.updates[1].studentID)
	assert.Equal(t, studentStatusInactive, repo.updates[1].to)
}

func TestRunActivateStudentsForTenant_Idempotent(t *testing.T) {
	t.Parallel()

	// After the first run flips the student to active, a second run sees no due
	// pending rows. Simulate by clearing pendingDue between calls.
	pending := []int64{401}
	repo := &fakeStudentLifecycleRepo{pendingDue: pending}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	// "Re-query" empty-handed: the repo would now return no rows because the
	// previous tick already moved them to active. Clear the test fixture to
	// reflect that, then run again.
	repo.mu.Lock()
	repo.pendingDue = nil
	repo.mu.Unlock()

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Len(t, repo.updates, 1, "second tick must not re-update already-active students")
}

func TestRunActivateStudentsForTenant_FindPendingError_StillProcessesActive(t *testing.T) {
	t.Parallel()

	due := []int64{501}

	repo := &fakeStudentLifecycleRepo{
		pendingErr: errors.New("pending query failed"),
		activeDue:  due,
	}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.updates, 1, "active deactivation should still run when pending lookup fails")
	assert.Equal(t, studentStatusInactive, repo.updates[0].to)
}

func TestRunActivateStudentsForTenant_FindActiveError_NoUpdates(t *testing.T) {
	t.Parallel()

	repo := &fakeStudentLifecycleRepo{
		activeErr: errors.New("active query failed"),
	}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.updates)
}

func TestRunActivateStudentsForTenant_UpdateError_SkipsRowContinuesBatch(t *testing.T) {
	t.Parallel()

	// 601 fails its transition; 602 must still be processed.
	pending := []int64{601, 602}

	repo := &fakeStudentLifecycleRepo{
		pendingDue:     pending,
		updateErr:      errors.New("update blew up"),
		updateErrForID: 601,
	}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.updates, 1, "second student should still be processed after first one fails")
	assert.Equal(t, int64(602), repo.updates[0].studentID)
}

func TestRunActivateStudentsForTenant_NoDueRows_NoUpdates(t *testing.T) {
	t.Parallel()

	repo := &fakeStudentLifecycleRepo{}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		studentLifecycleRepo: repo})

	assert.NoError(t, s.runActivateStudentsForTenantWithError(context.Background(), 7, time.Now()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.updates)
	assert.Equal(t, 1, repo.pendingCalls)
	assert.Equal(t, 1, repo.activeCalls)
}

// -----------------------------------------------------------------------------
// activateStudentsInterval — fixed cadence (#3733)
// -----------------------------------------------------------------------------

// The interval was a school setting that no school ever changed; the tick
// keeps running every 60 minutes as a code constant.
func TestActivateStudentsInterval_IsSixtyMinutes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 60*time.Minute, activateStudentsInterval)
}
