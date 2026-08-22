package reminders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// --- fakes -------------------------------------------------------------------

type fakeSettings struct {
	bools   map[string]bool
	ints    map[string]int
	strings map[string]string
	boolErr error
	intErr  error
	strErr  error
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

func (f fakeSettings) ResolveString(_ context.Context, key string) (string, error) {
	if f.strErr != nil {
		return "", f.strErr
	}
	return f.strings[key], nil
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

type fakeRoom struct {
	rooms []*facilitiesModel.Room
	err   error
	empty bool
}

func (f fakeRoom) FindByIDs(_ context.Context, ids []int64) ([]*facilitiesModel.Room, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return []*facilitiesModel.Room{}, nil
	}
	if f.rooms != nil {
		return f.rooms, nil
	}
	rooms := make([]*facilitiesModel.Room, 0, len(ids))
	for _, id := range ids {
		rooms = append(rooms, &facilitiesModel.Room{Model: modelBase.Model{ID: id}, Name: "Lernraum"})
	}
	return rooms, nil
}

func (f fakeInstance) FindByTenantAndDate(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return f.instances, f.err
}

type fakeStudent struct {
	students map[int64]*userModel.Student
	err      error
}

func (f fakeStudent) FindReadScopeByIDs(_ context.Context, _ []int64) (map[int64]*userModel.Student, error) {
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
	t.Parallel()

	const nowMin = 600 // 10:00
	const lead = 10

	students := map[int64]*userModel.Student{
		1: {PersonID: 11, SchoolClass: "1a"},
	}
	persons := map[int64]*userModel.Person{
		11: {FirstName: "Anna", LastName: "Müller"},
	}

	newSvc := func(times map[int64]*scheduleService.EffectivePickupTime) *service {
		return &service{Dependencies: Dependencies{Pickup: fakePickup{times: times}, Student: fakeStudent{students: students}, Person: fakePerson{persons: persons}}}
	}

	t.Run("upcoming within lead is reported", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)})
		out, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
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
		out, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, TypePickupOverdue, out[0].Type)
		assert.Equal(t, -5, out[0].MinutesAway)
		assert.Equal(t, "09:55", out[0].DueTime)
	})

	t.Run("beyond lead window is ignored", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(615)}) // +15 > lead 10
		out, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("toggles suppress the disabled type", func(t *testing.T) {
		times := map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605), 2: pickupAt(595)}
		// Both students need a record: the read path is fail-closed, so a due
		// student without a resolvable record emits nothing (which would mask the
		// toggle behavior this test exercises).
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: times}, Student: fakeStudent{students: map[int64]*userModel.Student{
			1: {PersonID: 11, SchoolClass: "1a"},
			2: {PersonID: 12, SchoolClass: "1b"},
		}}, Person: fakePerson{persons: map[int64]*userModel.Person{
			11: {FirstName: "Anna", LastName: "Müller"},
			12: {FirstName: "Ben", LastName: "Bauer"},
		}}},
		}

		upcomingOnly, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, false)
		require.NoError(t, err)
		require.Len(t, upcomingOnly, 1)
		assert.Equal(t, TypePickupUpcoming, upcomingOnly[0].Type)

		overdueOnly, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, false, true)
		require.NoError(t, err)
		require.Len(t, overdueOnly, 1)
		assert.Equal(t, TypePickupOverdue, overdueOnly[0].Type)
	})

	t.Run("student without an effective pickup time is skipped", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: nil})
		out, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("empty student list returns nothing", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)})
		out, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, nil, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("propagates a pickup lookup error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{err: errors.New("boom")}}}
		_, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})

	t.Run("propagates a student name lookup error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)}}, Student: fakeStudent{err: errors.New("boom")}}}
		_, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})

	t.Run("propagates a person name lookup error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)}}, Student: fakeStudent{students: students}, Person: fakePerson{err: errors.New("boom")}}}
		_, _, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.Error(t, err)
	})

	// --- read access (#2329) -------------------------------------------------
	// Every reminder recipient is staff, and staff read every child of their
	// tenant. A caregiver therefore sees the pickup of each present child in
	// their rooms, whatever education group the child belongs to (or none).

	g100 := int64(100)
	g200 := int64(200)
	twoDueStudents := map[int64]*userModel.Student{
		1: {PersonID: 11, SchoolClass: "1a", GroupID: &g100},
		2: {PersonID: 12, SchoolClass: "1b", GroupID: &g200},
	}
	twoPersons := map[int64]*userModel.Person{
		11: {FirstName: "Anna", LastName: "A"},
		12: {FirstName: "Ben", LastName: "B"},
	}
	twoDueTimes := map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605), 2: pickupAt(605)}
	caregiver := Scope{IsAdmin: false, StaffID: 7}

	t.Run("caregiver sees every present student regardless of group", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: twoDueTimes}, Student: fakeStudent{students: twoDueStudents}, Person: fakePerson{persons: twoPersons}, Settings: fakeSettings{}}}
		out, _, err := svc.pickupReminders(context.Background(), caregiver, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 2)
	})

	t.Run("a student the read-scope query does not return stays out", func(t *testing.T) {
		// FindReadScopeByIDs is the read boundary: a child it omits (deleted,
		// outside the tenant, filtered by RLS) must not surface a reminder.
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: twoDueTimes}, Student: fakeStudent{students: map[int64]*userModel.Student{
			1: twoDueStudents[1],
		}}, Person: fakePerson{persons: twoPersons}, Settings: fakeSettings{}}}
		out, _, err := svc.pickupReminders(context.Background(), caregiver, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.NotNil(t, out[0].StudentID)
		assert.Equal(t, "1", *out[0].StudentID)
		assert.Equal(t, "Anna A", out[0].Title)
	})

	t.Run("a student read error is surfaced, not treated as no-access", func(t *testing.T) {
		loadErr := errors.New("student read failed")
		svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: twoDueTimes}, Student: fakeStudent{err: loadErr}, Person: fakePerson{persons: twoPersons}, Settings: fakeSettings{}}}
		_, _, err := svc.pickupReminders(context.Background(), caregiver, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, true)
		require.ErrorIs(t, err, loadErr)
	})
}

