package reminders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// --- fakes -------------------------------------------------------------------

type fakeSettings struct {
	bools   map[string]bool
	ints    map[string]int
	boolErr error
	intErr  error
}

func (f fakeSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if f.boolErr != nil {
		return false, f.boolErr
	}
	return f.bools[key], nil
}

func (f fakeSettings) ResolveInt(_ context.Context, key string) (int, error) {
	if f.intErr != nil {
		return 0, f.intErr
	}
	v, ok := f.ints[key]
	if !ok {
		return 0, nil
	}
	return v, nil
}

type fakeAttendance struct {
	ids []int64
	err error
}

func (f fakeAttendance) ListOpenStudentIDsForDate(_ context.Context, _ timezone.Date) ([]int64, error) {
	return f.ids, f.err
}

type fakePickup struct {
	times map[int64]*scheduleService.EffectivePickupTime
	err   error
}

func (f fakePickup) GetBulkEffectivePickupTimesForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	return f.times, f.err
}

type fakeInstance struct {
	instances []*scheduleModel.ActivityInstance
	err       error
}

func (f fakeInstance) FindByTenantAndDate(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return f.instances, f.err
}

type fakeStudent struct {
	students map[int64]*userModel.Student
	err      error
}

func (f fakeStudent) FindByIDs(_ context.Context, _ []int64) (map[int64]*userModel.Student, error) {
	return f.students, f.err
}

type fakePerson struct {
	persons map[int64]*userModel.Person
	err     error
}

func (f fakePerson) FindByIDs(_ context.Context, _ []int64) (map[int64]*userModel.Person, error) {
	return f.persons, f.err
}

type fakeSupervision struct {
	supervisions   []*activeModel.GroupSupervisor
	groups         map[int64]*activeModel.Group
	presentByRoom  map[int64][]int64
	supervisionErr error
}

func (f fakeSupervision) GetStaffActiveSupervisions(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return f.supervisions, f.supervisionErr
}

func (f fakeSupervision) GetActiveGroupsByIDs(_ context.Context, _ []int64) (map[int64]*activeModel.Group, error) {
	return f.groups, nil
}

func (f fakeSupervision) ListStudentsPresentInRoom(_ context.Context, roomID int64) ([]int64, error) {
	return f.presentByRoom[roomID], nil
}

// wallClock builds a time whose Hour/Minute equal the given minute-of-day. The
// date and zone are irrelevant because the service only reads Hour()/Minute().
func wallClock(minuteOfDay int) time.Time {
	return time.Date(2000, 1, 1, minuteOfDay/60, minuteOfDay%60, 0, 0, time.UTC)
}

func pickupAt(minuteOfDay int) *scheduleService.EffectivePickupTime {
	t := wallClock(minuteOfDay)
	return &scheduleService.EffectivePickupTime{PickupTime: &t}
}

func plannedInstance(title string, roomID int64, startMin, endMin int) *scheduleModel.ActivityInstance {
	return &scheduleModel.ActivityInstance{
		Title:     title,
		RoomID:    roomID,
		StartTime: wallClock(startMin),
		EndTime:   wallClock(endMin),
		Status:    scheduleModel.InstanceStatusPlanned,
	}
}

// --- pickupReminders ---------------------------------------------------------

