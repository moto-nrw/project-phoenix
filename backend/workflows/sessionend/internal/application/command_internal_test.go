package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The decision tests replace every owner with a recorder. They pin the order
// of the close, the single unit of work around it, and what reaches the
// announcement, without a database.

type recorder struct {
	steps []string
}

func (r *recorder) step(name string) { r.steps = append(r.steps, name) }

type fakePresence struct {
	rec        *recorder
	group      studentpresence.LiveGroup
	lockErr    error
	ended      studentpresence.EndedGroupSession
	endErr     error
	endedAt    time.Time
	endedGroup int64
}

func (f *fakePresence) LockGroup(context.Context, int64) (studentpresence.LiveGroup, error) {
	f.rec.step("lock")
	return f.group, f.lockErr
}

func (f *fakePresence) EndGroupSession(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
	f.rec.step("presence")
	f.endedGroup, f.endedAt = id, at
	return f.ended, f.endErr
}

type fakeTimetable struct {
	rec         *recorder
	instances   []timetable.ActivityInstance
	listErr     error
	checkoutErr error
	checkoutAt  time.Time
	groupName   string
	groupErr    error
}

func (f *fakeTimetable) ListActivityInstances(_ context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error) {
	f.rec.step("find-instance")
	if filter.ActiveGroupID == nil {
		return nil, errors.New("filter must name the active group")
	}
	return f.instances, f.listErr
}

func (f *fakeTimetable) CloseOpenCheckoutsByActiveGroupIDs(_ context.Context, _ []int64, at time.Time) (int, error) {
	f.rec.step("checkouts")
	f.checkoutAt = at
	return 1, f.checkoutErr
}

func (f *fakeTimetable) FindGroup(context.Context, int64) (timetable.Group, error) {
	f.rec.step("activity-name")
	return timetable.Group{Name: f.groupName}, f.groupErr
}

type fakeCompletion struct {
	rec     *recorder
	changed int64
	err     error
	at      time.Time
}

func (f *fakeCompletion) CompleteActiveByActiveGroupIDs(_ context.Context, _ []int64, at time.Time) (int64, error) {
	f.rec.step("complete")
	f.at = at
	return f.changed, f.err
}

type fakeStudents struct {
	rec  *recorder
	rows []peopledirectory.Student
	err  error
	ids  []int64
}

func (f *fakeStudents) ListStudentsByID(_ context.Context, ids []int64) ([]peopledirectory.Student, error) {
	f.rec.step("students")
	f.ids = ids
	return f.rows, f.err
}

type fakeRooms struct {
	rec  *recorder
	name string
	err  error
}

func (f *fakeRooms) FindRoom(context.Context, int64) (facilities.Room, error) {
	f.rec.step("room-name")
	return facilities.Room{Name: f.name}, f.err
}

type fakeNotifier struct {
	rec   *recorder
	notes []ports.Notification
}

func (f *fakeNotifier) SessionEnded(note ports.Notification) {
	f.rec.step("notify")
	f.notes = append(f.notes, note)
}

// fakeRuntime is a unit of work that runs fn once, records whether it
// committed, and only then drains the after-commit queue.
type fakeRuntime struct {
	rec       *recorder
	committed bool
	rolled    bool
}

func (f *fakeRuntime) runtime(now time.Time) ports.Runtime {
	var hooks []func()
	return ports.Runtime{
		TenantID: func(context.Context) int64 { return 42 },
		WithinTenant: func(ctx context.Context, fn func(context.Context) error) error {
			f.rec.step("begin")
			hooks = nil
			err := fn(ctx)
			if err != nil {
				f.rolled = true
				f.rec.step("rollback")
				return err
			}
			f.committed = true
			f.rec.step("commit")
			for _, hook := range hooks {
				hook()
			}
			return nil
		},
		AfterCommit: func(_ context.Context, fn func()) { hooks = append(hooks, fn) },
		Now:         func() time.Time { return now },
	}
}

type harness struct {
	rec        *recorder
	presence   *fakePresence
	timetable  *fakeTimetable
	completion *fakeCompletion
	students   *fakeStudents
	rooms      *fakeRooms
	notifier   *fakeNotifier
	runtime    *fakeRuntime
	observed   []ports.Observation
	now        time.Time
}