// --- activityReminders -------------------------------------------------------

func TestActivityReminders(t *testing.T) {
	t.Parallel()

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
		instances[0].ID = 101
		instances[1].ID = 102

		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: instances}, Room: fakeRoom{}}}
		out, _, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		require.Len(t, out, 2)

		byTitle := map[string]Reminder{}
		for _, r := range out {
			byTitle[r.Title] = r
		}
		require.Contains(t, byTitle, "Schach")
		assert.Equal(t, TypeActivityStart, byTitle["Schach"].Type)
		assert.Equal(t, 5, byTitle["Schach"].MinutesAway)
		require.NotNil(t, byTitle["Schach"].ActivityInstanceID)
		assert.Equal(t, "101", *byTitle["Schach"].ActivityInstanceID)
		require.Contains(t, byTitle, "Fußball")
		assert.Equal(t, TypeActivityOverdue, byTitle["Fußball"].Type)
		assert.Equal(t, -10, byTitle["Fußball"].MinutesAway)
		assert.Equal(t, "09:50", byTitle["Fußball"].DueTime)
		require.NotNil(t, byTitle["Fußball"].ActivityInstanceID)
		assert.Equal(t, "102", *byTitle["Fußball"].ActivityInstanceID)
	})

	t.Run("toggles gate each variant independently", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 1, 605, 700),  // upcoming
			plannedInstance("Fußball", 1, 590, 650), // overdue
		}
		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: instances}, Room: fakeRoom{}}}

		upcomingOnly, _, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, false)
		require.NoError(t, err)
		require.Len(t, upcomingOnly, 1)
		assert.Equal(t, TypeActivityStart, upcomingOnly[0].Type)

		overdueOnly, _, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, false, true)
		require.NoError(t, err)
		require.Len(t, overdueOnly, 1)
		assert.Equal(t, TypeActivityOverdue, overdueOnly[0].Type)
	})

	t.Run("caregiver only sees activities in supervised rooms", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 10, 605, 700),  // room 10 — supervised
			plannedInstance("Fußball", 99, 605, 700), // room 99 — not supervised
		}
		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: instances}, Room: fakeRoom{}}}
		out, _, err := svc.activityReminders(context.Background(), Scope{IsAdmin: false}, []int64{10}, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, "Schach", out[0].Title)
	})

	t.Run("Schulhof becomes overdue like any other room", func(t *testing.T) {
		// Since #2161 the Schulhof is a regular plannable room: planned yard
		// blocks produce overdue reminders exactly like indoor blocks.
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Lernzeit", 1, 590, 650),
			plannedInstance("Schulhof-Block", 2, 590, 640),
		}
		rooms := fakeRoom{rooms: []*facilitiesModel.Room{
			{Model: modelBase.Model{ID: 1}, Name: "Lernraum"},
			{Model: modelBase.Model{ID: 2}, Name: constants.SchulhofRoomName},
		}}
		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: instances}, Room: rooms}}

		overdueOnly, next, err := svc.activityReminders(
			context.Background(), adminScope, nil, timezone.TodayDate(), nowMin,
			lead, overdueThreshold, false, true,
		)

		require.NoError(t, err)
		require.Len(t, overdueOnly, 2)
		titles := []string{overdueOnly[0].Title, overdueOnly[1].Title}
		assert.ElementsMatch(t, []string{"Lernzeit", "Schulhof-Block"}, titles)
		assert.Equal(t, 640, next, "the earlier Schulhof end time schedules the next overdue refetch")
	})

	t.Run("room resolution failures suppress activity reminders", func(t *testing.T) {
		instances := []*scheduleModel.ActivityInstance{
			plannedInstance("Unbekannter Raum", 7, 590, 650),
		}
		for _, room := range []fakeRoom{
			{err: errors.New("rooms unavailable")},
			{empty: true},
		} {
			svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: instances}, Room: room}}
			out, next, err := svc.activityReminders(
				context.Background(), adminScope, nil, timezone.TodayDate(), nowMin,
				lead, overdueThreshold, false, true,
			)
			require.Error(t, err)
			assert.Empty(t, out)
			assert.Equal(t, -1, next)
		}
	})

	t.Run("nil instance reader yields nothing", func(t *testing.T) {
		svc := &service{}
		out, _, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

// --- pickup / activity scope resolution --------------------------------------

func TestPickupScopeStudentIDs(t *testing.T) {
	t.Parallel()

	t.Run("admin sees all present students", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Attendance: fakeAttendance{ids: []int64{1, 2, 3}}}}
		ids, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: true}, timezone.TodayDate())
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("caregiver sees students present in supervised rooms", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{
			supervisions: []*activeModel.GroupSupervisor{{GroupID: 100}, {GroupID: 200}},
			groups: map[int64]*activeModel.Group{
				100: {RoomID: 10},
				200: {RoomID: 20},
			},
			presentByRoom: map[int64][]int64{10: {1, 2}, 20: {2, 3}},
		}}}
		ids, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3}, ids)
	})

	t.Run("caregiver with no supervision sees nothing", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{supervisions: nil}}}
		ids, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("propagates a supervision error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{supervisionErr: errors.New("boom")}}}
		_, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.Error(t, err)
	})

	t.Run("binary mode caregiver reads presence from attendance, not visits", func(t *testing.T) {
		// Binary-mode tenants no-op CreateVisit, so there are no active.visits
		// rows: the room path would return empty. The scope must fall back to
		// open attendance (the read predicate in pickupReminders then locks it
		// down to the caregiver's own groups).
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{strings: map[string]string{
			configModel.KeyPresenceMode: configModel.PresenceModeBinary,
		}}, Attendance: fakeAttendance{ids: []int64{1, 2, 3}}, Supervision:
		// Supervision would return nothing (no visits) — proving attendance won.
		fakeSupervision{presentByRoom: map[int64][]int64{}}},
		}
		ids, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("detailed mode caregiver still reads presence from supervised rooms", func(t *testing.T) {
		// Explicit detailed mode must keep the room/visit path, not attendance.
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{strings: map[string]string{
			configModel.KeyPresenceMode: configModel.PresenceModeDetailed,
		}}, Attendance: fakeAttendance{ids: []int64{99}}, Supervision: // must be ignored
		fakeSupervision{
			supervisions:  []*activeModel.GroupSupervisor{{GroupID: 100}},
			groups:        map[int64]*activeModel.Group{100: {RoomID: 10}},
			presentByRoom: map[int64][]int64{10: {1, 2}},
		}},
		}
		ids, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, ids)
	})

	t.Run("surfaces a presence-mode resolution error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{strErr: errors.New("boom")}}}
		_, err := svc.pickupScopeStudentIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7}, timezone.TodayDate())
		require.Error(t, err)
	})
}