func TestPickupReminders(t *testing.T) {
	const nowMin = 600 // 10:00
	const lead = 10

	students := map[int64]*userModel.Student{
		1: {PersonID: 11, SchoolClass: "1a"},
	}
	persons := map[int64]*userModel.Person{
		11: {FirstName: "Anna", LastName: "Müller"},
	}

	newSvc := func(times map[int64]*scheduleService.EffectivePickupTime) *service {
		return &service{
			pickup:  fakePickup{times: times},
			student: fakeStudent{students: students},
			person:  fakePerson{persons: persons},
		}
	}

	t.Run("upcoming within lead is reported", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)})
		out, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, TypePickupUpcoming, out[0].Type)
		assert.Equal(t, 5, out[0].MinutesAway)
		assert.Equal(t, "10:05", out[0].DueTime)
		assert.Equal(t, "Anna Müller", out[0].Title)
		assert.Equal(t, "1a", out[0].Subtitle)
		require.NotNil(t, out[0].StudentID)
		assert.Equal(t, "1", *out[0].StudentID)
	})

	t.Run("overdue is reported with negative minutes", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(595)})
		out, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, TypePickupOverdue, out[0].Type)
		assert.Equal(t, -5, out[0].MinutesAway)
		assert.Equal(t, "09:55", out[0].DueTime)
	})

	t.Run("beyond lead window is ignored", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(615)}) // +15 > lead 10
		out, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("toggles suppress the disabled type", func(t *testing.T) {
		times := map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605), 2: pickupAt(595)}
		svc := newSvc(times)

		upcomingOnly, err := svc.pickupReminders(context.Background(), []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, false)
		require.NoError(t, err)
		require.Len(t, upcomingOnly, 1)
		assert.Equal(t, TypePickupUpcoming, upcomingOnly[0].Type)

		overdueOnly, err := svc.pickupReminders(context.Background(), []int64{1, 2}, timezone.TodayDate(), nowMin, lead, false, true)
		require.NoError(t, err)
		require.Len(t, overdueOnly, 1)
		assert.Equal(t, TypePickupOverdue, overdueOnly[0].Type)
	})

	t.Run("student without an effective pickup time is skipped", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: nil})
		out, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("empty student list returns nothing", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)})
		out, err := svc.pickupReminders(context.Background(), nil, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("propagates a pickup lookup error", func(t *testing.T) {
		svc := &service{pickup: fakePickup{err: errors.New("boom")}}
		_, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})

	t.Run("propagates a student name lookup error", func(t *testing.T) {
		svc := &service{
			pickup:  fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)}},
			student: fakeStudent{err: errors.New("boom")},
		}
		_, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})

	t.Run("propagates a person name lookup error", func(t *testing.T) {
		svc := &service{
			pickup:  fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)}},
			student: fakeStudent{students: students},
			person:  fakePerson{err: errors.New("boom")},
		}
		_, err := svc.pickupReminders(context.Background(), []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})
}

// --- activityReminders -------------------------------------------------------

