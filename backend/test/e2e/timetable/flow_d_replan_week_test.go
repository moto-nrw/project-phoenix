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

// TestFlowD_ReplanWeekMergeStrategy covers the "Admin aendert Template
// nachtraeglich" workflow:
//   - 4 materialized instances plus 1 spontaneous instance on the same day.
//   - One starts (active), one starts+completes, one gets cancelled, one
//     stays planned. Spontaneous instance is planned + is_spontaneous=true.
//   - POST /instances/re-plan-week deletes only planned + non-spontaneous
//     rows and re-materializes from templates. Active/completed/cancelled/
//     spontaneous instances survive untouched.
func TestFlowD_ReplanWeekMergeStrategy(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	target := nextWeekday(timezone.TodayDate(), 4, 7) // Thursday, >=7 days out
	fromS := target.String()

	s.createActivePeriod(fmt.Sprintf("E2E-Flow-D-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowD-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", room.ID)
	})

	staff := testpkg.CreateTestStaff(t, s.db, "FlowD", "Staff")
	student := testpkg.CreateTestStudent(t, s.db, "Dora", "FlowD", "3a")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupStaffFixtures(t, s.db, staff.ID)
		testpkg.CleanupTableRecords(t, s.db, "users.students", student.ID)
	})

	// Four templates at distinct times on Thursday.
	tmpls := make([]*templateFixture, 4)
	for i, slot := range []struct {
		name  string
		start string
		end   string
	}{
		{"D-Active", "08:00", "09:00"},
		{"D-Completed", "10:00", "11:00"},
		{"D-Cancelled", "12:00", "13:00"},
		{"D-Planned", "14:00", "15:00"},
	} {
		tmpls[i] = s.buildTemplate(templateSpec{
			name:       slot.name,
			weekday:    4,
			startHHMM:  slot.start,
			endHHMM:    slot.end,
			roomID:     room.ID,
			staffIDs:   []int64{staff.ID},
			studentIDs: []int64{student.ID},
		})
	}

	// --- Step 1: materialize the four templates ----------------------------
	matReq := map[string]any{"from_date": fromS, "to_date": fromS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())
	var matResp struct {
		InstancesCreated int `json:"instances_created"`
	}
	decodeResponse(t, rr, &matResp)
	require.Equal(t, 4, matResp.InstancesCreated)

	instActive := fetchOneInstance(t, s, tmpls[0].group.ID, target)
	instCompleted := fetchOneInstance(t, s, tmpls[1].group.ID, target)
	instCancelled := fetchOneInstance(t, s, tmpls[2].group.ID, target)
	instPlanned := fetchOneInstance(t, s, tmpls[3].group.ID, target)
	s.registerCleanup("schedule.activity_instances",
		instActive.ID, instCompleted.ID, instCancelled.ID, instPlanned.ID)

	// --- Step 2: drive the three lifecycle transitions ---------------------
	rr = s.do("POST", fmt.Sprintf("/instances/%d/start", instActive.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "start active body=%s", rr.Body.String())

	rr = s.do("POST", fmt.Sprintf("/instances/%d/start", instCompleted.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "start completed body=%s", rr.Body.String())
	rr = s.do("POST", fmt.Sprintf("/instances/%d/complete", instCompleted.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "complete body=%s", rr.Body.String())

	rr = s.do("POST", fmt.Sprintf("/instances/%d/cancel", instCancelled.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "cancel body=%s", rr.Body.String())

	// --- Step 3: add a spontaneous instance (planned, is_spontaneous=true) -
	startFixture := parseHHMMLocal(t, "16:00")
	endFixture := parseHHMMLocal(t, "17:00")
	spont := &scheduleModel.ActivityInstance{
		Date:          target,
		Title:         "D-Spontaneous",
		StartTime:     startFixture,
		EndTime:       endFixture,
		RoomID:        room.ID,
		Status:        scheduleModel.InstanceStatusPlanned,
		IsSpontaneous: true,
	}
	spont.SetTenantID(s.primaryTenant)
	_, err := s.db.NewInsert().Model(spont).
		ModelTableExpr(`schedule.activity_instances`).Exec(s.tenantCtx())
	require.NoError(t, err, "insert spontaneous instance")
	s.registerCleanup("schedule.activity_instances", spont.ID)

	// Capture snapshots of the 4 preserved instances for post-replan compare.
	beforeActive := reload(t, s, instActive.ID)
	beforeCompleted := reload(t, s, instCompleted.ID)
	beforeCancelled := reload(t, s, instCancelled.ID)
	beforeSpont := reload(t, s, spont.ID)

	require.Equal(t, scheduleModel.InstanceStatusActive, beforeActive.Status)
	require.Equal(t, scheduleModel.InstanceStatusCompleted, beforeCompleted.Status)
	require.Equal(t, scheduleModel.InstanceStatusCancelled, beforeCancelled.Status)
	require.Equal(t, scheduleModel.InstanceStatusPlanned, beforeSpont.Status)
	require.True(t, beforeSpont.IsSpontaneous)

	// --- Step 4: re-plan the week -----------------------------------------
	rr = s.do("POST", "/instances/re-plan-week", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "re-plan body=%s", rr.Body.String())

	var replanResp struct {
		DeletedInstances int `json:"deleted_instances"`
		InstancesCreated int `json:"instances_created"`
	}
	decodeResponse(t, rr, &replanResp)
	assert.Equal(t, 1, replanResp.DeletedInstances,
		"only the planned non-spontaneous instance should be deleted")
	assert.Equal(t, 1, replanResp.InstancesCreated,
		"only one template is re-materialized (the one whose instance was deleted)")

	// --- Step 5: protected instances must be unchanged ---------------------
	afterActive := reload(t, s, instActive.ID)
	afterCompleted := reload(t, s, instCompleted.ID)
	afterCancelled := reload(t, s, instCancelled.ID)
	afterSpont := reload(t, s, spont.ID)

	assert.Equal(t, beforeActive.Status, afterActive.Status)
	assert.Equal(t, beforeCompleted.Status, afterCompleted.Status)
	assert.Equal(t, beforeCancelled.Status, afterCancelled.Status)
	assert.Equal(t, beforeSpont.Status, afterSpont.Status)
	assert.True(t, afterSpont.IsSpontaneous, "spontaneous instance survives untouched")

	// The originally-planned instance (tmpls[3]) was deleted and recreated:
	// its ID is new even though the template is unchanged.
	newPlanned := fetchOneInstance(t, s, tmpls[3].group.ID, target)
	assert.NotEqual(t, instPlanned.ID, newPlanned.ID,
		"deleted-and-recreated gets a fresh ID")
	assert.Equal(t, scheduleModel.InstanceStatusPlanned, newPlanned.Status)
	s.registerCleanup("schedule.activity_instances", newPlanned.ID)

	// --- Step 6: tenant isolation -----------------------------------------
	// Secondary tenant re-plans their (empty) week. No primary data sees it,
	// so deleted_instances stays zero.
	rr = s.do("POST", "/instances/re-plan-week", matReq, s.secondaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "tenant2 body=%s", rr.Body.String())
	var t2Resp struct {
		DeletedInstances int `json:"deleted_instances"`
		InstancesCreated int `json:"instances_created"`
	}
	decodeResponse(t, rr, &t2Resp)
	assert.Equal(t, 0, t2Resp.DeletedInstances, "secondary tenant sees no primary instances")
	assert.Equal(t, 0, t2Resp.InstancesCreated, "secondary tenant has no templates")
}

// reload fetches an activity_instance by ID, skipping a not-found error if
// the row has been deleted (signalled by an empty struct return).
func reload(t *testing.T, s *scenario, id int64) scheduleModel.ActivityInstance {
	t.Helper()
	var inst scheduleModel.ActivityInstance
	err := s.db.NewSelect().Model(&inst).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, id).
		Where(`"activity_instance".tenant_id = ?`, s.primaryTenant).
		Scan(s.tenantCtx())
	require.NoError(t, err, "reload instance %d", id)
	return inst
}