func TestSupervisedRoomIDs(t *testing.T) {
	t.Parallel()

	t.Run("returns the deduplicated supervised rooms", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{
			supervisions: []*activeModel.GroupSupervisor{{GroupID: 100}, {GroupID: 200}},
			groups: map[int64]*activeModel.Group{
				100: {RoomID: 10},
				200: {RoomID: 20},
			},
		}}}
		rooms, err := svc.supervisedRoomIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7})
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{10, 20}, rooms)
	})

	t.Run("no supervision yields no rooms", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{supervisions: nil}}}
		rooms, err := svc.supervisedRoomIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7})
		require.NoError(t, err)
		assert.Empty(t, rooms)
	})

	t.Run("propagates a supervision error", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Supervision: fakeSupervision{supervisionErr: errors.New("boom")}}}
		_, err := svc.supervisedRoomIDs(context.Background(), Scope{IsAdmin: false, StaffID: 7})
		require.Error(t, err)
	})
}

// --- threshold helpers -------------------------------------------------------

func TestLeadAndThresholdFallbacks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("configured value wins", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{ints: map[string]int{
			configModel.KeyRemindersPickupUpcomingLeadMinutes: 25,
			configModel.KeyTimetableOverdueThresholdMinutes:   7,
		}}}}
		lead, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.NoError(t, err)
		assert.Equal(t, 25, lead)
		threshold, err := svc.overdueThresholdMinutes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 7, threshold)
	})

	t.Run("non-positive value falls back", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{ints: map[string]int{
			configModel.KeyRemindersPickupUpcomingLeadMinutes: 0,
			configModel.KeyTimetableOverdueThresholdMinutes:   0,
		}}}}
		lead, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.NoError(t, err)
		assert.Equal(t, 10, lead)
		threshold, err := svc.overdueThresholdMinutes(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, threshold)
	})

	t.Run("lookup error is propagated, not silently defaulted", func(t *testing.T) {
		resolveErr := errors.New("boom")
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{intErr: resolveErr}}}
		_, err := svc.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		require.ErrorIs(t, err, resolveErr)
		_, err = svc.overdueThresholdMinutes(ctx)
		require.ErrorIs(t, err, resolveErr)
	})
}