func TestActivityReminders(t *testing.T) {
	const nowMin = 600 // 10:00
	const lead = 10
	const overdueThreshold = 5
	adminScope := Scope{IsAdmin: true}

	t.Run("upcoming and overdue are reported, edge cases dropped", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 1, 605, 700),  // +5 upcoming
			plannedInstance("Fußball", 1, 590, 650), // -10 overdue (>= threshold), slot running
			plannedInstance("Basteln", 1, 597, 650), // -3 overdue but < threshold → dropped
			plannedInstance("Lesen", 1, 500, 590),   // overdue but slot ended (end < now) → dropped
			plannedInstance("Turnen", 1, 615, 700),  // +15 > lead → dropped
		}
		// Active (not planned) instance is ignored even though it's "starting now".
		active := plannedInstance("Kochen", 1, 605, 700)
		active.Status = scheduleModel.InstanceStatusActive
		instances = append(instances, active)

		svc := &service{instance: fakeInstance{instances: instances}}
		out, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		require.Len(t, out, 2)

		byTitle := map[string]Reminder{}
		for _, r := range out {
			byTitle[r.Title] = r
		}
		require.Contains(t, byTitle, "Schach")
		assert.Equal(t, TypeActivityStart, byTitle["Schach"].Type)
		assert.Equal(t, 5, byTitle["Schach"].MinutesAway)
		require.Contains(t, byTitle, "Fußball")
		assert.Equal(t, TypeActivityOverdue, byTitle["Fußball"].Type)
		assert.Equal(t, -10, byTitle["Fußball"].MinutesAway)
		assert.Equal(t, "09:50", byTitle["Fußball"].DueTime)
	})

	t.Run("toggles gate each variant independently", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 1, 605, 700),  // upcoming
			plannedInstance("Fußball", 1, 590, 650), // overdue
		}
		svc := &service{instance: fakeInstance{instances: instances}}

		upcomingOnly, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, false)
		require.NoError(t, err)
		require.Len(t, upcomingOnly, 1)
		assert.Equal(t, TypeActivityStart, upcomingOnly[0].Type)

		overdueOnly, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, false, true)
		require.NoError(t, err)
		require.Len(t, overdueOnly, 1)
		assert.Equal(t, TypeActivityOverdue, overdueOnly[0].Type)
	})

	t.Run("caregiver only sees activities in supervised rooms", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 10, 605, 700),  // room 10 — supervised
			plannedInstance("Fußball", 99, 605, 700), // room 99 — not supervised
		}
		svc := &service{instance: fakeInstance{instances: instances}}
		out, err := svc.activityReminders(context.Background(), Scope{IsAdmin: false}, []int64{10}, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "Schach", out[0].Title)
	})

	t.Run("nil instance reader yields nothing", func(t *testing.T) {
		svc := &service{}
		out, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

// --- resolveScope ------------------------------------------------------------

func TestResolveScope(t *testing.T) {
	t.Run("admin sees all present students and no room filter", func(t *testing.T) {
		svc := &service{attendance: fakeAttendance{ids: []int64{1, 2, 3}}}
		students, rooms, err := svc.resolveScope(context.Background(), Scope{IsAdmin: true}, timezone.TodayDate())
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, students)
		assert.Nil(t, rooms)
	})

	t.Run("caregiver sees students present in supervised rooms", func(t *testing.T) {
		svc := &service{supervision: fakeSupervision{
			supervisions: []*activeModel.GroupSupervisor{{GroupID: 100}, {GroupID: 200}},
			groups: map[int64]*activeModel.Group{
				100: {RoomID: 10},
				200: {RoomID: 20},
			},
			presentByRoom: map[int64][]int64{10: {1, 2}, 20: {2, 3}},
		}}
		students, rooms, err := svc.resolveScope(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3}, students)
		assert.ElementsMatch(t, []int64{10, 20}, rooms)
	})

	t.Run("caregiver with no supervision sees nothing", func(t *testing.T) {
		svc := &service{supervision: fakeSupervision{supervisions: nil}}
		students, rooms, err := svc.resolveScope(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.Empty(t, students)
		assert.Empty(t, rooms)
	})

	t.Run("propagates a supervision error", func(t *testing.T) {
		svc := &service{supervision: fakeSupervision{supervisionErr: errors.New("boom")}}
		_, _, err := svc.resolveScope(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.Error(t, err)
	})
}

// --- threshold helpers -------------------------------------------------------

func TestLeadAndThresholdFallbacks(t *testing.T) {
	ctx := context.Background()

	t.Run("configured value wins", func(t *testing.T) {
		svc := &service{settings: fakeSettings{ints: map[string]int{
			configModel.KeyRemindersPickupUpcomingLeadMinutes: 25,
			configModel.KeyTimetableOverdueThresholdMinutes:   7,
		}}}
		lead, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.NoError(t, err)
		assert.Equal(t, 25, lead)
		threshold, err := svc.overdueThresholdMinutes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 7, threshold)
	})

	t.Run("non-positive value falls back", func(t *testing.T) {
		svc := &service{settings: fakeSettings{ints: map[string]int{
			configModel.KeyRemindersPickupUpcomingLeadMinutes: 0,
			configModel.KeyTimetableOverdueThresholdMinutes:   0,
		}}}
		lead, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.NoError(t, err)
		assert.Equal(t, 10, lead)
		threshold, err := svc.overdueThresholdMinutes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, threshold)
	})

	t.Run("lookup error is propagated, not silently defaulted", func(t *testing.T) {
		resolveErr := errors.New("boom")
		svc := &service{settings: fakeSettings{intErr: resolveErr}}
		_, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.ErrorIs(t, err, resolveErr)
		_, err = svc.overdueThresholdMinutes(ctx)
		require.ErrorIs(t, err, resolveErr)
	})
}

