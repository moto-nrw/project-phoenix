package reminders

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// The world
//
// One fixture object serves BOTH the per-caller readers Compute uses and the
// bulk readers ComputeBatch uses, deriving every answer from the same fields.
// That is the point: hand-writing separate bulk fixtures would only test the
// fixture writing, not whether the two code paths agree.
// =============================================================================

type world struct {
	settingBools   map[string]bool
	settingInts    map[string]int
	settingStrings map[string]string

	students    map[int64]*userModel.Student
	persons     map[int64]*userModel.Person
	pickupTimes map[int64]*scheduleService.EffectivePickupTime

	attendanceIDs []int64
	visitsByRoom  map[int64][]int64
	roomsByStaff  map[int64][]int64

	instances     []*scheduleModel.ActivityInstance
	instanceStaff []*scheduleModel.InstanceStaff
	roomNames     map[int64]string

	mu    sync.Mutex
	calls map[string]int
}

func newWorld() *world {
	return &world{
		settingBools: map[string]bool{
			configModel.KeyRemindersPickupUpcomingEnabled:  true,
			configModel.KeyRemindersPickupOverdueEnabled:   true,
			configModel.KeyRemindersActivityStartEnabled:   true,
			configModel.KeyRemindersActivityOverdueEnabled: true,
		},
		settingInts: map[string]int{
			configModel.KeyRemindersPickupUpcomingLeadMinutes: 10,
			configModel.KeyRemindersActivityStartLeadMinutes:  10,
			configModel.KeyTimetableOverdueThresholdMinutes:   5,
		},
		settingStrings: map[string]string{
			configModel.KeyPresenceMode: configModel.PresenceModeDetailed,
		},
		students:     map[int64]*userModel.Student{},
		persons:      map[int64]*userModel.Person{},
		pickupTimes:  map[int64]*scheduleService.EffectivePickupTime{},
		visitsByRoom: map[int64][]int64{},
		roomsByStaff: map[int64][]int64{},
		roomNames:    map[int64]string{},
		calls:        map[string]int{},
	}
}

func (w *world) hit(op string) {
	w.mu.Lock()
	w.calls[op]++
	w.mu.Unlock()
}

func (w *world) resetCalls() {
	w.mu.Lock()
	w.calls = map[string]int{}
	w.mu.Unlock()
}

func (w *world) snapshot() map[string]int {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]int, len(w.calls))
	maps.Copy(out, w.calls)
	return out
}

// addStudent registers a child with a name, a class and an education group.
func (w *world) addStudent(id int64, group int64, name, class string) *world {
	st := &userModel.Student{SchoolClass: class}
	st.ID = id
	st.PersonID = id
	if group > 0 {
		g := group
		st.GroupID = &g
	}
	w.students[id] = st

	p := &userModel.Person{FirstName: name, LastName: fmt.Sprintf("Nr%d", id)}
	p.ID = id
	w.persons[id] = p
	return w
}

func (w *world) presentInRoom(roomID int64, studentIDs ...int64) *world {
	w.visitsByRoom[roomID] = append(w.visitsByRoom[roomID], studentIDs...)
	w.attendanceIDs = append(w.attendanceIDs, studentIDs...)
	return w
}

// presentOnlyInRoom checks a child into a room WITHOUT an attendance row. That
// combination is the reason a batch may never derive its hydration sets from
// the admin (attendance-based) set.
func (w *world) presentOnlyInRoom(roomID int64, studentIDs ...int64) *world {
	w.visitsByRoom[roomID] = append(w.visitsByRoom[roomID], studentIDs...)
	return w
}

func (w *world) pickupAtMinute(studentID int64, minuteOfDay int) *world {
	w.pickupTimes[studentID] = pickupAt(minuteOfDay)
	return w
}

func (w *world) supervises(staffID int64, roomIDs ...int64) *world {
	w.roomsByStaff[staffID] = append(w.roomsByStaff[staffID], roomIDs...)
	return w
}

func (w *world) addInstance(id int64, title string, roomID int64, startMin, endMin int) *world {
	inst := plannedInstance(title, roomID, startMin, endMin)
	inst.ID = id
	w.instances = append(w.instances, inst)
	if _, ok := w.roomNames[roomID]; !ok {
		w.roomNames[roomID] = "Lernraum"
	}
	return w
}

