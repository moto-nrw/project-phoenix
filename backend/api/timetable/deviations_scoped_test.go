package timetable_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apiTest "github.com/moto-nrw/project-phoenix/api/testutil"
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type scopedDevSetup struct {
	db       *bun.DB
	ctx      context.Context
	router   chi.Router
	claimsID int
	roomID   int64
	staffA   int64
	staffX   int64
	staffY   int64
	staffB   int64
}

type scopedApplyResponse struct {
	Data struct {
		AffectedInstances []struct {
			InstanceID int64  `json:"instance_id"`
			Action     string `json:"action"`
		} `json:"affected_instances"`
	} `json:"data"`
}

func setupScopedDeviationsRoute(t *testing.T) *scopedDevSetup {
	t.Helper()
	db, serviceFactory := apiTest.SetupAPITest(t)
	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Scoped-Dev-Room-%d", suffix))
	a := testpkg.CreateTestStaff(t, db, "Planned", fmt.Sprintf("%d", suffix))
	x := testpkg.CreateTestStaff(t, db, "CurrentSub", fmt.Sprintf("%d", suffix+1))
	y := testpkg.CreateTestStaff(t, db, "Replacement", fmt.Sprintf("%d", suffix+2))
	b := testpkg.CreateTestStaff(t, db, "Planned2", fmt.Sprintf("%d", suffix+3))
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("scoped-dev-%d", suffix))
	resource := timetableAPI.NewResource(timetableAPI.Dependencies{
		InstanceService: serviceFactory.Instance,
		DB:              db,
	})
	router := chi.NewRouter()
	router.Mount("/timetable", resource.Router())
	return &scopedDevSetup{
		db: db, ctx: ctx, router: router, claimsID: int(account.ID), roomID: room.ID,
		staffA: a.ID, staffX: x.ID, staffY: y.ID, staffB: b.ID,
	}
}

func doScopedDev(t *testing.T, setup *scopedDevSetup, instanceID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/timetable/instances/%d/deviations", instanceID), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return apiTest.ExecuteWithAuthPermissions(t, setup.router, req, apiTest.AdminTestClaims(setup.claimsID), []string{permissions.SchedulesManage})
}

func scopedInstanceStaff(t *testing.T, db *bun.DB, ctx context.Context, instanceID int64) []*scheduleModel.InstanceStaff {
	t.Helper()
	rows, err := scheduleRepo.NewInstanceStaffRepository(db).FindByInstanceID(ctx, instanceID)
	require.NoError(t, err)
	return rows
}

func readScopedInstanceStaff(t *testing.T, db *bun.DB, ctx context.Context, id int64) *scheduleModel.InstanceStaff {
	t.Helper()
	row, err := scheduleRepo.NewInstanceStaffRepository(db).FindByID(ctx, id)
	require.NoError(t, err)
	return row
}

func TestApplyDeviations_SubstitutionTargetsSelectedInstances(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"substitutions": []map[string]any{{
			"absent_staff_id":     s.staffA,
			"substitute_staff_id": s.staffY,
			"instance_ids":        []int64{selected.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	selectedRows := scopedInstanceStaff(t, s.db, s.ctx, selected.ID)
	assertStaffState(t, selectedRows, s.staffA, true, false)
	assertStaffState(t, selectedRows, s.staffY, false, true)

	otherRows := scopedInstanceStaff(t, s.db, s.ctx, other.ID)
	assertStaffState(t, otherRows, s.staffA, false, false)
	assertStaffMissing(t, otherRows, s.staffY)

	var response scopedApplyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.AffectedInstances, 1)
	assert.Equal(t, selected.ID, response.Data.AffectedInstances[0].InstanceID)
}

func TestApplyDeviations_AbsenceTargetsSelectedInstances(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"absences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{selected.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, selected.ID), s.staffA, true, false)
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, other.ID), s.staffA, false, false)
}

