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

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// overdueSetup wires a scheduler instance just enough to exercise the
// overdue tick. Real DB + real settings service; only the broadcaster is
// faked.
type overdueSetup struct {
	sched *Scheduler
	db    *bun.DB
	ctx   context.Context
	spy   *testpkg.RecordingBroadcaster
	room  int64
	// now is a fixed wall-clock anchor (noon UTC today) used by both seedPlanned
	// and the test calls to runOverdueForTenant. Anchoring at noon keeps any
	// reasonable backstep inside the same UTC day, so fixtures that subtract
	// `minutesAgo` from this anchor cannot cross midnight UTC and produce a
	// StartTime that lands tomorrow (which would compute as ~23.5h in the future
	// and silently drop the broadcast).
	now time.Time
}

func buildOverdue(t *testing.T) *overdueSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repoFactory := repositories.NewFactory(db)

	spy := testpkg.NewRecordingBroadcaster()
	sched := unitScheduler(&Scheduler{
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),
		logger: slog.Default()})

	sched.SetInstanceOverdueDeps(repoFactory.ActivityInstance, repoFactory.Room, spy)

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("OVR-Room-%d", time.Now().UnixNano()))
	today := time.Now().UTC()
	anchor := time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, time.UTC)
	return &overdueSetup{
		sched: sched,
		db:    db,
		ctx:   testpkg.Ctx(t),
		spy:   spy,
		room:  room.ID,
		now:   anchor,
	}
}

// seedPlanned inserts one planned activity_instance with a StartTime of
// `minutesAgo` minutes before s.now, so callers can control the overdue
// margin relative to the default 5-minute threshold. Uses s.now (noon UTC
// anchor) rather than time.Now() to keep the fixture deterministic
// regardless of when the suite runs — see overdueSetup.now.
func seedPlanned(t *testing.T, s *overdueSetup, minutesAgo int) *scheduleModels.ActivityInstance {
	t.Helper()
	start := s.now.Add(-time.Duration(minutesAgo) * time.Minute)

	ai := &scheduleModels.ActivityInstance{
		Date:          timezone.DateFromTime(s.now),
		Title:         fmt.Sprintf("OVR-%d", time.Now().UnixNano()),
		StartTime:     time.Date(1, 1, 1, start.Hour(), start.Minute(), start.Second(), 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 23, 59, 0, 0, time.UTC),
		RoomID:        s.room,
		Status:        scheduleModels.InstanceStatusPlanned,
		IsSpontaneous: true, // avoids needing a template FK
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	return ai
}

// setStatus forces the status column (for the active-not-overdue branch).
func setStatus(t *testing.T, s *overdueSetup, id int64, status string) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", status).
		Where(`"activity_instance".id = ?`, id).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(s.ctx)
	require.NoError(t, err)
}

// The tests call runOverdueForTenant directly with the same tenant ctx used
// to seed fixtures (testpkg.Ctx(t)). checkAndRunOverdue's tenant-
// iteration relies on a SchoolRepo + SettingsService stack we'd otherwise
// need to stand up; the per-tenant helper is the real unit of behaviour
// and is what the iteration delegates to in production.

func TestOverdueTick_BroadcastsOncePerInstance(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	// 30 minutes past — well beyond the 5-minute default threshold.
	ai := seedPlanned(t, s, 30)

	s.sched.runOverdueForTenant(s.ctx, testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtime.EventInstanceOverdue), "one overdue broadcast for the seeded instance expected")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtime.EventActiveSupervisionChanged), "one active supervision refresh broadcast expected")

	evt := spyFindByInstance(s.spy, ai.ID, realtime.EventInstanceOverdue)
	require.NotNil(t, evt, "expected broadcast for instance id %d", ai.ID)
	assert.Equal(t, realtime.EventInstanceOverdue, evt.Type)
	require.NotNil(t, evt.Data.InstanceID)
	assert.Equal(t, fmt.Sprintf("%d", ai.ID), *evt.Data.InstanceID)

	refresh := spyFindByInstance(s.spy, ai.ID, realtime.EventActiveSupervisionChanged)
	require.NotNil(t, refresh, "expected active supervision refresh for instance id %d", ai.ID)
	require.NotNil(t, refresh.Data.Reason)
	assert.Equal(t, "instance_overdue", *refresh.Data.Reason)
}

func TestOverdueTick_ReFireGuard(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)

	s.sched.runOverdueForTenant(s.ctx, testpkg.Tenant(t), 5, s.now)
	s.sched.runOverdueForTenant(s.ctx, testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtime.EventInstanceOverdue), "second tick on same instance must suppress overdue event")
	assert.Equal(t, 1, spyFilter(s.spy, ai.ID, realtime.EventActiveSupervisionChanged), "second tick on same instance must suppress active supervision refresh")
}

func TestOverdueTick_ActiveInstancesNotBroadcast(t *testing.T) {
	t.Parallel()
	s := buildOverdue(t)

	ai := seedPlanned(t, s, 30)
	setStatus(t, s, ai.ID, scheduleModels.InstanceStatusActive)

	s.sched.runOverdueForTenant(s.ctx, testpkg.Tenant(t), 5, s.now)

	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, realtime.EventInstanceOverdue), "active instances must not trigger overdue broadcast")
	assert.Equal(t, 0, spyFilter(s.spy, ai.ID, realtime.EventActiveSupervisionChanged), "active instances must not trigger active supervision refresh")
}

// spyFilter counts broadcasts whose InstanceID matches the given id. Needed
// because the test DB is shared across runs and may contain unrelated
// today-dated rows from other tests; we care only about our fixture's
// fire count, not the grand total. The overdue tick only ever calls
// BroadcastToTenant, so filtering the "tenant" method reproduces the old
// spy's b.all semantics.
func spyFilter(b *testpkg.RecordingBroadcaster, instanceID int64, eventType realtime.EventType) int {
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
func spyFindByInstance(b *testpkg.RecordingBroadcaster, instanceID int64, eventType realtime.EventType) *realtime.Event {
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
	yesterday := timezone.NewDate(2026, 4, 19)
	sched.overdueEmittedDay = yesterday
	key := overdueKey{tenantID: 1, instanceID: 42}
	sched.overdueEmitted.Store(key, yesterday.UTCMidnight().Add(14*time.Hour))

	// Rotate using "today" → cache must clear and the day must advance.
	today := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	sched.rotateOverdueCacheIfNewDay(today)

	_, present := sched.overdueEmitted.Load(key)
	assert.False(t, present, "entry from yesterday must be evicted on day rollover")
	assert.Equal(t, timezone.NewDate(2026, 4, 20), sched.overdueEmittedDay)
}