// assignsInstance plans a staff member onto an activity instance, the relation
// the Betreuungsplan records.
func (w *world) assignsInstance(staffID, instanceID int64, absent bool) *world {
	w.instanceStaff = append(w.instanceStaff, &scheduleModel.InstanceStaff{
		InstanceID: instanceID,
		StaffID:    staffID,
		IsAbsent:   absent,
	})
	return w
}

func (w *world) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStaff, error) {
	w.hit("FindByInstanceIDs")
	wanted := make(map[int64]struct{}, len(instanceIDs))
	for _, id := range instanceIDs {
		wanted[id] = struct{}{}
	}
	out := make([]*scheduleModel.InstanceStaff, 0, len(w.instanceStaff))
	for _, row := range w.instanceStaff {
		if _, ok := wanted[row.InstanceID]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

func (w *world) service() *service {
	return &service{Dependencies: Dependencies{
		Settings:          w,
		Attendance:        w,
		Pickup:            w,
		Instance:          w,
		Room:              worldRooms{w},
		Student:           w,
		Person:            worldPersons{w},
		Supervision:       w,
		BulkSupervision:   w,
		BulkVisits:        w,
		BulkInstanceStaff: w,
	}}
}

// --- settingsResolver ---------------------------------------------------------

func (w *world) ResolveBool(_ context.Context, key string) (bool, error) {
	w.hit("ResolveBool")
	return w.settingBools[key], nil
}

func (w *world) ResolveInt(_ context.Context, key string) (int, error) {
	w.hit("ResolveInt")
	return w.settingInts[key], nil
}

func (w *world) ResolveString(_ context.Context, key string) (string, error) {
	w.hit("ResolveString")
	return w.settingStrings[key], nil
}

// --- attendanceReader ---------------------------------------------------------

func (w *world) ListOpenStudentIDsForDate(_ context.Context, _ timezone.Date) ([]int64, error) {
	w.hit("ListOpenStudentIDsForDate")
	return append([]int64(nil), w.attendanceIDs...), nil
}

// --- pickupReader -------------------------------------------------------------

// Narrows to the requested IDs, unlike the older fake in service_internal_test.go.
// The batch asks over a union, so a fake that ignores its argument would hide a
// missing student.
func (w *world) GetBulkEffectivePickupTimesForDate(_ context.Context, ids []int64, _ timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	w.hit("GetBulkEffectivePickupTimesForDate")
	out := make(map[int64]*scheduleService.EffectivePickupTime, len(ids))
	for _, id := range ids {
		if t, ok := w.pickupTimes[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

// --- instanceReader -----------------------------------------------------------

func (w *world) FindByTenantAndDate(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	w.hit("FindByTenantAndDate")
	return w.instances, nil
}

// --- studentReader ------------------------------------------------------------

func (w *world) FindReadScopeByIDs(_ context.Context, ids []int64) (map[int64]*userModel.Student, error) {
	w.hit("FindReadScopeByIDs")
	out := make(map[int64]*userModel.Student, len(ids))
	for _, id := range ids {
		if st, ok := w.students[id]; ok {
			out[id] = st
		}
	}
	return out, nil
}

// --- supervisionReader --------------------------------------------------------
//
// The test world identifies an active group with the room it runs in, so a
// supervision row carries the room ID as its GroupID.

func (w *world) GetStaffActiveSupervisions(_ context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
	w.hit("GetStaffActiveSupervisions")
	out := make([]*activeModel.GroupSupervisor, 0, len(w.roomsByStaff[staffID]))
	for _, roomID := range w.roomsByStaff[staffID] {
		out = append(out, &activeModel.GroupSupervisor{StaffID: staffID, GroupID: roomID})
	}
	return out, nil
}

func (w *world) GetActiveGroupsByIDs(_ context.Context, groupIDs []int64) (map[int64]*activeModel.Group, error) {
	w.hit("GetActiveGroupsByIDs")
	out := make(map[int64]*activeModel.Group, len(groupIDs))
	for _, id := range groupIDs {
		out[id] = &activeModel.Group{RoomID: id}
	}
	return out, nil
}

func (w *world) ListStudentsPresentInRoom(_ context.Context, roomID int64) ([]int64, error) {
	w.hit("ListStudentsPresentInRoom")
	return w.visitsByRoom[roomID], nil
}

// --- bulk readers -------------------------------------------------------------

func (w *world) ListActiveSupervisedRooms(_ context.Context) ([]activeModel.StaffRoomSupervision, error) {
	w.hit("ListActiveSupervisedRooms")
	var out []activeModel.StaffRoomSupervision
	for staffID, rooms := range w.roomsByStaff {
		for _, roomID := range rooms {
			out = append(out, activeModel.StaffRoomSupervision{StaffID: staffID, RoomID: roomID})
		}
	}
	return out, nil
}

func (w *world) ListOpenVisitStudentIDsByRoom(_ context.Context) (map[int64][]int64, error) {
	w.hit("ListOpenVisitStudentIDsByRoom")
	return w.visitsByRoom, nil
}

// --- adapters for the two colliding FindByIDs signatures ----------------------

type worldRooms struct{ w *world }

func (r worldRooms) FindByIDs(_ context.Context, ids []int64) ([]*facilitiesModel.Room, error) {
	r.w.hit("Room.FindByIDs")
	out := make([]*facilitiesModel.Room, 0, len(ids))
	for _, id := range ids {
		name, ok := r.w.roomNames[id]
		if !ok {
			name = "Lernraum"
		}
		room := &facilitiesModel.Room{Name: name}
		room.ID = id
		out = append(out, room)
	}
	return out, nil
}

type worldPersons struct{ w *world }

func (p worldPersons) FindByIDs(_ context.Context, ids []int64) (map[int64]*userModel.Person, error) {
	p.w.hit("Person.FindByIDs")
	out := make(map[int64]*userModel.Person, len(ids))
	for _, id := range ids {
		if person, ok := p.w.persons[id]; ok {
			out[id] = person
		}
	}
	return out, nil
}

// =============================================================================
// Equivalence
// =============================================================================

// assertBatchMatchesCompute is the core assertion of this package: for every
// scope, ComputeBatch must produce exactly what Compute produces.
//
// Both entry points read the clock themselves, so a minute tick between the two
// calls would shift every MinutesAway by one. The pair is therefore retried once
// if the wall-clock minute moved.
func assertBatchMatchesCompute(t *testing.T, w *world, scopes []BatchScope) {
	t.Helper()

	svc := w.service()
	ctx := context.Background()

	for attempt := 0; attempt < 2; attempt++ {
		before := minutesOfDay(timezone.Now())

		batch, err := svc.ComputeBatch(ctx, scopes)
		require.NoError(t, err)

		singles := make(map[int64]*Result, len(scopes))
		for _, sc := range scopes {
			single, serr := svc.Compute(ctx, sc.Scope)
			require.NoError(t, serr)
			singles[sc.StaffID] = single
		}

		if minutesOfDay(timezone.Now()) != before {
			if attempt == 0 {
				continue // the minute rolled over mid-comparison, redo the pair
			}
			t.Skip("wall-clock minute kept changing during the comparison")
		}

		require.Len(t, batch, len(scopes), "every requested scope needs an entry")
		for _, sc := range scopes {
			single := singles[sc.StaffID]
			got := batch[sc.StaffID]
			require.NotNil(t, got, "staff %d must have a result", sc.StaffID)

			assert.Equal(t, single.Enabled, got.Enabled, "staff %d: Enabled", sc.StaffID)
			assert.Equal(t, single.Count, got.Count, "staff %d: Count", sc.StaffID)
			assert.Equal(t, single.NextChangeAt, got.NextChangeAt, "staff %d: NextChangeAt", sc.StaffID)
			assert.Equal(t, single.Reminders, got.Reminders, "staff %d: Reminders", sc.StaffID)
		}
		return
	}
}

func caregiver(staffID int64) BatchScope {
	return BatchScope{Scope: Scope{IsAdmin: false, StaffID: staffID}}
}

func personalCaregiver(staffID int64) BatchScope {
	return BatchScope{
		Scope:                        Scope{IsAdmin: false, StaffID: staffID},
		IncludeAssignedActivityStart: true,
	}
}

func admin(staffID int64) BatchScope {
	return BatchScope{Scope: Scope{IsAdmin: true, StaffID: staffID}}
}

func TestComputeBatchMatchesCompute(t *testing.T) {
	nowMin := minutesOfDay(timezone.Now())
	if nowMin < 60 || nowMin > 1380 {
		t.Skip("skipping near midnight: fixtures would wrap the day boundary")
	}

	soon := nowMin + 5  // inside the 10 minute lead window
	late := nowMin - 15 // overdue
	far := nowMin + 120 // outside every window, contributes only a boundary
	start := nowMin + 5 // activity starting soon
	overdue := nowMin - 20

	// baseWorld: two rooms, two education groups, one child each, plus a child
	// in a room nobody supervises.
	baseWorld := func() *world {
		w := newWorld()
		w.addStudent(1, 100, "Anna", "1a").addStudent(2, 200, "Ben", "2b").addStudent(3, 300, "Cem", "3c")
		w.pickupAtMinute(1, soon).pickupAtMinute(2, late).pickupAtMinute(3, soon)
		w.presentInRoom(10, 1).presentInRoom(20, 2).presentInRoom(30, 3)
		w.supervises(7, 10)
		w.supervises(8, 20)
		w.addInstance(101, "Schach", 10, start, start+60)
		w.addInstance(102, "Fußball", 20, overdue, overdue+90)
		return w
	}

	t.Run("admin sees everything", func(t *testing.T) {
		assertBatchMatchesCompute(t, baseWorld(), []BatchScope{admin(1)})
	})

	t.Run("caregiver with one room", func(t *testing.T) {
		assertBatchMatchesCompute(t, baseWorld(), []BatchScope{caregiver(7)})
	})

	t.Run("caregiver without supervision sees nothing", func(t *testing.T) {
		w := baseWorld()
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(99)})

		// Asserted in absolute terms as well, not only against Compute: an
		// equivalence check compares the two paths with each other and is blind
		// to a bug they share. "Supervises nothing" turning into "sees
		// everything" is the worst such bug, so it gets its own claim —
		// especially now that the per-child read gate is gone (#2329) and
		// supervision alone decides which children reach a caregiver.
		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{caregiver(99)})
		require.NoError(t, err)
		assert.Empty(t, batch[99].Reminders,
			"a caregiver without a live supervision must see no activity and no child")
	})

	t.Run("caregiver sees every child in a supervised room, whatever group", func(t *testing.T) {
		// Staff 9 supervises room 10 without any relation to the child's
		// education group — since #2329 that is enough to read the child.
		w := baseWorld()
		w.supervises(9, 10)
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(9)})

		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{caregiver(9)})
		require.NoError(t, err)
		assert.NotEmpty(t, batch[9].Reminders,
			"a room supervisor reads the children standing in their room")
	})

	t.Run("binary mode reads presence from attendance", func(t *testing.T) {
		w := baseWorld()
		w.settingStrings[configModel.KeyPresenceMode] = configModel.PresenceModeBinary
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7)})
	})

	t.Run("two caregivers must not contaminate each other", func(t *testing.T) {
		assertBatchMatchesCompute(t, baseWorld(), []BatchScope{caregiver(7), caregiver(8)})
	})

	t.Run("admin and caregiver in one batch", func(t *testing.T) {
		assertBatchMatchesCompute(t, baseWorld(), []BatchScope{admin(1), caregiver(7)})
	})

	t.Run("two rooms sharing a child are deduplicated", func(t *testing.T) {
		w := baseWorld()
		w.supervises(7, 20)
		w.presentInRoom(20, 1) // the same child is present in both rooms
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7)})
	})

	t.Run("child present in a room but missing from attendance", func(t *testing.T) {
		// The case that breaks any design deriving hydration sets from the
		// admin (attendance-based) set: the caregiver must still see the child
		// standing in their own room.
		w := baseWorld()
		w.addStudent(4, 100, "Dana", "1a")
		w.pickupAtMinute(4, soon)
		w.presentOnlyInRoom(10, 4)
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7), admin(1)})

		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{caregiver(7)})
		require.NoError(t, err)
		var found bool
		for _, r := range batch[7].Reminders {
			if r.StudentID != nil && *r.StudentID == "4" {
				found = true
			}
		}
		assert.True(t, found, "the visit-only child must produce a reminder for its room supervisor")
	})

	t.Run("due child without a person record is dropped identically", func(t *testing.T) {
		w := baseWorld()
		delete(w.persons, 1)
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7), admin(1)})
	})

	t.Run("child outside every window contributes only a boundary", func(t *testing.T) {
		w := baseWorld()
		w.pickupAtMinute(1, far)
		assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7)})
	})

	t.Run("planned upcoming assignment widens the batch and never narrows it", func(t *testing.T) {
		// The batch deliberately diverges from Compute here: it also counts the
		// slots a person is planned on, because ten minutes before the start
		// they supervise nothing yet and the room filter alone would reach
		// everyone except the one person who has to show up.
		w := baseWorld()
		// Staff 7 supervises room 10; instance 103 runs in room 20, which they
		// do not supervise, but they are planned on it.
		w.addInstance(103, "Theater", 20, start, start+60)
		w.assignsInstance(7, 103, false)

		single, err := w.service().Compute(context.Background(), Scope{StaffID: 7})
		require.NoError(t, err)
		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{personalCaregiver(7)})
		require.NoError(t, err)

		hasInstance := func(rs []Reminder, id string) bool {
			for _, r := range rs {
				if r.ActivityInstanceID != nil && *r.ActivityInstanceID == id {
					return true
				}
			}
			return false
		}

		assert.False(t, hasInstance(single.Reminders, "103"),
			"Compute stays room-scoped and must not change")
		assert.True(t, hasInstance(batch[7].Reminders, "103"),
			"the batch must reach the person planned on the slot")
		assert.GreaterOrEqual(t, len(batch[7].Reminders), len(single.Reminders),
			"the batch may show more, never fewer")
	})

	t.Run("personal assignment is independent of the room activity gate", func(t *testing.T) {
		w := baseWorld()
		w.settingBools[configModel.KeyRemindersPickupUpcomingEnabled] = false
		w.settingBools[configModel.KeyRemindersPickupOverdueEnabled] = false
		w.settingBools[configModel.KeyRemindersActivityStartEnabled] = false
		w.settingBools[configModel.KeyRemindersActivityOverdueEnabled] = false
		w.addInstance(103, "Unassigned", 10, start, start+60)
		w.assignsInstance(7, 101, false)

		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{personalCaregiver(7)})
		require.NoError(t, err)
		require.Len(t, batch[7].Reminders, 1)
		require.NotNil(t, batch[7].Reminders[0].ActivityInstanceID)
		assert.Equal(t, "101", *batch[7].Reminders[0].ActivityInstanceID)
		assert.Equal(t, TypeActivityStart, batch[7].Reminders[0].Type)
	})

	t.Run("an absent assignment is ignored", func(t *testing.T) {
		// Someone who called in sick must not be pinged about the slot they
		// were relieved of.
		w := baseWorld()
		w.assignsInstance(7, 102, true)

		batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{caregiver(7)})
		require.NoError(t, err)

		for _, r := range batch[7].Reminders {
			if r.ActivityInstanceID != nil {
				assert.NotEqual(t, "102", *r.ActivityInstanceID)
			}
		}
	})

	t.Run("without assignments both paths stay identical", func(t *testing.T) {
		assertBatchMatchesCompute(t, baseWorld(), []BatchScope{caregiver(7), caregiver(8), admin(1)})
	})

	t.Run("toggle combinations", func(t *testing.T) {
		combos := []struct {
			name                                string
			pickUp, pickOver, actStart, actOver bool
		}{
			{"pickups only", true, true, false, false},
			{"activities only", false, false, true, true},
			{"upcoming pickup only", true, false, false, false},
			{"overdue activity only", false, false, false, true},
			{"all off", false, false, false, false},
		}
		for _, c := range combos {
			t.Run(c.name, func(t *testing.T) {
				w := baseWorld()
				w.settingBools[configModel.KeyRemindersPickupUpcomingEnabled] = c.pickUp
				w.settingBools[configModel.KeyRemindersPickupOverdueEnabled] = c.pickOver
				w.settingBools[configModel.KeyRemindersActivityStartEnabled] = c.actStart
				w.settingBools[configModel.KeyRemindersActivityOverdueEnabled] = c.actOver
				assertBatchMatchesCompute(t, w, []BatchScope{caregiver(7), admin(1)})
			})
		}
	})
}