func TestApplyDeviations_SelectedCoverageWithAllDayAbsence(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	covered := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	uncovered := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, covered.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, uncovered.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, covered.ID, map[string]any{
		"absences": []map[string]any{{"staff_id": s.staffA}},
		"substitutions": []map[string]any{{
			"absent_staff_id":     s.staffA,
			"substitute_staff_id": s.staffY,
			"instance_ids":        []int64{covered.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	coveredRows := scopedInstanceStaff(t, s.db, s.ctx, covered.ID)
	assertStaffState(t, coveredRows, s.staffA, true, false)
	assertStaffState(t, coveredRows, s.staffY, false, true)

	uncoveredRows := scopedInstanceStaff(t, s.db, s.ctx, uncovered.ID)
	assertStaffState(t, uncoveredRows, s.staffA, true, false)
	assertStaffMissing(t, uncoveredRows, s.staffY)

	var response scopedApplyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.AffectedInstances, 2, "each changed appointment appears once")
	assert.ElementsMatch(t, []int64{covered.ID, uncovered.ID}, []int64{
		response.Data.AffectedInstances[0].InstanceID,
		response.Data.AffectedInstances[1].InstanceID,
	})
}

func TestApplyDeviations_PartiallyAbsentStaffCanCoverAnotherAppointment(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	target := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, target.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffY, testpkg.InstanceStaffOpts{IsAbsent: true})

	w := doScopedDev(t, s, target.ID, map[string]any{
		"substitutions": []map[string]any{{
			"absent_staff_id":     s.staffA,
			"substitute_staff_id": s.staffY,
			"instance_ids":        []int64{target.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, target.ID), s.staffY, false, true)
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, other.ID), s.staffY, true, false)
}

func TestApplyDeviations_SameSaveCanAbsenceAndSubstituteStaffOnDifferentAppointments(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	target := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, target.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffY, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, target.ID, map[string]any{
		"absences": []map[string]any{{
			"staff_id":     s.staffY,
			"instance_ids": []int64{other.ID},
		}},
		"substitutions": []map[string]any{{
			"absent_staff_id":     s.staffA,
			"substitute_staff_id": s.staffY,
			"instance_ids":        []int64{target.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, target.ID), s.staffY, false, true)
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, other.ID), s.staffY, true, false)
}

func TestApplyDeviations_PresenceTargetsSelectedInstances(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"presences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{selected.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, selected.ID), s.staffA, false, false)
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, other.ID), s.staffA, true, false)
}

func TestApplyDeviations_SameStaffCanBePresentAndAbsentOnDifferentAppointments(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	restored := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	absent := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, restored.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, absent.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, restored.ID, map[string]any{
		"presences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{restored.ID},
		}},
		"absences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{absent.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, restored.ID), s.staffA, false, false)
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, absent.ID), s.staffA, true, false)
}

func TestApplyDeviations_CannotClearSickAbsence(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	row := testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	sickAbsenceID := row.ID
	row.SickAbsenceID = &sickAbsenceID
	require.NoError(t, scheduleRepo.NewInstanceStaffRepository(s.db).Update(s.ctx, row))

	w := doScopedDev(t, s, instance.ID, map[string]any{
		"presences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{instance.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "sick_absence_scope_locked")

	stored := readScopedInstanceStaff(t, s.db, s.ctx, row.ID)
	assert.True(t, stored.IsAbsent)
	require.NotNil(t, stored.SickAbsenceID)
	assert.Equal(t, sickAbsenceID, *stored.SickAbsenceID)
}

func TestApplyDeviations_RemovesSubstituteOnlyFromSelectedInstances(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	for _, instanceID := range []int64{selected.ID, other.ID} {
		testpkg.CreateTestInstanceStaff(t, s.db, instanceID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
		testpkg.CreateTestInstanceStaff(t, s.db, instanceID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})
	}

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"substitution_removals": []map[string]any{{
			"staff_id":     s.staffX,
			"instance_ids": []int64{selected.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	selectedRows := scopedInstanceStaff(t, s.db, s.ctx, selected.ID)
	assertStaffMissing(t, selectedRows, s.staffX)
	assertStaffState(t, selectedRows, s.staffA, true, false)

	otherRows := scopedInstanceStaff(t, s.db, s.ctx, other.ID)
	assertStaffState(t, otherRows, s.staffX, false, true)

	var response scopedApplyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.AffectedInstances, 1)
	assert.Equal(t, selected.ID, response.Data.AffectedInstances[0].InstanceID)
	assert.Equal(t, "substitute_removed", response.Data.AffectedInstances[0].Action)
}

func TestApplyDeviations_CannotRemoveSickSubstitute(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	row := testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true, IsAbsent: true})
	sickAbsenceID := row.ID
	row.SickAbsenceID = &sickAbsenceID
	require.NoError(t, scheduleRepo.NewInstanceStaffRepository(s.db).Update(s.ctx, row))

	w := doScopedDev(t, s, instance.ID, map[string]any{
		"substitution_removals": []map[string]any{{
			"staff_id":     s.staffX,
			"instance_ids": []int64{instance.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "sick_absence_scope_locked")

	stored := readScopedInstanceStaff(t, s.db, s.ctx, row.ID)
	assert.True(t, stored.IsSubstitute)
	assert.True(t, stored.IsAbsent)
	require.NotNil(t, stored.SickAbsenceID)
	assert.Equal(t, sickAbsenceID, *stored.SickAbsenceID)
}

func TestApplyDeviations_RejectsSubstituteRemovalWithoutSelectedAssignment(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"substitution_removals": []map[string]any{{
			"staff_id":     s.staffX,
			"instance_ids": []int64{selected.ID},
		}},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assertStaffState(t, scopedInstanceStaff(t, s.db, s.ctx, other.ID), s.staffX, false, true)
}

func TestApplyDeviations_DeduplicatesOverlappingSubstituteRemovals(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})

	removal := map[string]any{
		"staff_id":     s.staffX,
		"instance_ids": []int64{instance.ID},
	}
	w := doScopedDev(t, s, instance.ID, map[string]any{
		"substitution_removals": []map[string]any{removal, removal},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assertStaffMissing(t, scopedInstanceStaff(t, s.db, s.ctx, instance.ID), s.staffX)

	var response scopedApplyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data.AffectedInstances, 1)
	assert.Equal(t, "substitute_removed", response.Data.AffectedInstances[0].Action)
}

func TestApplyDeviations_ReplacesSubstituteInOneScopedSave(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})

	w := doScopedDev(t, s, instance.ID, map[string]any{
		"substitution_removals": []map[string]any{{
			"staff_id": s.staffX, "instance_ids": []int64{instance.ID},
		}},
		"substitutions": []map[string]any{{
			"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY, "instance_ids": []int64{instance.ID},
		}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rows := scopedInstanceStaff(t, s.db, s.ctx, instance.ID)
	assertStaffMissing(t, rows, s.staffX)
	assertStaffState(t, rows, s.staffY, false, true)
}

func TestApplyDeviations_PartialAbsenceElsewhereDoesNotMakeSelectedBlockUnderstaffed(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)

	target := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Randstunde", StartHHMM: "08:00", EndHHMM: "09:00",
	})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Lernzeit", StartHHMM: "11:00", EndHHMM: "12:00",
	})
	rowA := testpkg.CreateTestInstanceStaff(t, s.db, target.ID, s.staffA, testpkg.InstanceStaffOpts{})
	rowBTarget := testpkg.CreateTestInstanceStaff(t, s.db, target.ID, s.staffB, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffB, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, target.ID, map[string]any{
		"understaffed_ack": true,
		"absences": []map[string]any{{
			"staff_id":     s.staffB,
			"instance_ids": []int64{other.ID},
		}},
		"substitutions": []map[string]any{{
			"absent_staff_id":     s.staffA,
			"substitute_staff_id": s.staffY,
			"instance_ids":        []int64{target.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "understaffed_still_staffed")

	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, rowA.ID).IsAbsent)
	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, rowBTarget.ID).IsAbsent)
	assertStaffMissing(t, scopedInstanceStaff(t, s.db, s.ctx, target.ID), s.staffY)
}

func TestApplyDeviations_RejectsEmptyAppointmentScope(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	row := testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, instance.ID, map[string]any{
		"absences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{},
		}},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, row.ID).IsAbsent)
}

