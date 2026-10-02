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
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	spy       *testpkg.RecordingBroadcaster
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
	spy := testpkg.NewRecordingBroadcaster()
	const room = 61
	sched := unitScheduler(&Scheduler{
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
func seedPlanned(t *testing.T, s *overdueSetup, minutesAgo int) DayInstance {
	t.Helper()
	start := s.now.Add(-time.Duration(minutesAgo) * time.Minute)
	s.nextID++
	s.instances.instances = append(s.instances.instances, DayInstance{
		ID:        s.nextID,
		Date:      calendar.DateFromTime(s.now),
		StartTime: time.Date(1, 1, 1, start.Hour(), start.Minute(), start.Second(), 0, time.UTC),
		RoomID:    s.room,
		Status:    timetable.InstanceStatusPlanned,
	})
	return s.instances.instances[len(s.instances.instances)-1]
}

// setStatus sets the composed status the port reports for the instance (for
// the active-not-overdue branch).
func setStatus(t *testing.T, s *overdueSetup, id int64, status string) {
	t.Helper()
	for index := range s.instances.instances {
		if s.instances.instances[index].ID == id {
			s.instances.instances[index].Status = status
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

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtimeevents.EventInstanceOverdue), "one overdue broadcast for the seeded instance expected")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtimeevents.EventActiveSupervisionChanged), "one active supervision refresh broadcast expected")

	evt := spyFindByInstance(s.spy, ai.ID, realtimeevents.EventInstanceOverdue)
	require.NotNil(t, evt, "expected broadcast for instance id %d", ai.ID)
	assert.Equal(t, realtimeevents.EventInstanceOverdue, evt.Type)
	require.NotNil(t, evt.Data.InstanceID)
	assert.Equal(t, fmt.Sprintf("%d", ai.ID), *evt.Data.InstanceID)

	refresh := spyFindByInstance(s.spy, ai.ID, realtimeevents.EventActiveSupervisionChanged)
	require.NotNil(t, refresh, "expected active supervision refresh for instance id %d", ai.ID)
	require.NotNil(t, refresh.Data.Reason)
	assert.Equal(t, "instance_overdue", *refresh.Data.Reason)
}

func TestOverdueTick_ReFireGuard(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)

	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)
	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtimeevents.EventInstanceOverdue), "second tick on same instance must suppress overdue event")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtimeevents.EventActiveSupervisionChanged), "second tick on same instance must suppress active supervision refresh")
}

func TestOverdueTick_ActiveInstancesNotBroadcast(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)
	setStatus(t, s, ai.ID, "active")

	s.sched.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, realtimeevents.EventInstanceOverdue), "active instances must not trigger overdue broadcast")
	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, realtimeevents.EventActiveSupervisionChanged), "active instances must not trigger active supervision refresh")
}

// spyFilter counts broadcasts whose InstanceID matches the given id. Needed
// because the test DB is shared across runs and may contain unrelated
// today-dated rows from other tests; we care only about our fixture's
// fire count, not the grand total. The overdue tick only ever calls
// BroadcastToTenant, so filtering the "tenant" method reproduces the old
// spy's b.all semantics.
func spyFilter(b *testpkg.RecordingBroadcaster, instanceID int64, eventType realtimeevents.EventType) int {
	n := 0
	needle := fmt.Sprintf("%d", instanceID)
	for _, c := range b.CallsByMethod("tenant") {
		if c.Event.Type == eventType && c.Event.Data.InstanceID != nil && *c.Event.Data.InstanceID == needle {
			n++
		}
	}
	return n
}

// spyFindByInstance returns the first recorded event matching instanceID,
// or nil if none. Companion to spyFilter for envelope-shape assertions.
func spyFindByInstance(b *testpkg.RecordingBroadcaster, instanceID int64, eventType realtimeevents.EventType) *realtimeevents.Event {
	needle := fmt.Sprintf("%d", instanceID)
	for _, c := range b.CallsByMethod("tenant") {
		if c.Event.Type == eventType && c.Event.Data.InstanceID != nil && *c.Event.Data.InstanceID == needle {
			e := c.Event
			return &e
		}
	}
	return nil
}

// Day-rollover is a pure-memory behaviour — test it without the DB.
func TestRotateOverdueCacheIfNewDay(t *testing.T) {
	t.Parallel()
	sched := unitScheduler(&Scheduler{
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