// --- Compute (gating, enabled flag, sorting) ---------------------------------

func TestComputeGating(t *testing.T) {
	t.Parallel()

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
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{}}}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.False(t, res.Enabled)
		assert.Empty(t, res.Reminders)
	})

	t.Run("enabled with no data is enabled but empty", func(t *testing.T) {
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{bools: map[string]bool{configModel.KeyRemindersPickupUpcomingEnabled: true}}, Attendance: fakeAttendance{ids: nil}}}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.NoError(t, err)
		assert.True(t, res.Enabled, "the header bell shows whenever a type is enabled, even with nothing due")
		assert.Empty(t, res.Reminders)
		assert.Equal(t, 0, res.Count)
	})

	t.Run("a setting resolution error is surfaced, not swallowed as disabled", func(t *testing.T) {
		resolveErr := errors.New("config read failed")
		svc := &service{Dependencies: Dependencies{Settings: fakeSettings{boolErr: resolveErr}}}
		res, err := svc.Compute(ctx, Scope{IsAdmin: true})
		require.Error(t, err, "a broken config read must not look like a healthy empty result")
		assert.ErrorIs(t, err, resolveErr)
		assert.Nil(t, res)
	})
}

func TestComputeSortsMostUrgentFirst(t *testing.T) {
	t.Parallel()

	now := timezone.Now()
	nowMin := now.Hour()*60 + now.Minute()
	// Guard the wall-clock arithmetic against day wrap near midnight.
	if nowMin < 30 || nowMin > 1410 {
		t.Skip("skipping wall-clock-relative sort test near midnight")
	}

	svc := &service{Dependencies: Dependencies{Settings: fakeSettings{bools: map[string]bool{
		configModel.KeyRemindersPickupUpcomingEnabled: true,
		configModel.KeyRemindersPickupOverdueEnabled:  true,
	}}, Attendance: fakeAttendance{ids: []int64{1, 2}}, Pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{
		1: pickupAt(nowMin + 5),  // upcoming
		2: pickupAt(nowMin - 15), // overdue, more urgent
	}}, Student: fakeStudent{students: map[int64]*userModel.Student{
		1: {PersonID: 11, SchoolClass: "1a"},
		2: {PersonID: 12, SchoolClass: "1b"},
	}}, Person: fakePerson{persons: map[int64]*userModel.Person{
		11: {FirstName: "Anna", LastName: "A"},
		12: {FirstName: "Ben", LastName: "B"},
	}}},
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

// --- next-change (time-based refresh scheduling) -----------------------------
// The frontend schedules a timer to NextChangeAt to refetch exactly when a
// reminder crosses a threshold, instead of only on its fixed poll. These tests
// pin the boundary math the timer depends on.

func TestNextChangeMinFutureHelper(t *testing.T) {
	t.Parallel()

	assert.Equal(t, -1, futureBoundary(600, 600), "a boundary at now is not in the future")
	assert.Equal(t, -1, futureBoundary(590, 600), "a past boundary yields the absent sentinel")
	assert.Equal(t, 610, futureBoundary(610, 600))

	assert.Equal(t, 5, minFuture(-1, 5), "first candidate replaces the absent sentinel")
	assert.Equal(t, 5, minFuture(5, -1), "an absent candidate leaves the current value")
	assert.Equal(t, 5, minFuture(8, 5), "the sooner candidate wins")
	assert.Equal(t, 5, minFuture(5, 8))
	assert.Equal(t, -1, minFuture(-1, -1), "nothing pending stays absent")
}

func TestPickupNextChange(t *testing.T) {
	t.Parallel()

	const nowMin = 600 // 10:00
	const lead = 10

	students := map[int64]*userModel.Student{1: {PersonID: 11, SchoolClass: "1a"}}
	persons := map[int64]*userModel.Person{11: {FirstName: "Anna", LastName: "A"}}
	newSvc := func(times map[int64]*scheduleService.EffectivePickupTime) *service {
		return &service{Dependencies: Dependencies{Pickup: fakePickup{times: times}, Student: fakeStudent{students: students}, Person: fakePerson{persons: persons}}}
	}

	t.Run("a pickup outside the window schedules its window-entry moment", func(t *testing.T) {
		// pickup 20 min away, lead 10 → not due yet, but enters the window at
		// pickupMin-lead = 610. That is the next change even though `out` is empty.
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(620)})
		out, next, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Equal(t, 610, next)
	})

	t.Run("an in-window upcoming pickup schedules its flip-to-overdue moment", func(t *testing.T) {
		// pickup 5 min away: window-entry (595) is already past. At pickupMin (605)
		// the diff is still 0 (upcoming); the flip to overdue is the first minute
		// pickupMin is in the past, pickupMin+1 = 606.
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(605)})
		out, next, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, 606, next)
	})

	t.Run("an already-overdue pickup has no future time-based change", func(t *testing.T) {
		// Overdue persists until the child is picked up (a data event, not a clock
		// tick), so there is nothing to schedule.
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(595)})
		_, next, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, true, true)
		require.NoError(t, err)
		assert.Equal(t, -1, next)
	})

	t.Run("overdue-only schedules the flip, never the window entry", func(t *testing.T) {
		svc := newSvc(map[int64]*scheduleService.EffectivePickupTime{1: pickupAt(620)})
		_, next, err := svc.pickupReminders(context.Background(), Scope{IsAdmin: true}, []int64{1}, timezone.TodayDate(), nowMin, lead, false, true)
		require.NoError(t, err)
		assert.Equal(t, 621, next, "overdue-only: no window entry, only the flip at pickupMin+1 (first overdue minute)")
	})
}