func newHarness() *harness {
	rec := &recorder{}
	activityID := int64(7)
	eduGroup := int64(3)
	h := &harness{
		rec: rec,
		now: time.Date(2026, 9, 8, 15, 30, 0, 0, time.UTC),
		presence: &fakePresence{
			rec:   rec,
			group: studentpresence.LiveGroup{ID: 66, RoomID: 5, ActivityGroupID: &activityID},
			ended: studentpresence.EndedGroupSession{
				GroupID: 66,
				ClosedVisits: []studentpresence.Visit{
					{ID: 1, StudentID: 100, ActiveGroupID: 66},
					{ID: 2, StudentID: 200, ActiveGroupID: 66},
					{ID: 3, StudentID: 100, ActiveGroupID: 66},
				},
				EndedSupervisorIDs: []int64{9},
			},
		},
		timetable: &fakeTimetable{
			rec:       rec,
			instances: []timetable.ActivityInstance{{ID: 77, Date: "2026-09-08", StartTime: "15:00:00", RoomID: 5}},
			groupName: "Fußball",
		},
		completion: &fakeCompletion{rec: rec, changed: 1},
		students: &fakeStudents{rec: rec, rows: []peopledirectory.Student{
			{ID: 100, GroupID: &eduGroup},
			{ID: 200},
		}},
		rooms:    &fakeRooms{rec: rec, name: "Turnhalle"},
		notifier: &fakeNotifier{rec: rec},
		runtime:  &fakeRuntime{rec: rec},
	}
	return h
}

func (h *harness) command() sessionend.Command {
	return NewCommand(Dependencies{
		Presence:   h.presence,
		Timetable:  h.timetable,
		Completion: h.completion,
		Students:   h.students,
		Rooms:      h.rooms,
		Notifier:   h.notifier,
		Runtime:    h.runtime.runtime(h.now),
		Observe:    func(o ports.Observation) { h.observed = append(h.observed, o) },
	})
}

func TestEndSessionClosesBothOwnersInOneUnitAndAnnouncesAfterCommit(t *testing.T) {
	t.Parallel()
	h := newHarness()

	result, err := h.command().EndSession(context.Background(), 66)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"begin", "lock", "presence", "find-instance", "checkouts", "complete",
		"students", "activity-name", "room-name", "commit", "notify",
	}, h.rec.steps, "presence closes first, the timetable follows in the same unit, the announcement waits for the commit")
	assert.Equal(t, int64(66), h.presence.endedGroup)
	assert.Equal(t, h.now, h.presence.endedAt)
	assert.Equal(t, h.now, h.timetable.checkoutAt, "slot check-outs carry the close instant")
	assert.Equal(t, h.now, h.completion.at, "the completion carries the close instant")
	assert.Equal(t, []int64{100, 200}, h.students.ids, "students are resolved once per child")

	assert.Equal(t, int64(66), result.ActiveGroupID)
	assert.Equal(t, h.now, result.EndedAt)
	assert.Equal(t, 3, result.StudentsCheckedOut)
	assert.Equal(t, 1, result.SupervisorsEnded)
	require.NotNil(t, result.MirroredInstanceID)
	assert.Equal(t, int64(77), *result.MirroredInstanceID)

	require.Len(t, h.notifier.notes, 1)
	note := h.notifier.notes[0]
	assert.Equal(t, int64(42), note.TenantID)
	assert.Equal(t, int64(66), note.ActiveGroupID)
	assert.Equal(t, int64(5), note.RoomID)
	assert.Equal(t, "Fußball", note.ActivityName)
	assert.Equal(t, "Turnhalle", note.RoomName)
	require.Len(t, note.Students, 2)
	assert.Equal(t, int64(100), note.Students[0].StudentID)
	require.NotNil(t, note.Students[0].EducationGroupID)
	assert.Equal(t, int64(3), *note.Students[0].EducationGroupID)
	assert.Equal(t, int64(200), note.Students[1].StudentID)
	assert.Nil(t, note.Students[1].EducationGroupID)
	require.NotNil(t, note.Instance)
	assert.Equal(t, ports.CompletedInstance{ID: 77, Date: "2026-09-08", StartTime: "15:00:00", RoomID: 5}, *note.Instance)

	require.Len(t, h.observed, 1)
	assert.Equal(t, "end_session", h.observed[0].Operation)
	assert.NoError(t, h.observed[0].Err)
	assert.Equal(t, 3, h.observed[0].StudentsCheckedOut)
	assert.True(t, h.observed[0].InstanceCompleted)
}

func TestEndSessionWithoutMirroredInstanceSkipsTheTimetableWrites(t *testing.T) {
	t.Parallel()
	h := newHarness()
	h.timetable.instances = nil
	h.presence.group.ActivityGroupID = nil
	h.presence.ended.ClosedVisits = nil

	result, err := h.command().EndSession(context.Background(), 66)
	require.NoError(t, err)

	assert.Equal(t, []string{"begin", "lock", "presence", "find-instance", "room-name", "commit", "notify"}, h.rec.steps,
		"no slot check-outs, no completion, no student lookup, no template lookup for a spontaneous session")
	assert.Nil(t, result.MirroredInstanceID)
	assert.Zero(t, result.StudentsCheckedOut)
	require.Len(t, h.notifier.notes, 1)
	assert.Nil(t, h.notifier.notes[0].Instance)
	assert.Empty(t, h.notifier.notes[0].Students)
	assert.Empty(t, h.notifier.notes[0].ActivityName)
}

