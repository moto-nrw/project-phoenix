package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

func activeInstance(id int64, title string, roomID int64, startMin, endMin int) *ports.ActivityInstance {
	inst := plannedInstance(title, roomID, startMin, endMin)
	inst.ID = id
	inst.Status = ports.InstanceStatusActive
	return inst
}

func runningOverSettings() fakeSettings {
	return fakeSettings{
		bools: map[string]bool{"reminders.activity_running_over_enabled": true},
		ints:  map[string]int{"reminders.activity_running_over_minutes": 10},
	}
}

func TestDueRunningOverBoundaries(t *testing.T) {
	t.Parallel()

	const grace = 10
	// The block ends at 11:40 (700), so it runs over from 11:50 (710).
	block := activeInstance(1, "Fußball-AG", 5, 600, 700)

	t.Run("one minute before the threshold nothing is due, the threshold is the next change", func(t *testing.T) {
		due, next := dueRunningOver([]*ports.ActivityInstance{block}, nil, runningOverWindow{nowMin: 709, grace: grace})
		assert.Empty(t, due)
		assert.Equal(t, 710, next)
	})

	t.Run("at exactly end plus grace the block is due", func(t *testing.T) {
		due, next := dueRunningOver([]*ports.ActivityInstance{block}, nil, runningOverWindow{nowMin: 710, grace: grace})
		require.Len(t, due, 1)
		assert.Equal(t, -1, next, "a due block adds no future boundary; its end is a data change")
	})

	t.Run("at the planned end itself the block is not due yet", func(t *testing.T) {
		due, _ := dueRunningOver([]*ports.ActivityInstance{block}, nil, runningOverWindow{nowMin: 700, grace: grace})
		assert.Empty(t, due)
	})

	t.Run("completed, planned and cancelled blocks never run over", func(t *testing.T) {
		completed := activeInstance(2, "Beendet", 5, 600, 700)
		completed.Status = "completed"
		planned := activeInstance(3, "Nie gestartet", 5, 600, 700)
		planned.Status = ports.InstanceStatusPlanned
		cancelled := activeInstance(4, "Fällt aus", 5, 600, 700)
		cancelled.Status = "cancelled"

		due, next := dueRunningOver([]*ports.ActivityInstance{completed, planned, cancelled}, nil, runningOverWindow{nowMin: 760, grace: grace})
		assert.Empty(t, due)
		assert.Equal(t, -1, next)
	})

	t.Run("a block without a room holds no children and is skipped", func(t *testing.T) {
		duty := activeInstance(5, "Hofaufsicht", 0, 600, 700)
		due, _ := dueRunningOver([]*ports.ActivityInstance{duty}, nil, runningOverWindow{nowMin: 760, grace: grace})
		assert.Empty(t, due)
	})

	t.Run("a spontaneous block has no real planned end and never runs over", func(t *testing.T) {
		spontaneous := activeInstance(6, "Spontan im Hof", 5, 600, 660)
		spontaneous.IsSpontaneous = true
		due, next := dueRunningOver([]*ports.ActivityInstance{spontaneous}, nil, runningOverWindow{nowMin: 760, grace: grace})
		assert.Empty(t, due)
		assert.Equal(t, -1, next)
	})

	t.Run("the scope filter keeps foreign rooms out", func(t *testing.T) {
		filter := activityScopeFilter(map[int64]struct{}{9: {}}, nil)
		due, next := dueRunningOver([]*ports.ActivityInstance{block}, filter, runningOverWindow{nowMin: 705, grace: grace})
		assert.Empty(t, due)
		assert.Equal(t, -1, next, "a block outside the scope must not leak its end time")
	})
}