// TestPickupNextChangeExcludesUnreadableStudents guards the GDPR gate on the
// next-change timer: a child outside the caller's read scope must not leak its
// future pickup minute through next_change_at, even though its reminder is
// never emitted. Student 2 — omitted by FindReadScopeByIDs, the read boundary —
// has the SOONER boundary; only student 1's must survive.
func TestPickupNextChangeExcludesUnreadableStudents(t *testing.T) {
	t.Parallel()

	const nowMin = 600 // 10:00
	const lead = 10
	g100 := int64(100)

	svc := &service{Dependencies: Dependencies{Pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{
		1: pickupAt(660), // readable: enters its window at 650
		2: pickupAt(620), // outside the read scope: would enter at 610 (sooner) — must not leak
	}}, Student: fakeStudent{students: map[int64]*userModel.Student{
		1: {PersonID: 11, SchoolClass: "1a", GroupID: &g100},
	}}, Person: fakePerson{persons: map[int64]*userModel.Person{11: {FirstName: "Anna", LastName: "A"}}}, Settings: fakeSettings{}},
	}
	caregiver := Scope{IsAdmin: false, StaffID: 7}

	out, next, err := svc.pickupReminders(context.Background(), caregiver, []int64{1, 2}, timezone.TodayDate(), nowMin, lead, true, true)
	require.NoError(t, err)
	assert.Empty(t, out, "neither pickup is inside its window yet")
	assert.Equal(t, 650, next, "only the readable child's boundary may schedule the refetch")
}

