package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowC_GapsAndSubstitute exercises the Krankmeldung scenario:
// a staff member calls in sick on the day of two concurrent instances,
// the admin swaps in a substitute across all affected instances, a follow-up
// conflict attempt is rejected atomically (no partial writes), and /gaps
// surfaces an unrelated instance with zero non-absent staff.
func TestFlowC_GapsAndSubstitute(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := setupTimetableScenarioModule(t)

	// Pick a Tuesday ≥ 7 days out (must be today-or-future for /gaps and /substitute).
	target := nextWeekday(timezone.TodayDate(), 2, 7)
	fromS := target.String()

	s.createActivePeriod(fmt.Sprintf("E2E-Flow-C-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowC-Room")

	staff1 := testpkg.CreateTestStaff(t, s.db, "FlowC", "S1-Primary")
	staff2 := testpkg.CreateTestStaff(t, s.db, "FlowC", "S2-Co")
	staff3 := testpkg.CreateTestStaff(t, s.db, "FlowC", "S3-Sub")
	staff4 := testpkg.CreateTestStaff(t, s.db, "FlowC", "S4-Other")
	staff5 := testpkg.CreateTestStaff(t, s.db, "FlowC", "S5-GapOnly")
	student := testpkg.CreateTestStudent(t, s.db, "Anna", "FlowC", "3a")

	// Two templates on the same weekday at different times.
	tmpl1 := s.buildTemplate(templateSpec{
		name:       "FlowC-AG1",
		weekday:    2,
		startHHMM:  "10:00",
		endHHMM:    "11:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff1.ID, staff2.ID},
		studentIDs: []int64{student.ID},
	})
	tmpl2 := s.buildTemplate(templateSpec{
		name:       "FlowC-AG2",
		weekday:    2,
		startHHMM:  "14:00",
		endHHMM:    "15:00",
		roomID:     room.ID,
		staffIDs:   []int64{staff1.ID, staff2.ID},
		studentIDs: []int64{student.ID},
	})

	// --- Step 1: materialize both templates --------------------------------
	matReq := map[string]any{"from_date": fromS, "to_date": fromS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())
	var matResp struct {
		InstancesCreated int `json:"instances_created"`
	}
	decodeResponse(t, rr, &matResp)
	require.Equal(t, 2, matResp.InstancesCreated)

	inst1 := fetchOneInstance(t, s, tmpl1.group.ID, target)
	inst2 := fetchOneInstance(t, s, tmpl2.group.ID, target)

	// --- Step 2: start inst1 so we can assert active.group_supervisors rotate
	rr = s.do("POST", fmt.Sprintf("/instances/%d/start", inst1.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "start body=%s", rr.Body.String())

	var startResp struct {
		ActiveGroupID int64 `json:"active_group_id"`
		Status        string
	}
	decodeResponse(t, rr, &startResp)
	require.Equal(t, scheduleModel.InstanceStatusActive, startResp.Status)
	require.NotZero(t, startResp.ActiveGroupID)

	// --- Step 3: /gaps — baseline should be empty --------------------------
	qc := testpkg.CaptureQueries(t, s.db)

	rr = s.do("GET", fmt.Sprintf("/gaps?date=%s&date_to=%s", fromS, fromS), nil, s.primaryAdminClaims())
	gapsQueryCount := qc.Total()
	require.Equal(t, http.StatusOK, rr.Code, "gaps body=%s", rr.Body.String())

	var gapsResp1 struct {
		Gaps []struct {
			InstanceID int64 `json:"instance_id"`
		}
	}
	decodeResponse(t, rr, &gapsResp1)
	assert.Empty(t, gapsResp1.Gaps, "baseline: both instances have 2 non-absent staff")

	t.Logf("query-budget /gaps baseline: %d queries (PR #1303 target ≤ 2)", gapsQueryCount)

	// --- Step 4: substitute S1 → S3 across both instances -----------------
	// Konsolidierung (#1886): die Vertretung läuft über den atomaren
	// Deviations-Save; die Zuweisung bleibt tagesweit über alle Blöcke.
	subReq := map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": staff1.ID, "substitute_staff_id": staff3.ID},
		},
	}
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst1.ID), subReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "substitute body=%s", rr.Body.String())

	var subResp struct {
		AffectedInstances []struct {
			InstanceID int64  `json:"instance_id"`
			Action     string `json:"action"`
		} `json:"affected_instances"`
	}
	decodeResponse(t, rr, &subResp)
	require.Len(t, subResp.AffectedInstances, 2, "both instances touched")
	for _, ai := range subResp.AffectedInstances {
		assert.Equal(t, "substituted", ai.Action, "instance %d", ai.InstanceID)
	}

	// --- Step 5: DB-assert the staff mutations -----------------------------
	// For each instance: the S1 row is_absent=true, a new S3 row
	// is_substitute=true.
	for _, instID := range []int64{inst1.ID, inst2.ID} {
		rows := fetchInstanceStaff(t, s, instID)
		var s1, s3 *scheduleModel.InstanceStaff
		for i := range rows {
			switch rows[i].StaffID {
			case staff1.ID:
				s1 = &rows[i]
			case staff3.ID:
				s3 = &rows[i]
			}
		}
		require.NotNil(t, s1, "instance %d: S1 row still present", instID)
		require.NotNil(t, s3, "instance %d: S3 substitute row inserted", instID)
		assert.True(t, s1.IsAbsent, "S1 flipped to is_absent=true on instance %d", instID)
		assert.False(t, s1.IsSubstitute, "S1 original row stays is_substitute=false")
		assert.True(t, s3.IsSubstitute, "S3 new row is_substitute=true on instance %d", instID)
		assert.False(t, s3.IsAbsent)
	}

	// For inst1 (active): active.group_supervisors was rotated.
	supervisors := fetchGroupSupervisors(t, s, startResp.ActiveGroupID)
	var supS1, supS3 *activeModel.GroupSupervisor
	for i := range supervisors {
		switch supervisors[i].StaffID {
		case staff1.ID:
			supS1 = &supervisors[i]
		case staff3.ID:
			supS3 = &supervisors[i]
		}
	}
	require.NotNil(t, supS1, "original S1 supervisor row on active group")
	assert.NotNil(t, supS1.EndDate, "S1 supervisor end_date is set after substitute")
	require.NotNil(t, supS3, "S3 supervisor row inserted on active group")
	assert.Nil(t, supS3.EndDate, "S3 supervisor end_date is NULL (currently serving)")

	// --- Step 6: substitute conflict — different substitute for same absent.
	// Expected: 409 substitute_conflict, NO writes (dry-run-first).
	preconflictStaffRows := fetchInstanceStaff(t, s, inst1.ID)
	preconflictCount := len(preconflictStaffRows)

	conflictReq := map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": staff1.ID, "substitute_staff_id": staff4.ID},
		},
	}
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst1.ID), conflictReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusConflict, rr.Code,
		"second substitute with different sub expected 409 (got %d: %s)", rr.Code, rr.Body.String())

	// DB-assert: no S4 row added anywhere (atomicity).
	for _, instID := range []int64{inst1.ID, inst2.ID} {
		rows := fetchInstanceStaff(t, s, instID)
		for _, r := range rows {
			assert.NotEqual(t, staff4.ID, r.StaffID,
				"conflict attempt must not insert S4 row on instance %d (dry-run-first atomicity)", instID)
		}
	}
	assert.Equal(t, preconflictCount, len(fetchInstanceStaff(t, s, inst1.ID)),
		"instance_staff row count on inst1 must be unchanged after 409")

	// --- Step 7: gap scenario — spontaneous instance with only absent staff.
	// Direct fixture: planned instance with two instance_staff rows, both
	// is_absent=true. Simulates "nobody uncovered this before /gaps was
	// queried."
	startFixture := parseHHMMLocal(t, "16:00")
	endFixture := parseHHMMLocal(t, "17:00")
	gapInstance := &scheduleModel.ActivityInstance{
		Date:          scheduleModel.Date(target),
		Title:         "FlowC-Gap-Instance",
		StartTime:     startFixture,
		EndTime:       endFixture,
		RoomID:        room.ID,
		Status:        scheduleModel.InstanceStatusPlanned,
		IsSpontaneous: true,
	}
	gapInstance.SetTenantID(s.primaryTenant)
	_, err := s.db.NewInsert().Model(gapInstance).
		ModelTableExpr(`schedule.activity_instances`).Exec(s.tenantCtx())
	require.NoError(t, err, "insert gap instance")

	// Both staff on this instance are absent → qualifies as a gap.
	for _, stid := range []int64{staff1.ID, staff5.ID} {
		row := &scheduleModel.InstanceStaff{
			InstanceID: gapInstance.ID,
			StaffID:    stid,
			IsAbsent:   true,
		}
		row.SetTenantID(s.primaryTenant)
		_, err := s.db.NewInsert().Model(row).
			ModelTableExpr(`schedule.instance_staff`).Exec(s.tenantCtx())
		require.NoError(t, err, "insert absent instance_staff for gap instance")
	}

	// --- Step 8: /gaps must now surface the gap instance -------------------
	qc.Reset()
	rr = s.do("GET", fmt.Sprintf("/gaps?date=%s&date_to=%s", fromS, fromS), nil, s.primaryAdminClaims())
	gapsQueryCount2 := qc.Total()
	require.Equal(t, http.StatusOK, rr.Code, "gaps(with-gap) body=%s", rr.Body.String())

	var gapsResp2 struct {
		Gaps []struct {
			InstanceID         int64  `json:"instance_id"`
			Title              string `json:"title"`
			AssignedStaffCount int    `json:"assigned_staff_count"`
			AbsentStaffCount   int    `json:"absent_staff_count"`
			Status             string `json:"status"`
		}
	}
	decodeResponse(t, rr, &gapsResp2)
	require.Len(t, gapsResp2.Gaps, 1, "exactly one gap expected")
	g := gapsResp2.Gaps[0]
	assert.Equal(t, gapInstance.ID, g.InstanceID)
	// BUG-FINDING (E2E-C1): per PR #1303 docs the response should carry
	// `assigned_staff_count=2, absent_staff_count=2` for this input. The
	// handler hardcodes assigned_staff_count=0 (gaps.go:168) — the comment
	// there misreads the invariant: "nonAbsentCounts==0" means "no NON-absent
	// staff", not "no staff at all". Tracked in E2E_FINDINGS.md.
	assert.Equal(t, 2, g.AssignedStaffCount,
		"assigned_staff_count must reflect total assigned staff (PR #1303 contract); got 0 — see E2E_FINDINGS.md#E2E-C1")
	assert.Equal(t, 2, g.AbsentStaffCount, "all two are absent")
	assert.Equal(t, scheduleModel.InstanceStatusPlanned, g.Status)

	t.Logf("query-budget /gaps (with 1 gap over 3 instances): %d queries (TenantTxMiddleware overhead included; PR target ≤ 2 was handler-queries only)", gapsQueryCount2)

	// --- Step 9: tenant isolation --------------------------------------------
	// Secondary tenant attempting to substitute this tenant's staff: 404
	// because FindByID returns no row under the tenant-2 tx.
	rr = s.do("POST", "/substitute", subReq, s.secondaryAdminClaims())
	assert.Equal(t, http.StatusNotFound, rr.Code,
		"secondary tenant must not substitute primary's staff (got %d: %s)", rr.Code, rr.Body.String())
}

// fetchInstanceStaff returns all instance_staff rows for an instance.
func fetchInstanceStaff(t *testing.T, s *scenario, instanceID int64) []scheduleModel.InstanceStaff {
	t.Helper()
	var rows []scheduleModel.InstanceStaff
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Where(`"instance_staff".tenant_id = ?`, s.primaryTenant).
		Scan(s.tenantCtx())
	require.NoError(t, err, "fetch instance_staff for %d", instanceID)
	return rows
}

// fetchGroupSupervisors returns all supervisor rows for an active.group.
func fetchGroupSupervisors(t *testing.T, s *scenario, activeGroupID int64) []activeModel.GroupSupervisor {
	t.Helper()
	var rows []activeModel.GroupSupervisor
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, activeGroupID).
		Where(`"group_supervisor".tenant_id = ?`, s.primaryTenant).
		Scan(s.tenantCtx())
	require.NoError(t, err, "fetch group_supervisors for %d", activeGroupID)
	return rows
}
