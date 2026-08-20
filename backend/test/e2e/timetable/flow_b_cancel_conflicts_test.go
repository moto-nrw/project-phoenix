package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowB_CancelAndExceptionConflicts covers the admin "aktivitaet faellt
// aus" workflow:
//   - Materialize creates an instance + 3 instance_students (expected).
//   - Admin adds a `cancelled` exception for that date. The /exception-conflicts
//     endpoint surfaces two warnings (for students with arrival plans), and
//     skips the student with an explicit arrival absence.
//   - Swap in a `modified` exception with an earlier start_time; the endpoint
//     reports `modified_instance_time_mismatch` for arriving students.
//
// Tenant-isolation: the neighbor tenant sees an empty conflict list.
// Query-budget: ≤ 22 queries per B13.
// Deliberately NOT parallel: the test installs a query hook on the SHARED
// package pool and asserts a query budget, so any test running beside it is
// counted too.
func TestFlowB_CancelAndExceptionConflicts(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	// --- Setup: target Monday at 13:00–14:00 -------------------------------
	target := nextWeekday(timezone.TodayDate(), 1, 7) // Mon, >=7 days out
	fromS := target.String()

	s.createActivePeriod(fmt.Sprintf("E2E-Flow-B-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowB-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	staff1 := testpkg.CreateTestStaff(t, s.db, "FlowB", "Staff1")
	staff2 := testpkg.CreateTestStaff(t, s.db, "FlowB", "Staff2")
	alice := testpkg.CreateTestStudent(t, s.db, "Alice", "FlowB", "3a")
	bob := testpkg.CreateTestStudent(t, s.db, "Bob", "FlowB", "3a")
	cleo := testpkg.CreateTestStudent(t, s.db, "Cleo", "FlowB", "3a")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	tmpl := s.buildTemplate(templateSpec{
		name:       "Lesen-AG",
		weekday:    1, // Monday
		startHHMM:  "13:00",
		endHHMM:    "14:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff1.ID, staff2.ID},
		studentIDs: []int64{alice.ID, bob.ID, cleo.ID},
	})

	// Arrival schedules for Monday: alice + bob arrive 12:50. Cleo has an
	// arrival exception saying she won't come at all (absence).
	testpkg.CreateTestArrivalSchedule(t, s.db, alice.ID, 1, staff1.ID, "12:50")
	testpkg.CreateTestArrivalSchedule(t, s.db, bob.ID, 1, staff1.ID, "12:50")
	testpkg.CreateTestArrivalException(t, s.db, cleo.ID, target, staff1.ID, "", "krank")
	// Arrival-schedules/exceptions are captured by teardown via the
	// scenario's cleanupOrder; no explicit registerCleanup needed because
	// rows belong to the students we delete via users.students cascade would
	// not cover — but the teardown DELETE-by-ID loop targets exactly these
	// tables. Explicit registration keeps things tidy.
	s.registerArrivalFixtures(t)

	// --- Step 1: materialize -----------------------------------------------
	matReq := map[string]any{"from_date": fromS, "to_date": fromS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())

	var matResp struct {
		InstancesCreated        int `json:"instances_created"`
		InstanceStudentsCreated int `json:"instance_students_created"`
	}
	decodeResponse(t, rr, &matResp)
	require.Equal(t, 1, matResp.InstancesCreated)
	require.Equal(t, 3, matResp.InstanceStudentsCreated)

	instance := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", instance.ID)

	// --- Step 2: insert cancelled exception --------------------------------
	cancelledExc := &scheduleModel.ActivityException{
		ActivityGroupID: tmpl.group.ID,
		ExceptionDate:   target,
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		Reason:          testpkg.StrPtr("Heizung kaputt"),
	}
	cancelledExc.SetTenantID(s.primaryTenant)
	_, err := s.db.NewInsert().
		Model(cancelledExc).
		ModelTableExpr(`schedule.activity_exceptions`).
		Exec(s.tenantCtx())
	require.NoError(t, err, "insert cancelled exception")
	s.registerCleanup("schedule.activity_exceptions", cancelledExc.ID)

	// --- Step 3: re-materialize — skipped_exception, no new instance -------
	rr = s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "rematerialize body=%s", rr.Body.String())

	var matResp2 struct {
		InstancesCreated           int `json:"instances_created"`
		CandidatesSkippedExisting  int `json:"candidates_skipped_existing"`
		CandidatesSkippedException int `json:"candidates_skipped_exception"`
	}
	decodeResponse(t, rr, &matResp2)
	assert.Equal(t, 0, matResp2.InstancesCreated, "no new instance created")
	// Empirical behaviour (WP-B8): the materialization service checks
	// exceptions BEFORE existing-row dedupe, so a cancelled exception
	// short-circuits the candidate and counts as skipped_exception even
	// when a planned instance already exists for that date. The existing
	// instance itself stays in place — the next steps operate on it.
	assert.Equal(t, 1, matResp2.CandidatesSkippedException,
		"cancelled exception skips the candidate even with an existing planned instance")
	assert.Equal(t, 0, matResp2.CandidatesSkippedExisting)

	// --- Step 4: /exception-conflicts surfaces cancelled warnings ---------
	// Attach the query counter now so we measure a real request.
	qc := &queryCounter{}
	s.db.AddQueryHook(qc)
	qc.reset()

	path := fmt.Sprintf("/exception-conflicts?date=%s&date_to=%s", fromS, fromS)
	rr = s.do("GET", path, nil, s.primaryAdminClaims())
	cancelledQueryCount := qc.get()

	require.Equal(t, http.StatusOK, rr.Code, "exception-conflicts body=%s", rr.Body.String())

	var conflictsResp struct {
		Conflicts []struct {
			Kind               string
			Date               string
			ActivityGroupID    int64  `json:"activity_group_id"`
			InstanceID         int64  `json:"instance_id"`
			StudentID          int64  `json:"student_id"`
			ExpectedArrival    string `json:"expected_arrival"`
			ArrivalSource      string `json:"arrival_source"`
			CancellationReason string `json:"cancellation_reason"`
		}
	}
	decodeResponse(t, rr, &conflictsResp)

	require.Len(t, conflictsResp.Conflicts, 2,
		"alice + bob have arrival schedules → 2 warnings; cleo has explicit absence → suppressed")

	studentsSeen := map[int64]bool{}
	for _, c := range conflictsResp.Conflicts {
		studentsSeen[c.StudentID] = true
		assert.Equal(t, "cancelled_instance_with_scheduled_arrivals", c.Kind)
		assert.Equal(t, "12:50", c.ExpectedArrival)
		assert.Equal(t, "schedule", c.ArrivalSource)
		assert.Equal(t, "Heizung kaputt", c.CancellationReason)
		assert.Equal(t, instance.ID, c.InstanceID)
	}
	assert.True(t, studentsSeen[alice.ID], "alice must be in warnings")
	assert.True(t, studentsSeen[bob.ID], "bob must be in warnings")
	assert.False(t, studentsSeen[cleo.ID], "cleo (explicit absence) must NOT be in warnings")

	// Query-budget — PR #1304 guarantees ≤ 22 queries regardless of fan-out.
	// We include the TenantTxMiddleware SET LOCAL overhead in this count;
	// the production repo budget is ≤ 22 for the handler body.
	assert.LessOrEqual(t, cancelledQueryCount, int64(22),
		"/exception-conflicts (cancelled) query count %d exceeds budget of 22", cancelledQueryCount)

	// --- Step 5: swap in a modified exception ------------------------------
	_, err = s.db.NewDelete().
		Model((*scheduleModel.ActivityException)(nil)).
		ModelTableExpr(`schedule.activity_exceptions`).
		Where("id = ?", cancelledExc.ID).
		Exec(s.tenantCtx())
	require.NoError(t, err, "delete cancelled exception")

	modifiedStart := parseHHMM(t, "12:30")
	modifiedExc := &scheduleModel.ActivityException{
		ActivityGroupID: tmpl.group.ID,
		ExceptionDate:   target,
		ExceptionType:   scheduleModel.ActivityExceptionModified,
		StartTime:       &modifiedStart,
		Reason:          testpkg.StrPtr("Fruehschluss"),
	}
	modifiedExc.SetTenantID(s.primaryTenant)
	_, err = s.db.NewInsert().
		Model(modifiedExc).
		ModelTableExpr(`schedule.activity_exceptions`).
		Exec(s.tenantCtx())
	require.NoError(t, err, "insert modified exception")
	s.registerCleanup("schedule.activity_exceptions", modifiedExc.ID)

	qc.reset()
	rr = s.do("GET", path, nil, s.primaryAdminClaims())
	modifiedQueryCount := qc.get()
	require.Equal(t, http.StatusOK, rr.Code, "exception-conflicts(modified) body=%s", rr.Body.String())

	var modResp struct {
		Conflicts []struct {
			Kind              string
			StudentID         int64  `json:"student_id"`
			ExpectedArrival   string `json:"expected_arrival"`
			ArrivalSource     string `json:"arrival_source"`
			ModifiedStartTime string `json:"modified_start_time"`
			OriginalStartTime string `json:"original_start_time"`
		}
	}
	decodeResponse(t, rr, &modResp)
	require.Len(t, modResp.Conflicts, 2,
		"alice + bob arrive 12:50 > modified 12:30 → 2 modified-mismatch warnings")

	studentsSeen = map[int64]bool{}
	for _, c := range modResp.Conflicts {
		studentsSeen[c.StudentID] = true
		assert.Equal(t, "modified_instance_time_mismatch", c.Kind)
		assert.Equal(t, "12:50", c.ExpectedArrival)
		assert.Equal(t, "12:30", c.ModifiedStartTime)
		assert.Equal(t, "13:00", c.OriginalStartTime,
			"template still schedules at 13:00 → original_start_time surfaces")
	}
	assert.True(t, studentsSeen[alice.ID])
	assert.True(t, studentsSeen[bob.ID])
	assert.False(t, studentsSeen[cleo.ID], "cleo is absent → modified mismatch suppressed too")

	assert.LessOrEqual(t, modifiedQueryCount, int64(22),
		"/exception-conflicts (modified) query count %d exceeds budget of 22", modifiedQueryCount)

	// --- Step 6: tenant isolation -----------------------------------------
	rr = s.do("GET", path, nil, s.secondaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "tenant2 body=%s", rr.Body.String())
	var t2Resp struct {
		Conflicts []struct {
			Kind string
		}
	}
	decodeResponse(t, rr, &t2Resp)
	assert.Empty(t, t2Resp.Conflicts, "secondary tenant must see no conflicts from primary")
}

// registerArrivalFixtures adds the three arrival-schedule/exception tables
// to the scenario cleanup plan. The helpers already cover the dedicated
// schedule.student_arrival_* tables in cleanupOrder, but rows created via
// testpkg.CreateTestArrivalSchedule don't go through registerCleanup — they
// inherit the scenario's tenant and get picked up by table-level cleanup only
// if we remember the IDs. Simpler: do a targeted DELETE at teardown.
func (s *scenario) registerArrivalFixtures(_ *testing.T) {
	s.extraCleanup = append(s.extraCleanup, func() {
		ctx := s.tenantCtx()
		_, _ = s.db.NewDelete().
			TableExpr("schedule.student_arrival_schedules").
			Where("tenant_id = ?", s.primaryTenant).
			Where("created_at > NOW() - INTERVAL '5 minutes'").
			Exec(ctx)
		_, _ = s.db.NewDelete().
			TableExpr("schedule.student_arrival_exceptions").
			Where("tenant_id = ?", s.primaryTenant).
			Where("created_at > NOW() - INTERVAL '5 minutes'").
			Exec(ctx)
	})
}