func TestActivityNextChange(t *testing.T) {
	t.Parallel()

	const nowMin = 600 // 10:00
	const lead = 10
	const overdueThreshold = 5
	adminScope := Scope{IsAdmin: true}

	t.Run("an activity starting beyond the window schedules its earliest boundary", func(t *testing.T) {
		// start 620, end 700. Boundaries: window entry 610, upcoming exit 621,
		// overdue 625, slot end 700. The soonest is 610.
		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 1, 620, 700),
		}}, Room: fakeRoom{}}}
		out, next, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Equal(t, 610, next)
	})

	t.Run("an in-window upcoming activity schedules its flip-out at startMin+1", func(t *testing.T) {
		// now 615, window [610, 620]: entry already past. The start reminder is
		// still shown at exactly startMin (diff 0 is upcoming); it disappears the
		// first minute start is in the past, startMin+1 = 621 — NOT startMin.
		// Upcoming-only isolates that boundary from the overdue ones.
		const nowMin = 615
		svc := &service{Dependencies: Dependencies{Instance: fakeInstance{instances: []*scheduleModel.ActivityInstance{
			plannedInstance("Schach", 1, 620, 700),
		}}, Room: fakeRoom{}}}
		out, next, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, false)
		require.NoError(t, err)
		require.Len(t, out, 1)
		assert.Equal(t, 621, next, "upcoming exit is startMin+1, not startMin")
	})

	t.Run("nil instance reader yields no scheduled change", func(t *testing.T) {
		svc := &service{}
		_, next, err := svc.activityReminders(context.Background(), adminScope, nil, timezone.TodayDate(), nowMin, lead, overdueThreshold, true, true)
		require.NoError(t, err)
		assert.Equal(t, -1, next)
	})
}

func TestComputeExposesNextChangeAt(t *testing.T) {
	t.Parallel()

	now := timezone.Now()
	nowMin := now.Hour()*60 + now.Minute()
	// Guard the wall-clock arithmetic against day wrap near midnight.
	if nowMin < 30 || nowMin > 1400 {
		t.Skip("skipping wall-clock-relative next-change test near midnight")
	}

	svc := &service{Dependencies: Dependencies{Settings: fakeSettings{bools: map[string]bool{
		configModel.KeyRemindersPickupUpcomingEnabled: true,
	}}, Attendance: fakeAttendance{ids: []int64{1}}, Pickup: fakePickup{times: map[int64]*scheduleService.EffectivePickupTime{
		1: pickupAt(nowMin + 20), // outside the default 10-min lead window
	}}, Student: fakeStudent{students: map[int64]*userModel.Student{1: {PersonID: 11, SchoolClass: "1a"}}}, Person: fakePerson{persons: map[int64]*userModel.Person{11: {FirstName: "Anna", LastName: "A"}}}},
	}

	res, err := svc.Compute(context.Background(), Scope{IsAdmin: true})
	require.NoError(t, err)
	assert.True(t, res.Enabled)
	assert.Empty(t, res.Reminders, "the pickup is still outside its window, so nothing is due yet")
	// It enters the window (lead 10) at (nowMin+20)-10 = nowMin+10.
	assert.Equal(t, formatMinutes(nowMin+10), res.NextChangeAt)
}

// --- pure helpers ------------------------------------------------------------

func TestPureHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "10:05", formatMinutes(605))
	assert.Equal(t, "00:00", formatMinutes(0))
	assert.Equal(t, "09:55", formatMinutes(595))
	assert.Equal(t, 605, minutesOfDay(wallClock(605)))
	assert.Equal(t, "7", *int64String(7))
}