func TestApplyDeviations_RejectsTerminalAppointmentInExplicitScope(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	terminal := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: scheduleModel.InstanceStatusCancelled,
	})
	selectedRow := testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, terminal.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"absences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{selected.ID, terminal.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "instance_not_editable")
	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, selectedRow.ID).IsAbsent)
}

func TestApplyDeviations_RejectsAlreadyAbsentTerminalAppointmentInExplicitScope(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	terminal := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: scheduleModel.InstanceStatusCompleted,
	})
	selectedRow := testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, terminal.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"absences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{selected.ID, terminal.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "instance_not_editable")
	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, selectedRow.ID).IsAbsent)
}

func TestApplyDeviations_RejectsAlreadyPresentTerminalAppointmentInExplicitScope(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	terminal := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: scheduleModel.InstanceStatusCancelled,
	})
	selectedRow := testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, terminal.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doScopedDev(t, s, selected.ID, map[string]any{
		"presences": []map[string]any{{
			"staff_id":     s.staffA,
			"instance_ids": []int64{selected.ID, terminal.ID},
		}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "instance_not_editable")
	assert.True(t, readScopedInstanceStaff(t, s.db, s.ctx, selectedRow.ID).IsAbsent)
}

func TestApplyDeviations_DayWidePresenceSkipsTerminalSickAbsence(t *testing.T) {
	t.Parallel()

	s := setupScopedDeviationsRoute(t)
	date := timezone.TodayDate().AddDays(1)
	planned := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{})
	terminal := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: scheduleModel.InstanceStatusCompleted,
	})
	plannedRow := testpkg.CreateTestInstanceStaff(t, s.db, planned.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	terminalRow := testpkg.CreateTestInstanceStaff(t, s.db, terminal.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	sickAbsenceID := terminalRow.ID
	terminalRow.SickAbsenceID = &sickAbsenceID
	require.NoError(t, scheduleRepo.NewInstanceStaffRepository(s.db).Update(s.ctx, terminalRow))

	w := doScopedDev(t, s, planned.ID, map[string]any{
		"presences": []map[string]any{{"staff_id": s.staffA}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.False(t, readScopedInstanceStaff(t, s.db, s.ctx, plannedRow.ID).IsAbsent)
	assert.True(t, readScopedInstanceStaff(t, s.db, s.ctx, terminalRow.ID).IsAbsent)
}

func assertStaffState(t *testing.T, rows []*scheduleModel.InstanceStaff, staffID int64, absent, substitute bool) {
	t.Helper()
	for _, row := range rows {
		if row.StaffID == staffID {
			assert.Equal(t, absent, row.IsAbsent)
			assert.Equal(t, substitute, row.IsSubstitute)
			return
		}
	}
	t.Fatalf("staff %d missing", staffID)
}

func assertStaffMissing(t *testing.T, rows []*scheduleModel.InstanceStaff, staffID int64) {
	t.Helper()
	for _, row := range rows {
		if row.StaffID == staffID {
			t.Fatalf("staff %d unexpectedly present", staffID)
		}
	}
}