func TestBuildRunningOverRemindersCountsReachedPickups(t *testing.T) {
	t.Parallel()

	const nowMin = 720
	block := activeInstance(7, "Fußball-AG", 5, 600, 700)
	facts := runningOverFacts{
		roomNames:    map[int64]string{5: "Turnhalle"},
		roomStudents: map[int64][]int64{5: {1, 2, 3, 4}},
		pickupTimes: map[int64]*ports.EffectivePickupTime{
			1: pickupAt(700),        // reached earlier
			2: pickupAt(nowMin),     // reached exactly now
			3: pickupAt(nowMin + 1), // still ahead
			// 4 has no pickup time today
		},
	}

	out := buildRunningOverReminders([]*ports.ActivityInstance{block}, nowMin, facts)
	require.Len(t, out, 1)
	got := out[0]
	assert.Equal(t, reminder.TypeActivityRunningOver, got.Type)
	assert.Equal(t, "Fußball-AG", got.Title)
	assert.Equal(t, "Turnhalle", got.Subtitle)
	assert.Equal(t, "11:40", got.DueTime, "the due time is the planned end")
	assert.Equal(t, -20, got.MinutesAway)
	require.NotNil(t, got.ActivityInstanceID)
	assert.Equal(t, "7", *got.ActivityInstanceID)
	require.NotNil(t, got.PickupDueCount)
	assert.Equal(t, 2, *got.PickupDueCount)
	assert.Nil(t, got.StudentID, "the reminder names no child")
}

func TestComputeRunningOver(t *testing.T) {
	t.Parallel()

	// testClock is 12:00 (720).
	service := func(settings fakeSettings, instances []*ports.ActivityInstance, sup fakeSupervision) *service {
		return &service{QueryDependencies: ports.QueryDependencies{
			Clock:       testClock,
			Settings:    settings,
			Instance:    fakeInstance{instances: instances},
			Room:        fakeRoom{rooms: []*ports.Room{{ID: 5, Name: "Turnhalle"}}},
			Pickup:      fakePickup{times: map[int64]*ports.EffectivePickupTime{1: pickupAt(690), 2: pickupAt(800)}},
			Supervision: sup,
			Visits:      sup,
		}}
	}
	presence := fakeSupervision{presentByRoom: map[int64][]int64{5: {1, 2}}}

	t.Run("an admin sees the running-over block with room and reached pickups", func(t *testing.T) {
		svc := service(runningOverSettings(), []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, presence)
		res, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		require.True(t, res.Enabled)
		require.Len(t, res.Reminders, 1)
		assert.Equal(t, "Turnhalle", res.Reminders[0].Subtitle)
		require.NotNil(t, res.Reminders[0].PickupDueCount)
		assert.Equal(t, 1, *res.Reminders[0].PickupDueCount)
	})

	t.Run("once the block is completed the reminder is gone", func(t *testing.T) {
		completed := activeInstance(7, "Fußball-AG", 5, 600, 700)
		completed.Status = "completed"
		svc := service(runningOverSettings(), []*ports.ActivityInstance{completed}, presence)
		res, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.True(t, res.Enabled)
		assert.Empty(t, res.Reminders)
	})

	t.Run("a block that has not reached the threshold schedules the refetch", func(t *testing.T) {
		svc := service(runningOverSettings(), []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 715)}, presence)
		res, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.Empty(t, res.Reminders)
		assert.Equal(t, "12:05", res.NextChangeAt)
	})

	t.Run("switched off means no reminder", func(t *testing.T) {
		settings := runningOverSettings()
		settings.bools = map[string]bool{"reminders.activity_overdue_enabled": true}
		svc := service(settings, []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, presence)
		res, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		for _, r := range res.Reminders {
			assert.NotEqual(t, reminder.TypeActivityRunningOver, r.Type)
		}
	})

	t.Run("a caregiver sees only blocks in a supervised room", func(t *testing.T) {
		sup := fakeSupervision{
			supervisions:  []*ports.GroupSupervisor{{StaffID: 3, GroupID: 30}},
			groups:        map[int64]*ports.Group{30: {RoomID: 6}},
			presentByRoom: presence.presentByRoom,
		}
		svc := service(runningOverSettings(), []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, sup)
		res, err := svc.Compute(context.Background(), reminder.Scope{StaffID: 3})
		require.NoError(t, err)
		assert.Empty(t, res.Reminders)

		sup.groups = map[int64]*ports.Group{30: {RoomID: 5}}
		svc = service(runningOverSettings(), []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, sup)
		res, err = svc.Compute(context.Background(), reminder.Scope{StaffID: 3})
		require.NoError(t, err)
		require.Len(t, res.Reminders, 1)
	})

	t.Run("a failed grace read fails the request", func(t *testing.T) {
		settings := runningOverSettings()
		settings.intErr = errors.New("settings down")
		svc := service(settings, []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, presence)
		_, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.Error(t, err)
	})

	t.Run("an unresolvable room fails instead of an unnamed reminder", func(t *testing.T) {
		svc := service(runningOverSettings(), []*ports.ActivityInstance{activeInstance(7, "Fußball-AG", 5, 600, 700)}, presence)
		svc.Room = fakeRoom{empty: true}
		_, err := svc.Compute(context.Background(), reminder.Scope{IsAdmin: true})
		require.Error(t, err)
	})
}