// TestActivityRoomFilterNilOnlyForAdmins pins the nil-versus-empty rule
// directly. Both entry points share this helper, so the equivalence test cannot
// see a mistake in it — they would simply be wrong together.
func TestActivityRoomFilterNilOnlyForAdmins(t *testing.T) {
	t.Run("admin gets no restriction", func(t *testing.T) {
		assert.Nil(t, activityRoomFilter(Scope{IsAdmin: true}, nil))
		assert.Nil(t, activityRoomFilter(Scope{IsAdmin: true}, []int64{10}))
	})

	t.Run("caregiver without rooms is restricted to nothing", func(t *testing.T) {
		filter := activityRoomFilter(Scope{StaffID: 7}, nil)
		require.NotNil(t, filter, "an empty room list must restrict, never widen")
		assert.Empty(t, filter)
	})

	t.Run("caregiver is restricted to their rooms", func(t *testing.T) {
		filter := activityRoomFilter(Scope{StaffID: 7}, []int64{10, 20})
		require.NotNil(t, filter)
		assert.Len(t, filter, 2)
		assert.Contains(t, filter, int64(10))
		assert.Contains(t, filter, int64(20))
	})
}

func TestComputeBatchScopeHandling(t *testing.T) {
	w := newWorld()
	svc := w.service()
	ctx := context.Background()

	t.Run("empty input costs nothing", func(t *testing.T) {
		w.resetCalls()
		out, err := svc.ComputeBatch(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Empty(t, w.snapshot(), "an empty batch must not touch a single reader")
	})

	t.Run("non-positive staff IDs are dropped", func(t *testing.T) {
		out, err := svc.ComputeBatch(ctx, []BatchScope{caregiver(0), caregiver(-1)})
		require.NoError(t, err)
		assert.Empty(t, out, "a scope that addresses nobody must not be computed")
	})

	t.Run("staff-less admins use an explicit result key", func(t *testing.T) {
		out, err := svc.ComputeBatch(ctx, []BatchScope{
			{Scope: Scope{IsAdmin: true}, ResultKey: -41},
		})
		require.NoError(t, err)
		require.Contains(t, out, int64(-41))
		assert.NotNil(t, out[-41])
	})

	t.Run("repeated staff IDs collapse", func(t *testing.T) {
		out, err := svc.ComputeBatch(ctx, []BatchScope{caregiver(7), caregiver(7)})
		require.NoError(t, err)
		assert.Len(t, out, 1)
	})

	t.Run("repeated scopes preserve personal assignment requests", func(t *testing.T) {
		scopes := dedupeBatchScopes([]BatchScope{caregiver(7), personalCaregiver(7)})
		require.Len(t, scopes, 1)
		assert.True(t, scopes[0].IncludeAssignedActivityStart)
	})

	t.Run("every recipient gets its own result", func(t *testing.T) {
		out, err := svc.ComputeBatch(ctx, []BatchScope{caregiver(7), caregiver(8)})
		require.NoError(t, err)
		require.Len(t, out, 2)
		assert.NotSame(t, out[7], out[8], "results must not share a pointer")
	})

	t.Run("nil dependencies yield empty disabled results, not a panic", func(t *testing.T) {
		bare := &service{}
		out, err := bare.ComputeBatch(ctx, []BatchScope{caregiver(7), admin(1)})
		require.NoError(t, err)
		require.Len(t, out, 2)
		for id, res := range out {
			assert.False(t, res.Enabled, "staff %d", id)
			assert.Empty(t, res.Reminders, "staff %d", id)
		}
	})
}

// =============================================================================
// Query count
// =============================================================================

func TestComputeBatchQueryCountIsFlatInStaffCount(t *testing.T) {
	nowMin := minutesOfDay(timezone.Now())
	if nowMin < 60 || nowMin > 1380 {
		t.Skip("skipping near midnight")
	}

	// A world with many staff, each supervising their own room, with a child
	// due for pickup in it.
	build := func(staffCount int) (*world, []BatchScope) {
		w := newWorld()

		scopes := make([]BatchScope, 0, staffCount)
		for n := range staffCount {
			i := n + 1
			staffID := int64(i)
			roomID := int64(100 + i)
			groupID := int64(200 + i)
			studentID := int64(1000 + i)

			w.addStudent(studentID, groupID, fmt.Sprintf("Kind%d", i), "1a")
			w.pickupAtMinute(studentID, nowMin+5)
			w.presentInRoom(roomID, studentID)
			w.supervises(staffID, roomID)
			w.addInstance(int64(500+i), fmt.Sprintf("AG%d", i), roomID, nowMin+5, nowMin+65)

			scopes = append(scopes, caregiver(staffID))
		}
		return w, scopes
	}

	run := func(staffCount int) (map[string]int, map[int64]*Result) {
		w, scopes := build(staffCount)
		w.resetCalls()
		out, err := w.service().ComputeBatch(context.Background(), scopes)
		require.NoError(t, err)
		return w.snapshot(), out
	}

	oneCalls, oneOut := run(1)
	manyCalls, manyOut := run(50)

	require.Len(t, oneOut, 1)
	require.Len(t, manyOut, 50)

	// A degenerate early return would also produce a flat count, so prove the
	// batch actually computed something first.
	require.NotEmpty(t, manyOut[1].Reminders, "the batch must produce real reminders")

	assert.Equal(t, oneCalls, manyCalls,
		"the number of reads must not grow with the number of staff")

	assert.Equal(t, 1, manyCalls["ListActiveSupervisedRooms"])
	assert.Equal(t, 1, manyCalls["ListOpenVisitStudentIDsByRoom"])
	assert.Equal(t, 1, manyCalls["FindReadScopeByIDs"])
	assert.Equal(t, 1, manyCalls["GetBulkEffectivePickupTimesForDate"])
	assert.Equal(t, 1, manyCalls["Person.FindByIDs"])

	assert.Zero(t, manyCalls["ListStudentsPresentInRoom"],
		"no per-room query loop")
	assert.Zero(t, manyCalls["GetStaffActiveSupervisions"],
		"no per-staff supervision lookup")
}

// TestBatchReportsPersonalAssignments pins the one piece of per-person context
// the batch reports beyond the reminder list itself.
//
// A consumer that addresses people individually has to tell "an activity in the
// room you are watching" from "the slot you are planned on" — the assignment is
// exactly what the batch already knows and Compute does not. Without it the
// distinction would have to be re-derived from supervision data, which is how
// two answers to one question start drifting apart.
func TestBatchReportsPersonalAssignments(t *testing.T) {
	nowMin := minutesOfDay(timezone.Now())
	if nowMin < 60 || nowMin > 1380 {
		t.Skip("skipping near midnight: fixtures would wrap the day boundary")
	}
	start := nowMin + 5

	w := newWorld()
	w.addInstance(201, "Schach", 10, start, start+60)
	w.addInstance(202, "Fußball", 20, start, start+60)
	w.supervises(7, 10) // watches room 10 only
	w.supervises(8, 10) // same room, no assignment
	w.assignsInstance(7, 202, false)
	w.assignsInstance(1, 201, false)

	batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{personalCaregiver(7), caregiver(8), admin(1)})
	require.NoError(t, err)

	assert.Equal(t, map[string]struct{}{"202": {}}, batch[7].AssignedActivityInstanceIDs,
		"only the slot this person is planned on, not the room they supervise")
	assert.Nil(t, batch[8].AssignedActivityInstanceIDs,
		"no assignment means nil, which reads as 'nothing of mine' rather than an empty restriction")
	assert.Equal(t, map[string]struct{}{"201": {}}, batch[1].AssignedActivityInstanceIDs,
		"an admin's personal assignment must remain distinguishable from room-wide activities")

	// The field describes the list, so it has to be usable as a key into it.
	var matched bool
	for _, r := range batch[7].Reminders {
		if r.ActivityInstanceID == nil {
			continue
		}
		if _, ok := batch[7].AssignedActivityInstanceIDs[*r.ActivityInstanceID]; ok {
			matched = true
		}
	}
	assert.True(t, matched, "the assigned instance must appear in the reminder list too")
}

