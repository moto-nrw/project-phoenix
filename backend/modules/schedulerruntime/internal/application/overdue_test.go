// WP-B9 scheduler overdue-tick tests.
//
// Covers the three observable behaviours of checkAndRunOverdue:
//
//  1. A planned instance whose start_time is past the threshold fires exactly
//     one `instance_overdue` broadcast.
//  2. A second tick on the same fixture does NOT re-fire (sync.Map guard).
//  3. An active instance is NEVER broadcast (only planned triggers).
//
// Plus a unit test for the day-rollover cache clear, which is pure in-memory
// so it doesn't need the DB.
package application

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overdueSetup wires a scheduler instance just enough to exercise the
// overdue tick. The day's instances come through the scheduler's port; the
// root's binding of that port to the retained activity-instance reads is
// covered in api/scheduler_ports_test.go.
type overdueSetup struct {
	sched     *Scheduler
	instances *fakeInstanceRepo
	spy       *recordingOverdue
	room      int64
	nextID    int64
	// now is a fixed wall-clock anchor (noon UTC on a fixed day) used by both
	// seedPlanned and the test calls to runOverdueForTenant, so fixtures that
	// subtract `minutesAgo` cannot cross midnight.
	now time.Time
}

func buildOverdue(t *testing.T) *overdueSetup {
	t.Helper()
	instances := &fakeInstanceRepo{}
	spy := &recordingOverdue{}
	const room = 61
	sched := unitScheduler(&unitSchedulerDependencies{
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),
		logger: slog.Default()})

	sched.instanceRepo = instances.read
	sched.instanceRoomRepo = (&fakeOverdueRoomRepo{roomIDs: []int64{room}}).existing
	sched.overdueBroadcaster = spy

	return &overdueSetup{
		sched:     sched,
		instances: instances,
		spy:       spy,
		room:      room,
		nextID:    700,
		now:       time.Date(2026, time.April, 20, 12, 0, 0, 0, time.UTC),
	}
}

// seedPlanned adds one planned instance of the day with a StartTime of
// `minutesAgo` minutes before s.now, so callers can control the overdue
// margin relative to the default 5-minute threshold.
func seedPlanned(t *testing.T, s *overdueSetup, minutesAgo int) ports.DayInstance {
	t.Helper()
	start := s.now.Add(-time.Duration(minutesAgo) * time.Minute)
	s.nextID++
	s.instances.instances = append(s.instances.instances, ports.DayInstance{
		ID:        s.nextID,
		Date:      calendar.DateFromTime(s.now),
		StartTime: time.Date(1, 1, 1, start.Hour(), start.Minute(), start.Second(), 0, time.UTC),
		RoomID:    s.room,
		Planned:   true,
	})
	return s.instances.instances[len(s.instances.instances)-1]
}

// markStarted makes the port report the instance as no longer planned (for
// the active-not-overdue branch).
func markStarted(t *testing.T, s *overdueSetup, id int64) {
	t.Helper()
	for index := range s.instances.instances {
		if s.instances.instances[index].ID == id {
			s.instances.instances[index].Planned = false
			return
		}
	}
	t.Fatalf("instance %d was not seeded", id)
}

// The tests call runOverdueForTenant directly; checkAndRunOverdue's tenant
// iteration delegates to it in production.

func TestOverdueTick_BroadcastsOncePerInstance(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	// 30 minutes past — well beyond the 5-minute default threshold.
	ai := seedPlanned(t, s, 30)

	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, overdueAnnounced), "one overdue broadcast for the seeded instance expected")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, overdueRefreshed), "one active supervision refresh broadcast expected")

	announced := s.spy.find(ai.ID, overdueAnnounced)
	require.NotNil(t, announced, "expected broadcast for instance id %d", ai.ID)
	assert.Equal(t, testpkg.Tenant(t), announced.tenantID)
	assert.Equal(t, ai.ID, announced.instanceID)
	assert.Equal(t, ai.Date.String(), announced.date)
	assert.Equal(t, ai.StartTime.Format("15:04:05"), announced.startTime)
	assert.Equal(t, s.room, announced.roomID)
}