// --- Compute (gating, enabled flag, sorting) ---------------------------------

func TestComputeGating(t *testing.T) {
	ctx := context.Background()

	t.Run("nil settings returns an empty, disabled result", func(t *testing.T) {
		svc := &service{}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.False(t, res.Enabled)
		assert.Empty(t, res.Reminders)
		assert.Equal(t, 0, res.Count)
	})

	t.Run("all types off returns an empty, disabled result", func(t *testing.T) {
		svc := &service{settings: fakeSettings{}}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.False(t, res.Enabled)
		assert.Empty(t, res.Reminders)
	})

	t.Run("enabled with no data is enabled but empty", func(t *testing.T) {
		svc := &service{
			settings:   fakeSettings{bools: map[string]bool{configModel.KeyRemindersPickupUpcomingEnabled: true}},
			attendance: fakeAttendance{ids: nil},
		}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.True(t, res.Enabled, "the sidebar/bell shows the entry whenever a type is enabled, even with nothing due")
		assert.Empty(t, res.Reminders)
		assert.Equal(t, 0, res.Count)
	})

	t.Run("a setting resolution error is surfaced, not swallowed as disabled", func(t *testing.T) {
		resolveErr := errors.New("config read failed")
		svc := &service{settings: fakeSettings{boolErr: resolveErr}}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.Error(t, err, "a broken config read must not look like a healthy empty result")
		assert.ErrorIs(t, err, resolveErr)
		assert.Nil(t, res)
	})
}

func TestComputeSortsMostUrgentFirst(t *testing.T) {
	now := timezone.Now()
	nowMin := now.Hour()*60 + now.Minute()
	// Guard the wall-clock arithmetic against day wrap near midnight.
	if nowMin < 30 || nowMin > 1410 {
		t.Skip("skipping wall-clock-relative sort test near midnight")
	}

	svc := &service{
		settings: fakeSettings{bools: map[string]bool{
			configModel.KeyRemindersPickupUpcomingEnabled: true,
			configModel.KeyRemindersPickupOverdueEnabled:  true,
		}},
		attendance: fakeAttendance{ids: []int64{1, 2}},
		pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{
			1: pickupAt(nowMin + 5),  // upcoming
			2: pickupAt(nowMin - 15), // overdue, more urgent
		}},
		student: fakeStudent{students: map[int64]*userModel.Student{
			1: {PersonID: 11, SchoolClass: "1a"},
			2: {PersonID: 12, SchoolClass: "1b"},
		}},
		person: fakePerson{persons: map[int64]*userModel.Person{
			11: {FirstName: "Anna", LastName: "A"},
			12: {FirstName: "Ben", LastName: "B"},
		}},
	}

	res, err := svc.Compute(context.Background(), Scope{IsAdmin: true})
	require.NoError(t, err)
	require.Len(t, res.Reminders, 2)
	assert.True(t, res.Enabled)
	assert.Equal(t, 2, res.Count)
	// Overdue (negative minutes) sorts before the upcoming one.
	assert.Equal(t, TypePickupOverdue, res.Reminders[0].Type)
	assert.Equal(t, TypePickupUpcoming, res.Reminders[1].Type)
	assert.Less(t, res.Reminders[0].MinutesAway, res.Reminders[1].MinutesAway)
}

// --- pure helpers ------------------------------------------------------------

func TestPureHelpers(t *testing.T) {
	assert.Equal(t, "10:05", formatMinutes(605))
	assert.Equal(t, "00:00", formatMinutes(0))
	assert.Equal(t, "09:55", formatMinutes(595))
	assert.Equal(t, 605, minutesOfDay(wallClock(605)))
	assert.Equal(t, "7", *studentIDString(7))
}