func TestAssignedActivitiesExpandOnlyUpcomingScope(t *testing.T) {
	nowMin := minutesOfDay(timezone.Now())
	if nowMin < 60 || nowMin > 1380 {
		t.Skip("skipping near midnight: fixtures would wrap the day boundary")
	}

	w := newWorld()
	w.supervises(7, 10)
	w.addInstance(201, "Überfällig außerhalb der Aufsicht", 20, nowMin-20, nowMin+40)
	w.addInstance(202, "Nächster Einsatz außerhalb der Aufsicht", 20, nowMin+5, nowMin+65)
	w.assignsInstance(7, 201, false)
	w.assignsInstance(7, 202, false)

	batch, err := w.service().ComputeBatch(context.Background(), []BatchScope{personalCaregiver(7)})
	require.NoError(t, err)
	require.Contains(t, batch, int64(7))

	var reminderIDs []string
	for _, reminder := range batch[7].Reminders {
		if reminder.ActivityInstanceID != nil {
			reminderIDs = append(reminderIDs, *reminder.ActivityInstanceID)
		}
	}
	assert.Contains(t, reminderIDs, "202", "an upcoming personal assignment must be included")
	assert.NotContains(t, reminderIDs, "201", "an assignment outside supervised rooms must not widen overdue scope")
}