func TestComputeBatchMatchesComputeForRunningOver(t *testing.T) {
	t.Parallel()

	_, nowMin := testClock()
	build := func() *world {
		w := newWorld()
		w.settingBools["reminders.activity_running_over_enabled"] = true
		w.settingInts["reminders.activity_running_over_minutes"] = 10
		w.addStudent(1, 100, "Anna", "1a").addStudent(2, 200, "Ben", "2b")
		w.pickupAtMinute(1, nowMin-5).pickupAtMinute(2, nowMin+30)
		w.presentInRoom(10, 1, 2)
		w.supervises(7, 10)
		w.supervises(8, 20)
		w.addInstance(101, "Fußball-AG", 10, nowMin-90, nowMin-15)
		w.addInstance(102, "Theater-AG", 20, nowMin-60, nowMin-5)
		w.instances[0].Status = ports.InstanceStatusActive
		w.instances[1].Status = ports.InstanceStatusActive
		w.roomNames[10] = "Turnhalle"
		return w
	}

	assertBatchMatchesCompute(t, build(), []reminder.BatchScope{admin(1), caregiver(7), caregiver(8), caregiver(99)})

	batch, err := build().service().ComputeBatch(context.Background(), []reminder.BatchScope{caregiver(7), caregiver(8)})
	require.NoError(t, err)
	var running []reminder.Reminder
	for _, r := range batch[7].Reminders {
		if r.Type == reminder.TypeActivityRunningOver {
			running = append(running, r)
		}
	}
	require.Len(t, running, 1)
	assert.Equal(t, "Turnhalle", running[0].Subtitle)
	require.NotNil(t, running[0].PickupDueCount)
	assert.Equal(t, 1, *running[0].PickupDueCount)
	for _, r := range batch[8].Reminders {
		assert.NotEqual(t, reminder.TypeActivityRunningOver, r.Type,
			"Theater-AG is only 5 minutes past its end, below the 10 minute grace")
	}
}

func TestComputeBatchRunningOverLoadsOnlyRecipientRooms(t *testing.T) {
	t.Parallel()

	_, nowMin := testClock()
	w := newWorld()
	w.settingBools = map[string]bool{"reminders.activity_running_over_enabled": true}
	w.settingInts = map[string]int{"reminders.activity_running_over_minutes": 10}
	w.supervises(7, 10)
	w.addInstance(101, "Sichtbare AG", 10, nowMin-90, nowMin-15)
	w.addInstance(102, "Fremde AG", 20, nowMin-90, nowMin-15)
	for _, instance := range w.instances {
		instance.Status = ports.InstanceStatusActive
	}
	w.unresolvedRooms = map[int64]struct{}{20: {}}

	batch, err := w.service().ComputeBatch(context.Background(), []reminder.BatchScope{caregiver(7)})
	require.NoError(t, err)
	require.Len(t, batch[7].Reminders, 1)
	assert.Equal(t, "101", *batch[7].Reminders[0].ActivityInstanceID)

	_, err = w.service().ComputeBatch(context.Background(), []reminder.BatchScope{admin(1)})
	require.Error(t, err, "an admin still needs facts for every room")
}