func TestEndSessionDoesNotAnnounceAnInstanceAnotherPathCompleted(t *testing.T) {
	t.Parallel()
	h := newHarness()
	h.completion.changed = 0

	result, err := h.command().EndSession(context.Background(), 66)
	require.NoError(t, err)
	assert.Nil(t, result.MirroredInstanceID)
	require.Len(t, h.notifier.notes, 1)
	assert.Nil(t, h.notifier.notes[0].Instance, "re-announcing a completion that did not happen here would be a phantom event")
}

func TestEndSessionRejectsMissingAndEndedSessionsBeforeWriting(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		h := newHarness()
		h.presence.lockErr = studentpresence.ErrGroupNotFound
		_, err := h.command().EndSession(context.Background(), 66)
		require.ErrorIs(t, err, sessionend.ErrSessionNotFound)
		assert.Equal(t, []string{"begin", "lock", "rollback"}, h.rec.steps)
		assert.Empty(t, h.notifier.notes)
	})

	t.Run("already ended", func(t *testing.T) {
		t.Parallel()
		h := newHarness()
		ended := h.now.Add(-time.Hour)
		h.presence.group.EndTime = &ended
		_, err := h.command().EndSession(context.Background(), 66)
		require.ErrorIs(t, err, sessionend.ErrSessionAlreadyEnded)
		assert.Equal(t, []string{"begin", "lock", "rollback"}, h.rec.steps)
		assert.Empty(t, h.notifier.notes)
	})

	t.Run("ended by a concurrent close after the lock", func(t *testing.T) {
		t.Parallel()
		h := newHarness()
		h.presence.endErr = studentpresence.ErrGroupEnded
		_, err := h.command().EndSession(context.Background(), 66)
		require.ErrorIs(t, err, sessionend.ErrSessionAlreadyEnded)
		assert.Empty(t, h.notifier.notes)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		h := newHarness()
		_, err := h.command().EndSession(context.Background(), 0)
		require.Error(t, err)
		assert.Equal(t, []string{"begin", "rollback"}, h.rec.steps)
	})
}

// Every owner failure after the first write returns the error unchanged in
// meaning, leaves the unit rolled back, and announces nothing. A failing
// timetable side never leaves a closed session behind, and a failing
// presence side never leaves a completed block behind.
func TestEndSessionFailuresRollBackTheWholeUnitAndStaySilent(t *testing.T) {
	t.Parallel()
	boom := errors.New("owner down")
	cases := map[string]func(*harness){
		"presence end":     func(h *harness) { h.presence.endErr = boom },
		"instance lookup":  func(h *harness) { h.timetable.listErr = boom },
		"slot check-outs":  func(h *harness) { h.timetable.checkoutErr = boom },
		"completion":       func(h *harness) { h.completion.err = boom },
		"student lookup":   func(h *harness) { h.students.err = boom },
		"activity lookup":  func(h *harness) { h.timetable.groupErr = boom },
		"room name lookup": func(h *harness) { h.rooms.err = boom },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness()
			arrange(h)
			result, err := h.command().EndSession(context.Background(), 66)
			require.ErrorIs(t, err, boom, "the owner's error stays visible")
			assert.Equal(t, sessionend.Result{}, result)
			assert.True(t, h.runtime.rolled)
			assert.False(t, h.runtime.committed)
			assert.Empty(t, h.notifier.notes)
			require.Len(t, h.observed, 1)
			assert.ErrorIs(t, h.observed[0].Err, boom)
		})
	}
}

func TestEndSessionTreatsMissingTemplateAndRoomAsNamelessNotFailed(t *testing.T) {
	t.Parallel()
	h := newHarness()
	h.timetable.groupErr = timetable.ErrGroupNotFound
	h.rooms.err = facilities.ErrRoomNotFound

	_, err := h.command().EndSession(context.Background(), 66)
	require.NoError(t, err)
	require.Len(t, h.notifier.notes, 1)
	assert.Empty(t, h.notifier.notes[0].ActivityName)
	assert.Empty(t, h.notifier.notes[0].RoomName)
}

func TestNewCommandRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	h := newHarness()
	deps := Dependencies{
		Presence: h.presence, Timetable: h.timetable, Completion: h.completion, Students: h.students,
		Rooms: h.rooms, Notifier: h.notifier, Runtime: h.runtime.runtime(h.now), Observe: func(ports.Observation) {},
	}
	assert.NotPanics(t, func() { NewCommand(deps) })
	missing := deps
	missing.Completion = nil
	assert.Panics(t, func() { NewCommand(missing) })
	missing = deps
	missing.Runtime.AfterCommit = nil
	assert.Panics(t, func() { NewCommand(missing) })
}
