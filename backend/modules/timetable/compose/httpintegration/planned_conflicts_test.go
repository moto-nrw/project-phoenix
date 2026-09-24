// Hermetic integration tests for the Timetable owner's DetectPlannedConflicts (the planning-time
// conflict probe behind GET /api/timetable/conflicts).
//
// Matrix (#2139 — only person double-bookings warn):
//   - room overlap alone → NO warning; pure adjacency (end == start) → none
//   - staff assigned to overlapping instance → warning only when the rooms
//     differ or the probe's room is undetermined; IsAbsent rows excluded
//   - student expected on overlapping instance → warning, regardless of rooms
//   - exclude_instance_id removes the instance being edited from the probe
//   - cancelled instances are ignored
//   - cross-tenant instances are invisible
//
// All fixtures via testpkg.CreateTest* + t.Cleanup — no hardcoded entity IDs.
package httpintegration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type plannedConflictSetup struct {
	detection timetable.ConflictDetectionCapability
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	staffID   int64
	studentID int64
	date      calendar.Date
}

func buildPlannedConflictSetup(t *testing.T) *plannedConflictSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("PC-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "PC", fmt.Sprintf("Staff-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "PC-Kid", fmt.Sprintf("One-%d", suffix), "2b")

	return &plannedConflictSetup{
		detection: newConflictDetection(t, repos),
		db:        db,
		ctx:       testpkg.Ctx(t),
		roomID:    room.ID,
		staffID:   staff.ID,
		studentID: student.ID,
		date:      calendar.NewDate(2026, time.June, 22), // fixed Monday, period-independent
	}
}

// probeQuery builds the default probe slot 14:30–15:30, overlapping the
// fixture instances' default 14:00–15:00 window.
func probeQuery(s *plannedConflictSetup, mutate func(*timetable.PlannedConflictProbe)) timetable.PlannedConflictProbe {
	q := timetable.PlannedConflictProbe{
		Date:      s.date,
		StartTime: time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC),
		EndTime:   time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC),
	}
	if mutate != nil {
		mutate(&q)
	}
	return q
}

func seedConflictInstance(t *testing.T, s *plannedConflictSetup, opts testpkg.ActivityInstanceOpts) *scheduleModels.ActivityInstance {
	t.Helper()
	inst := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, opts)
	return inst
}

func TestDetectPlannedConflicts_RoomOverlapAloneIsNotAConflict(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Belegt"})

	// #2139: several groups may share a room — a probe that only shares the
	// room with an overlapping instance yields no warning.
	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &s.roomID
	}))

	assert.Empty(t, warnings, "a pure room overlap must not warn")
}

func TestDetectPlannedConflicts_AdjacencyIsNotAConflict(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Davor"}) // 14:00–15:00
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})

	// Probe 15:00–16:00 with the same staff — touching edges must not conflict.
	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.StartTime = time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
		q.EndTime = time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC)
		q.StaffIDs = []int64{s.staffID}
	}))

	assert.Empty(t, warnings)
}

func TestDetectPlannedConflicts_Staff_IncludingAbsentExclusion(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Personal"})

	suffix := time.Now().UnixNano()
	absentStaff := testpkg.CreateTestStaff(t, s.db, "PC", fmt.Sprintf("Absent-%d", suffix))
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, absentStaff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	// Probe without a room: the slot's room is undetermined, so the staff
	// overlap warns ("not certainly the same room").
	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.StaffIDs = []int64{s.staffID, absentStaff.ID}
	}))

	require.Len(t, warnings, 1, "is_absent staff must not be flagged")
	assert.Equal(t, timetable.ConflictKindStaff, warnings[0].Kind)
	assert.Equal(t, s.staffID, warnings[0].ResourceID)
	assert.Equal(t, inst.ID, warnings[0].ConflictingInstanceID)
	assert.Contains(t, warnings[0].Message, "PC-Personal")
}

func TestDetectPlannedConflicts_StaffSameRoomIsNotAConflict(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Parallel"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})

	// Same staff, overlapping window, SAME concrete room — sanctioned
	// parallel supervision (#2139), no warning.
	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &s.roomID
		q.StaffIDs = []int64{s.staffID}
	}))

	assert.Empty(t, warnings, "same-room staff overlap must not warn")
}