func TestOverdueTick_ReFireGuard(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)

	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)
	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, overdueAnnounced), "second tick on same instance must suppress overdue event")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, overdueRefreshed), "second tick on same instance must suppress active supervision refresh")
}

func TestOverdueTick_ActiveInstancesNotBroadcast(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)
	markStarted(t, s, ai.ID)

	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, overdueAnnounced), "active instances must not trigger overdue broadcast")
	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, overdueRefreshed), "active instances must not trigger active supervision refresh")
}

// overdueKind names the two announcements of an overdue planned block: the
// block itself and the refresh of the live-supervision views. Their event
// envelopes are covered in realtime and api/scheduler_presence_ports_test.go.
type overdueKind string

const (
	overdueAnnounced overdueKind = "instance_overdue"
	overdueRefreshed overdueKind = "supervision_refresh"
)

type overdueCall struct {
	kind       overdueKind
	tenantID   int64
	instanceID int64
	date       string
	startTime  string
	roomID     int64
}

// recordingOverdue records the Worker's overdue announcements; err makes
// every announcement fail after it was recorded.
type recordingOverdue struct {
	mu       sync.Mutex
	recorded []overdueCall
	err      error
}

func (r *recordingOverdue) BroadcastInstanceOverdue(tenantID, instanceID int64, date, startTime string, roomID int64) error {
	return r.record(overdueCall{kind: overdueAnnounced, tenantID: tenantID, instanceID: instanceID, date: date, startTime: startTime, roomID: roomID})
}

func (r *recordingOverdue) BroadcastInstanceOverdueRefresh(tenantID, instanceID int64) error {
	return r.record(overdueCall{kind: overdueRefreshed, tenantID: tenantID, instanceID: instanceID})
}

func (r *recordingOverdue) record(call overdueCall) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recorded = append(r.recorded, call)
	return r.err
}

func (r *recordingOverdue) calls() []overdueCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]overdueCall(nil), r.recorded...)
}

// find returns the first announcement of the kind for the instance, or nil.
func (r *recordingOverdue) find(instanceID int64, kind overdueKind) *overdueCall {
	for _, call := range r.calls() {
		if call.kind == kind && call.instanceID == instanceID {
			return &call
		}
	}
	return nil
}

// spyFilter counts the announcements of the kind for the instance. Needed
// because a fixture may hold unrelated rows of the day; we care only about
// our fixture's fire count, not the grand total.
func spyFilter(r *recordingOverdue, instanceID int64, kind overdueKind) int {
	n := 0
	for _, call := range r.calls() {
		if call.kind == kind && call.instanceID == instanceID {
			n++
		}
	}
	return n
}

// Day-rollover is a pure-memory behaviour — test it without the DB.
func TestRotateOverdueCacheIfNewDay(t *testing.T) {
	t.Parallel()
	sched := unitScheduler(&unitSchedulerDependencies{
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),
		logger: slog.Default()})

	// Seed: mark "yesterday" as the cache day, store one emitted key.
	yesterday := calendar.NewDate(2026, 4, 19)
	sched.overdueEmittedDay = yesterday
	key := overdueKey{tenantID: 1, instanceID: 42}
	sched.overdueEmitted.Store(key, yesterday.UTCMidnight().Add(14*time.Hour))

	// Rotate using "today" → cache must clear and the day must advance.
	today := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	sched.rotateOverdueCacheIfNewDay(today)

	_, present := sched.overdueEmitted.Load(key)
	assert.False(t, present, "entry from yesterday must be evicted on day rollover")
	assert.Equal(t, calendar.NewDate(2026, 4, 20), sched.overdueEmittedDay)
}
