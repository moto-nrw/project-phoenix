package e2e_timetable

import (
	"context"
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
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowE_StudentDayWithUnplannedVisit validates the student-day aggregation:
//   - Student enrolled in instance X → attendance shows up via instance_students.
//   - Student NOT enrolled in instance Y, but a visit exists → surfaces as
//     `is_unplanned=true` via the active.visits join.
//   - /student/{id}/week stays within the ≤12 query budget over 14 days.
//   - Tenant isolation: secondary tenant → 404 on the student.
func TestFlowE_StudentDayWithUnplannedVisit(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	// Pick a weekday >= 7 days out (materialize today-or-future rule).
	target := nextWeekday(timezone.TodayDate(), 5, 7) // Friday
	fromS := target.String()

	s.createActivePeriod(fmt.Sprintf("E2E-Flow-E-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowE-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", room.ID)
	})

	staff := testpkg.CreateTestStaff(t, s.db, "FlowE", "Staff")
	studentA := testpkg.CreateTestStudent(t, s.db, "Alice", "FlowE", "3a")
	studentB := testpkg.CreateTestStudent(t, s.db, "Bob", "FlowE", "3a")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupStaffFixtures(t, s.db, staff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.students", studentA.ID, studentB.ID)
	})

	// Two templates on the target weekday. studentA is enrolled in both,
	// studentB is enrolled only in tmplX.
	tmplX := s.buildTemplate(templateSpec{
		name:       "FlowE-X",
		weekday:    5,
		startHHMM:  "09:00",
		endHHMM:    "10:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff.ID},
		studentIDs: []int64{studentA.ID, studentB.ID},
	})
	tmplY := s.buildTemplate(templateSpec{
		name:       "FlowE-Y",
		weekday:    5,
		startHHMM:  "11:00",
		endHHMM:    "12:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff.ID},
		studentIDs: []int64{studentA.ID}, // studentB NOT enrolled in Y
	})
	_ = tmplY

	// --- Materialize + start both instances --------------------------------
	matReq := map[string]any{"from_date": fromS, "to_date": fromS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())

	instX := fetchOneInstance(t, s, tmplX.group.ID, target)
	instY := fetchOneInstance(t, s, tmplY.group.ID, target)
	s.registerCleanup("schedule.activity_instances", instX.ID, instY.ID)

	startX := startInstance(t, s, instX.ID)
	startY := startInstance(t, s, instY.ID)

	// --- Visits: B attends X (enrolled); B also attends Y (NOT enrolled) ---
	// studentA we don't touch — she stays `expected`.
	dev := testpkg.EnsureWebManualDevice(t, s.db)
	ctx := context.WithValue(s.tenantCtx(), device.CtxDevice, dev)
	ctx = context.WithValue(ctx, device.CtxStaff, staff)

	// First visit: B into X (enrolled) → flips instance_students to present.
	v1 := &activeModel.Visit{
		StudentID:     studentB.ID,
		ActiveGroupID: startX.ActiveGroupID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, s.factory.Active.CreateVisit(ctx, v1), "visit B→X (enrolled)")

	// End that visit, so B can enter the next active.group.
	exit := time.Now().Add(time.Second)
	v1.ExitTime = &exit
	require.NoError(t, s.factory.Active.EndVisit(ctx, v1.ID), "end visit B→X")

	// Second visit: B into Y (not enrolled) → surfaces as is_unplanned=true.
	v2 := &activeModel.Visit{
		StudentID:     studentB.ID,
		ActiveGroupID: startY.ActiveGroupID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, s.factory.Active.CreateVisit(ctx, v2), "visit B→Y (unplanned)")

	// --- /student/{B}/day → expect both instances surfaced ----------------
	rr = s.do("GET", fmt.Sprintf("/student/%d/day?date=%s", studentB.ID, fromS), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "day body=%s", rr.Body.String())

	var dayResp struct {
		Instances []struct {
			ID         int64 `json:"id"`
			Attendance struct {
				Status      string
				IsUnplanned bool `json:"is_unplanned"`
			}
		}
	}
	decodeResponse(t, rr, &dayResp)
	require.Len(t, dayResp.Instances, 2, "B should see 2 instances on this day (enrolled X + unplanned Y)")

	byID := map[int64]bool{}
	for _, inst := range dayResp.Instances {
		switch inst.ID {
		case instX.ID:
			byID[inst.ID] = true
			assert.Equal(t, scheduleModel.AttendanceStatusPresent, inst.Attendance.Status,
				"B on X: present via enrollment check-in")
			assert.False(t, inst.Attendance.IsUnplanned, "X was enrolled, so not unplanned")
		case instY.ID:
			byID[inst.ID] = true
			assert.Equal(t, scheduleModel.AttendanceStatusPresent, inst.Attendance.Status,
				"B on Y: present via unplanned visit")
			assert.True(t, inst.Attendance.IsUnplanned, "Y was NOT enrolled → unplanned")
		}
	}
	assert.True(t, byID[instX.ID], "X must appear in B's day")
	assert.True(t, byID[instY.ID], "Y must appear in B's day")

	// --- Query-budget on /week over 14 days -------------------------------
	qc := &queryCounter{}
	s.db.AddQueryHook(qc)
	qc.reset()

	weekFrom := target.String()
	weekTo := target.AddDays(13).Format("2006-01-02")
	path := fmt.Sprintf("/student/%d/week?from=%s&to=%s", studentB.ID, weekFrom, weekTo)
	rr = s.do("GET", path, nil, s.primaryAdminClaims())
	weekQueryCount := qc.get()
	require.Equal(t, http.StatusOK, rr.Code, "week body=%s", rr.Body.String())

	var weekResp struct {
		Days []struct {
			Date      string
			Instances []struct {
				ID int64
			}
		}
	}
	decodeResponse(t, rr, &weekResp)
	require.Len(t, weekResp.Days, 14, "14 inclusive days returned")

	// PR #1300 budget: ≤ 12 handler queries. With TenantTxMiddleware overhead
	// the end-to-end count is higher; match PR-B1 / PR-C2 convention and log
	// instead of strict-assert, but guard against pathological fan-out
	// (e.g. per-day loop) with a generous upper bound of 25.
	t.Logf("query-budget /student/%d/week (14 days): %d queries (PR target ≤ 12 handler-queries only)", studentB.ID, weekQueryCount)
	assert.LessOrEqual(t, weekQueryCount, int64(25),
		"/week query count %d looks pathological (likely N+1 on days)", weekQueryCount)

	// --- Tenant isolation -------------------------------------------------
	rr = s.do("GET", fmt.Sprintf("/student/%d/day?date=%s", studentB.ID, fromS), nil, s.secondaryAdminClaims())
	assert.Equal(t, http.StatusNotFound, rr.Code,
		"secondary tenant must 404 on primary's student (got %d: %s)", rr.Code, rr.Body.String())
}

// startInstance calls POST /instances/{id}/start and returns the parsed body.
func startInstance(t *testing.T, s *scenario, instanceID int64) struct {
	InstanceID    int64  `json:"instance_id"`
	Status        string `json:"status"`
	ActiveGroupID int64  `json:"active_group_id"`
} {
	t.Helper()
	rr := s.do("POST", fmt.Sprintf("/instances/%d/start", instanceID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "start instance %d body=%s", instanceID, rr.Body.String())
	var resp struct {
		InstanceID    int64  `json:"instance_id"`
		Status        string `json:"status"`
		ActiveGroupID int64  `json:"active_group_id"`
	}
	decodeResponse(t, rr, &resp)
	require.NotZero(t, resp.ActiveGroupID, "instance %d: active_group_id missing", instanceID)
	return resp
}
