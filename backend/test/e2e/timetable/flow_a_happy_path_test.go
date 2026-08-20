package e2e_timetable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowA_PlanToReport exercises the core timetable chain:
// admin materializes templates → staff starts the instance → two students
// check in → one student never shows → admin completes → /student/{id}/day
// reports the right state. Tenant-isolation is verified at the end.
func TestFlowA_PlanToReport(t *testing.T) {
	t.Parallel()

	s := newScenario(t)
	defer s.teardown()

	// --- Setup: period, room, staff, students, template --------------------
	// Pick a Wednesday far enough ahead that the scheduler accepts the window.
	target := nextWeekday(timezone.TodayDate(), 3, 7) // Wed, ≥7 days out
	s.createActivePeriod(fmt.Sprintf("E2E-Flow-A-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowA-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	staff1 := testpkg.CreateTestStaff(t, s.db, "Frau", "Schmidt")
	staff2 := testpkg.CreateTestStaff(t, s.db, "Herr", "Meier")
	student1 := testpkg.CreateTestStudent(t, s.db, "Anna", "A", "3a")
	student2 := testpkg.CreateTestStudent(t, s.db, "Ben", "B", "3a")
	student3 := testpkg.CreateTestStudent(t, s.db, "Cleo", "C", "3a")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	tmpl := s.buildTemplate(templateSpec{
		name:       "Mathe-AG",
		weekday:    3,
		startHHMM:  "14:00",
		endHHMM:    "15:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff1.ID, staff2.ID},
		studentIDs: []int64{student1.ID, student2.ID, student3.ID},
	})

	// --- Step 1: materialize the target week -------------------------------
	fromS := target.String()
	toS := fromS
	matReq := map[string]any{"from_date": fromS, "to_date": toS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())

	var matResp struct {
		InstancesCreated        int `json:"instances_created"`
		InstanceStudentsCreated int `json:"instance_students_created"`
		InstanceStaffCreated    int `json:"instance_staff_created"`
	}
	decodeResponse(t, rr, &matResp)
	assert.Equal(t, 1, matResp.InstancesCreated)
	assert.Equal(t, 3, matResp.InstanceStudentsCreated)
	assert.Equal(t, 2, matResp.InstanceStaffCreated)

	// Find the created instance (for later asserts + start/complete).
	instance := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", instance.ID)
	require.Equal(t, scheduleModel.InstanceStatusPlanned, instance.Status)

	// --- Step 2: assert DB baseline ---------------------------------------
	instStudents := fetchInstanceStudents(t, s, instance.ID)
	require.Len(t, instStudents, 3)
	for _, is := range instStudents {
		assert.Equal(t, scheduleModel.AttendanceStatusExpected, is.Status)
	}
	require.Equal(t, 2, countInstanceStaff(t, s, instance.ID))

	// --- Step 3: start the instance ----------------------------------------
	rr = s.do("POST", fmt.Sprintf("/instances/%d/start", instance.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "start body=%s", rr.Body.String())

	var startResp struct {
		InstanceID    int64             `json:"instance_id"`
		Status        string            `json:"status"`
		ActiveGroupID int64             `json:"active_group_id"`
		Warnings      []json.RawMessage `json:"warnings"`
	}
	decodeResponse(t, rr, &startResp)
	assert.Equal(t, scheduleModel.InstanceStatusActive, startResp.Status)
	require.NotZero(t, startResp.ActiveGroupID, "active_group_id must be set")
	assert.Empty(t, startResp.Warnings, "clean start should have no warnings")

	// --- Step 4: simulate two student check-ins via ActiveService ---------
	// We're not hitting the IoT HTTP endpoint here — that's PyrePortal's path.
	// We go directly through the active service so the attendance-sync mirror
	// is exercised end-to-end (B10 side effect).
	checkInStudent(t, s, student1.ID, startResp.ActiveGroupID, staff1)
	checkInStudent(t, s, student2.ID, startResp.ActiveGroupID, staff1)

	// --- Step 5: assert attendance sync flipped instance_students ----------
	instStudents = fetchInstanceStudents(t, s, instance.ID)
	byStudent := map[int64]*scheduleModel.InstanceStudent{}
	for i := range instStudents {
		byStudent[instStudents[i].StudentID] = &instStudents[i]
	}
	require.Contains(t, byStudent, student1.ID)
	require.Contains(t, byStudent, student2.ID)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, byStudent[student1.ID].Status)
	assert.NotNil(t, byStudent[student1.ID].CheckedInAt)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, byStudent[student2.ID].Status)
	assert.NotNil(t, byStudent[student2.ID].CheckedInAt)
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, byStudent[student3.ID].Status,
		"third student never checked in, should still be expected pre-complete")

	// --- Step 6: complete the instance ------------------------------------
	rr = s.do("POST", fmt.Sprintf("/instances/%d/complete", instance.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "complete body=%s", rr.Body.String())

	var completeResp struct {
		InstanceID  int64  `json:"instance_id"`
		Status      string `json:"status"`
		CompletedAt string `json:"completed_at"`
	}
	decodeResponse(t, rr, &completeResp)
	assert.Equal(t, scheduleModel.InstanceStatusCompleted, completeResp.Status)
	require.NotEmpty(t, completeResp.CompletedAt)

	// --- Step 7: assert complete marked the no-show absent -----------------
	instStudents = fetchInstanceStudents(t, s, instance.ID)
	byStudent = map[int64]*scheduleModel.InstanceStudent{}
	for i := range instStudents {
		byStudent[instStudents[i].StudentID] = &instStudents[i]
	}
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, byStudent[student1.ID].Status,
		"student1 stays present (B10 monotonicity)")
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, byStudent[student2.ID].Status,
		"student2 stays present")
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, byStudent[student3.ID].Status,
		"student3 flips expected → absent on complete (B11 fix)")

	// --- Step 8: student day report for each student ----------------------
	for _, studentID := range []int64{student1.ID, student2.ID, student3.ID} {
		path := fmt.Sprintf("/student/%d/day?date=%s", studentID, fromS)
		rr = s.do("GET", path, nil, s.primaryAdminClaims())
		require.Equal(t, http.StatusOK, rr.Code,
			"/student/%d/day body=%s", studentID, rr.Body.String())

		var dayResp struct {
			StudentID int64 `json:"student_id"`
			Instances []struct {
				ID         int64 `json:"id"`
				Status     string
				Attendance struct {
					Status      string
					IsUnplanned bool `json:"is_unplanned"`
				}
			}
		}
		decodeResponse(t, rr, &dayResp)
		require.Len(t, dayResp.Instances, 1, "student %d day: exactly one instance", studentID)
		entry := dayResp.Instances[0]
		assert.Equal(t, instance.ID, entry.ID)
		assert.Equal(t, scheduleModel.InstanceStatusCompleted, entry.Status)
		assert.False(t, entry.Attendance.IsUnplanned)

		expected := scheduleModel.AttendanceStatusPresent
		if studentID == student3.ID {
			expected = scheduleModel.AttendanceStatusAbsent
		}
		assert.Equal(t, expected, entry.Attendance.Status,
			"student %d expected attendance=%s", studentID, expected)
	}

	// --- Step 9: tenant isolation -----------------------------------------
	rr = s.do("GET", fmt.Sprintf("/student/%d/day?date=%s", student1.ID, fromS), nil, s.secondaryAdminClaims())
	assert.Equal(t, http.StatusNotFound, rr.Code,
		"secondary tenant must not see primary's student (got %d: %s)", rr.Code, rr.Body.String())
}

