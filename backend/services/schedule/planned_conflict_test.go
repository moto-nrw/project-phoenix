// Hermetic integration tests for DetectPlannedConflicts (the planning-time
// conflict probe behind GET /api/timetable/conflicts).
//
// Matrix:
//   - room overlap → warning; pure adjacency (end == start) → none
//   - staff assigned to overlapping instance → warning; IsAbsent rows excluded
//   - student expected on overlapping instance → warning
//   - exclude_instance_id removes the instance being edited from the probe
//   - cancelled instances are ignored
//   - cross-tenant instances are invisible
//
// All fixtures via testpkg.CreateTest* + t.Cleanup — no hardcoded entity IDs.
package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type plannedConflictSetup struct {
	deps      scheduleSvc.PlannedConflictDependencies
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	staffID   int64
	studentID int64
	date      timezone.Date
}

func buildPlannedConflictSetup(t *testing.T) *plannedConflictSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repoFactory := repositories.NewFactory(db)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("PC-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "PC", fmt.Sprintf("Staff-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "PC-Kid", fmt.Sprintf("One-%d", suffix), "2b")
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, 0, 0, room.ID)
	})

	return &plannedConflictSetup{
		deps: scheduleSvc.PlannedConflictDependencies{
			InstanceRepo:      repoFactory.ActivityInstance,
			InstanceStaffRepo: repoFactory.InstanceStaff,
			InstanceStudents:  repoFactory.InstanceStudent,
		},
		db:        db,
		ctx:       testpkg.TenantContext(1),
		roomID:    room.ID,
		staffID:   staff.ID,
		studentID: student.ID,
		date:      timezone.NewDate(2026, time.June, 22), // fixed Monday, period-independent
	}
}

// probeQuery builds the default probe slot 14:30–15:30, overlapping the
// fixture instances' default 14:00–15:00 window.
func probeQuery(s *plannedConflictSetup, mutate func(*scheduleSvc.PlannedConflictQuery)) scheduleSvc.PlannedConflictQuery {
	q := scheduleSvc.PlannedConflictQuery{
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
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
	return inst
}

func TestDetectPlannedConflicts_RoomOverlap(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Belegt"})

	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.RoomID = &s.roomID
	}), nil)

	require.Len(t, warnings, 1)
	assert.Equal(t, scheduleSvc.ConflictKindRoom, warnings[0].Kind)
	assert.Equal(t, s.roomID, warnings[0].ResourceID)
	assert.Equal(t, inst.ID, warnings[0].ConflictingInstanceID)
	assert.Equal(t, "PC-Belegt", warnings[0].ConflictingTitle)
	assert.Contains(t, warnings[0].Message, "22.06.2026")
	assert.Contains(t, warnings[0].Message, "14:00")
	assert.Contains(t, warnings[0].Message, "PC-Belegt")
}

func TestDetectPlannedConflicts_AdjacencyIsNotAConflict(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Davor"}) // 14:00–15:00

	// Probe 15:00–16:00 — touching edges must not conflict.
	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.StartTime = time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
		q.EndTime = time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC)
		q.RoomID = &s.roomID
	}), nil)

	assert.Empty(t, warnings)
}

func TestDetectPlannedConflicts_Staff_IncludingAbsentExclusion(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Personal"})

	suffix := time.Now().UnixNano()
	absentStaff := testpkg.CreateTestStaff(t, s.db, "PC", fmt.Sprintf("Absent-%d", suffix))
	row1 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})
	row2 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, absentStaff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, row1.ID, row2.ID)
		testpkg.CleanupActivityFixtures(t, s.db, 0, absentStaff.ID, 0, 0, 0)
	})

	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.StaffIDs = []int64{s.staffID, absentStaff.ID}
	}), nil)

	require.Len(t, warnings, 1, "is_absent staff must not be flagged")
	assert.Equal(t, scheduleSvc.ConflictKindStaff, warnings[0].Kind)
	assert.Equal(t, s.staffID, warnings[0].ResourceID)
	assert.Equal(t, inst.ID, warnings[0].ConflictingInstanceID)
	assert.Contains(t, warnings[0].Message, "PC-Personal")
}

func TestDetectPlannedConflicts_Student(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Kind"})

	row := testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, s.studentID, "")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.instance_students", row.ID) })

	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.StudentIDs = []int64{s.studentID}
	}), nil)

	require.Len(t, warnings, 1)
	assert.Equal(t, scheduleSvc.ConflictKindStudent, warnings[0].Kind)
	assert.Equal(t, s.studentID, warnings[0].ResourceID)
	assert.Equal(t, inst.ID, warnings[0].ConflictingInstanceID)
	assert.Contains(t, warnings[0].Message, "PC-Kind")
}

func TestDetectPlannedConflicts_ExcludeSelf(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Selbst"})

	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.RoomID = &s.roomID
		q.ExcludeInstanceID = &inst.ID
	}), nil)

	assert.Empty(t, warnings, "the instance being edited must not conflict with itself")
}

func TestDetectPlannedConflicts_CancelledIgnored(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{
		Title:  "PC-Abgesagt",
		Status: scheduleModels.InstanceStatusCancelled,
	})

	warnings := scheduleSvc.DetectPlannedConflicts(s.ctx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.RoomID = &s.roomID
	}), nil)

	assert.Empty(t, warnings)
}

func TestDetectPlannedConflicts_CrossTenantInvisible(t *testing.T) {
	s := buildPlannedConflictSetup(t)
	inst := seedConflictInstance(t, s, testpkg.ActivityInstanceOpts{Title: "PC-Fremd"})
	row := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffID, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, row.ID) })

	otherCtx := testpkg.TenantContext(999)
	warnings := scheduleSvc.DetectPlannedConflicts(otherCtx, s.deps, probeQuery(s, func(q *scheduleSvc.PlannedConflictQuery) {
		q.RoomID = &s.roomID
		q.StaffIDs = []int64{s.staffID}
	}), nil)

	assert.Empty(t, warnings, "tenant 999 must not see tenant 1 instances")
}
