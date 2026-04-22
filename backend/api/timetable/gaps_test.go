// Integration tests for the WP-B12 gaps endpoint. Mounts the handler without
// the JWT/TenantTx middleware stack; tenant context and admin permissions are
// injected directly — the same pattern used by student_day_test.go.
package timetable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type gapsSetup struct {
	res       *Resource
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	cleanupFn func()
}

func buildGapsSetup(t *testing.T) *gapsSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Gap-Room-%d", suffix))

	cleanup := func() {
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)
	}

	res := NewResource(Dependencies{
		ActivityInstanceRepo: scheduleRepo.NewActivityInstanceRepository(db),
		InstanceStaffRepo:    scheduleRepo.NewInstanceStaffRepository(db),
		SupervisorRepo:       activeRepo.NewGroupSupervisorRepository(db),
		StaffRepo:            usersRepo.NewStaffRepository(db),
		DB:                   db,
	})

	return &gapsSetup{res: res, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func gapsRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/gaps", res.getGaps)
	return r
}

func doGaps(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeGaps(t *testing.T, w *httptest.ResponseRecorder) GapsResponse {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out GapsResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

// futureDate produces a Berlin-local YYYY-MM-DD always in the future so the
// "past dates rejected" check does not flake. offsetDays=0 returns today.
func futureDate(offsetDays int) (string, time.Time) {
	d := time.Now().AddDate(0, 0, offsetDays)
	return d.Format("2006-01-02"), time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func TestGaps_Empty(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()
	router := gapsRouter(s.ctx, s.res)

	dateStr, _ := futureDate(1)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeGaps(t, w)
	assert.Equal(t, dateStr, got.From)
	assert.Equal(t, dateStr, got.To)
	assert.Empty(t, got.Gaps)
}

func TestGaps_OneGapPlannedNoStaff(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gap-1",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 1)
	assert.Equal(t, inst.ID, got.Gaps[0].InstanceID)
	assert.Equal(t, "14:00", got.Gaps[0].StartTime)
	assert.Equal(t, 0, got.Gaps[0].AssignedStaffCount)
	assert.Equal(t, 0, got.Gaps[0].AbsentStaffCount)
}

func TestGaps_AllStaffAbsent_IsAGap(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gap-All-Absent",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "GapA", fmt.Sprintf("One-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "GapA", fmt.Sprintf("Two-%d", suffix))

	row1 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	row2 := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, row1.ID, row2.ID)
		testpkg.CleanupActivityFixtures(t, s.db, 0, staff1.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, s.db, 0, staff2.ID, 0, 0, 0)
	})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 1)
	assert.Equal(t, 2, got.Gaps[0].AbsentStaffCount)
	assert.Equal(t, 0, got.Gaps[0].AssignedStaffCount)
}

func TestGaps_NonAbsentStaff_NotAGap(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Not-A-Gap",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	staff := testpkg.CreateTestStaff(t, s.db, "NotGap", fmt.Sprintf("%d", time.Now().UnixNano()))
	row := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, row.ID)
		testpkg.CleanupActivityFixtures(t, s.db, 0, staff.ID, 0, 0, 0)
	})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps)
}

func TestGaps_CompletedAndCancelled_Excluded(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	completed := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: schedule.InstanceStatusCompleted, StartHHMM: "14:00", EndHHMM: "15:00",
	})
	cancelled := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: schedule.InstanceStatusCancelled, StartHHMM: "15:00", EndHHMM: "16:00",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", completed.ID, cancelled.ID)
	})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps)
}

func TestGaps_DateValidation(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()
	router := gapsRouter(s.ctx, s.res)

	t.Run("missing date", func(t *testing.T) {
		w := doGaps(t, router, "/gaps")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("past date rejected", func(t *testing.T) {
		past, _ := futureDate(-1)
		w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", past))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("range too large", func(t *testing.T) {
		from, _ := futureDate(1)
		to, _ := futureDate(20)
		w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s&date_to=%s", from, to))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("date_to before date", func(t *testing.T) {
		from, _ := futureDate(2)
		to, _ := futureDate(1)
		w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s&date_to=%s", from, to))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("malformed date", func(t *testing.T) {
		w := doGaps(t, router, "/gaps?date=not-a-date")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGaps_Sorting(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	day1Str, day1 := futureDate(1)
	day2Str, day2 := futureDate(2)

	// Day 2, 14:00 should sort AFTER Day 1's two instances.
	d1Late := testpkg.CreateTestActivityInstance(t, s.db, day1, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "D1-Late",
	})
	d1Early := testpkg.CreateTestActivityInstance(t, s.db, day1, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "D1-Early",
	})
	d2 := testpkg.CreateTestActivityInstance(t, s.db, day2, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "D2",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", d1Late.ID, d1Early.ID, d2.ID)
	})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s&date_to=%s", day1Str, day2Str))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 3)
	assert.Equal(t, "D1-Early", got.Gaps[0].Title)
	assert.Equal(t, "D1-Late", got.Gaps[1].Title)
	assert.Equal(t, "D2", got.Gaps[2].Title)
}

func TestGaps_TenantIsolation(t *testing.T) {
	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	otherTenantCtx := testpkg.TenantContext(999)
	router := gapsRouter(otherTenantCtx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps, "tenant isolation: tenant 999 must not see tenant 1 instance")
}