// fetchOneInstance fetches the single materialized instance for (template, date).
func fetchOneInstance(t *testing.T, s *scenario, templateID int64, date timezone.Date) *scheduleModel.ActivityInstance {
	t.Helper()
	var inst scheduleModel.ActivityInstance
	err := s.db.NewSelect().
		Model(&inst).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".activity_group_id = ?`, templateID).
		Where(`"activity_instance".date = ?`, date).
		Where(`"activity_instance".tenant_id = ?`, s.primaryTenant).
		Scan(s.tenantCtx())
	require.NoError(t, err, "fetch instance for template %d on %s", templateID, date)
	return &inst
}

// fetchInstanceStudents loads all instance_students rows for an instance.
func fetchInstanceStudents(t *testing.T, s *scenario, instanceID int64) []scheduleModel.InstanceStudent {
	t.Helper()
	var rows []scheduleModel.InstanceStudent
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".tenant_id = ?`, s.primaryTenant).
		Scan(s.tenantCtx())
	require.NoError(t, err, "fetch instance_students for %d", instanceID)
	return rows
}

// countInstanceStaff returns the number of instance_staff rows.
func countInstanceStaff(t *testing.T, s *scenario, instanceID int64) int {
	t.Helper()
	n, err := s.db.NewSelect().
		Model((*scheduleModel.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Where(`"instance_staff".tenant_id = ?`, s.primaryTenant).
		Count(s.tenantCtx())
	require.NoError(t, err, "count instance_staff for %d", instanceID)
	return n
}

// checkInStudent creates a visit through the real active service, mirroring
// what a PyrePortal check-in does (minus the HTTP layer). The attendance-sync
// mirror (B10) runs as a side effect and flips instance_students.status.
func checkInStudent(t *testing.T, s *scenario, studentID, activeGroupID int64, staff *usersModel.Staff) {
	t.Helper()
	// Materialize a web-manual device so we have a deviceID to attribute
	// the visit to. Matches what the web check-in path does.
	dev := testpkg.EnsureWebManualDevice(t, s.db)

	ctx := context.WithValue(s.tenantCtx(), device.CtxDevice, dev)
	ctx = context.WithValue(ctx, device.CtxStaff, staff)

	visit := &activeModel.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     time.Now(),
	}
	err := s.factory.Active.CreateVisit(ctx, visit)
	require.NoError(t, err, "CreateVisit for student %d", studentID)
}