func TestDetectPlannedConflicts_StaffDifferentRoomWarns(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Anderswo"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("PC-Room-B-%d", time.Now().UnixNano()))

	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &otherRoom.ID
		q.StaffIDs = []int64{s.staffID}
	}))

	require.Len(t, warnings, 1)
	assert.Equal(t, timetable.ConflictKindStaff, warnings[0].Kind)
	assert.Equal(t, s.staffID, warnings[0].ResourceID)
	assert.Contains(t, warnings[0].Message, "anderer Raum")
}

func TestDetectPlannedConflicts_StaffRowRoomOverrideCountsAsSameRoom(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	// Instance sits in room A, but THIS staff member's row is overridden to
	// room B (multi-room split). Probing room B with the same staff must not
	// warn — the effective rooms match.
	otherRoom := testpkg.CreateTestRoom(t, s.db, fmt.Sprintf("PC-Room-C-%d", time.Now().UnixNano()))
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Split"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{RoomID: &otherRoom.ID})

	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &otherRoom.ID
		q.StaffIDs = []int64{s.staffID}
	}))

	assert.Empty(t, warnings, "per-row room override must count as the effective room")
}

func TestDetectPlannedConflicts_Student(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Kind"})

	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, s.studentID, "")

	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.StudentIDs = []int64{s.studentID}
	}))

	require.Len(t, warnings, 1)
	assert.Equal(t, timetable.ConflictKindStudent, warnings[0].Kind)
	assert.Equal(t, s.studentID, warnings[0].ResourceID)
	assert.Equal(t, inst.ID, warnings[0].ConflictingInstanceID)
	assert.Contains(t, warnings[0].Message, "PC-Kind")
}

func TestDetectPlannedConflicts_StudentWarnsEvenInSameRoom(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Kind-Raum"})

	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, s.studentID, "")

	// A child double-booking warns regardless of rooms (#2139) — even when
	// the probe targets the SAME room as the conflicting instance.
	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &s.roomID
		q.StudentIDs = []int64{s.studentID}
	}))

	require.Len(t, warnings, 1, "same room must not excuse a child double-booking")
	assert.Equal(t, timetable.ConflictKindStudent, warnings[0].Kind)
}

func TestDetectPlannedConflicts_ExcludeSelf(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Selbst"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})

	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.StaffIDs = []int64{s.staffID}
		q.ExcludeInstanceID = &inst.ID
	}))

	assert.Empty(t, warnings, "the instance being edited must not conflict with itself")
}

func TestDetectPlannedConflicts_CancelledIgnored(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{
		Title:  "PC-Abgesagt",
		Status: scheduleModels.InstanceStatusCancelled,
	})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})

	warnings := s.detection.DetectPlannedConflicts(s.ctx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.StaffIDs = []int64{s.staffID}
	}))

	assert.Empty(t, warnings)
}

func TestDetectPlannedConflicts_CrossTenantInvisible(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Fremd"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})

	otherCtx := testpkg.TenantContext(999)
	warnings := s.detection.DetectPlannedConflicts(otherCtx, probeQuery(s, func(q *timetable.PlannedConflictProbe) {
		q.RoomID = &s.roomID
		q.StaffIDs = []int64{s.staffID}
	}))

	assert.Empty(t, warnings, "tenant 999 must not see tenant 1 instances")
}

// errPresenceNotUsed marks a Student Presence read the planning suites never
// make: only starting a block consults the live layer.
var errPresenceNotUsed = errors.New("student presence is not used by the planning reads")

// planningOnlyPresence stands in for Student Presence in the suites that
// probe the plan but never start a block.
type planningOnlyPresence struct{}

func (planningOnlyPresence) ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	return nil, errPresenceNotUsed
}

func (planningOnlyPresence) QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error) {
	return nil, errPresenceNotUsed
}

// newConflictDetection composes the Timetable owner's conflict detection and
// staffing capability over the retained repositories, the way the root does.
func newConflictDetection(t *testing.T, repos repositories.TimetableTestRepositories) timetable.ConflictDetectionCapability {
	t.Helper()
	detection, err := services.NewTimetableConflictDetection(services.TimetableConflictReaders{
		Instances:         repos.ActivityInstance,
		InstanceStaff:     repos.InstanceStaff,
		InstanceStudents:  repos.InstanceStudent,
		Exceptions:        repos.ActivityException,
		Schedules:         repos.ActivitySchedule,
		Staff:             repos.Staff,
		CalendarPeriods:   repos.CalendarPeriod,
		ArrivalExceptions: repos.StudentArrivalException,
		Sessions:          repos.ActiveGroup,
		Shifts:            repos.StaffShift,
		Presence:          planningOnlyPresence{},
	})
	require.NoError(t, err)
	return detection
}
