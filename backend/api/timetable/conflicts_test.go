// Integration tests for the planned-conflict probe endpoint. Mounts the
// handler without the JWT/TenantTx middleware stack; tenant context is
// injected directly — the same pattern used by gaps_test.go.
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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type plannedConflictsSetup struct {
	res    *Resource
	db     *bun.DB
	ctx    context.Context
	roomID int64
	date   timezone.Date
}

func buildPlannedConflictsSetup(t *testing.T) *plannedConflictsSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Conf-Room-%d", suffix))

	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		DB:            db,
	})

	return &plannedConflictsSetup{
		res:    res,
		db:     db,
		ctx:    testpkg.Ctx(t),
		roomID: room.ID,
		date:   timezone.NewDate(2026, time.June, 22),
	}
}

func plannedConflictsRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/conflicts", res.getPlannedConflicts)
	return r
}

func doPlannedConflicts(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodePlannedConflicts(t *testing.T, w *httptest.ResponseRecorder) PlannedConflictsResponse {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out PlannedConflictsResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

func TestConflicts_BadRequestMatrix(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	base := fmt.Sprintf("date=%s&start_time=14:00&end_time=15:00&room_id=%d", s.date.String(), s.roomID)
	cases := []struct {
		name string
		path string
	}{
		{"missing date", fmt.Sprintf("/conflicts?start_time=14:00&end_time=15:00&room_id=%d", s.roomID)},
		{"malformed date", fmt.Sprintf("/conflicts?date=not-a-date&start_time=14:00&end_time=15:00&room_id=%d", s.roomID)},
		{"missing start_time", fmt.Sprintf("/conflicts?date=%s&end_time=15:00&room_id=%d", s.date.String(), s.roomID)},
		{"malformed start_time", fmt.Sprintf("/conflicts?date=%s&start_time=xx&end_time=15:00&room_id=%d", s.date.String(), s.roomID)},
		{"missing end_time", fmt.Sprintf("/conflicts?date=%s&start_time=14:00&room_id=%d", s.date.String(), s.roomID)},
		{"malformed end_time", fmt.Sprintf("/conflicts?date=%s&start_time=14:00&end_time=xx&room_id=%d", s.date.String(), s.roomID)},
		{"end before start", fmt.Sprintf("/conflicts?date=%s&start_time=15:00&end_time=14:00&room_id=%d", s.date.String(), s.roomID)},
		{"no resource param", fmt.Sprintf("/conflicts?date=%s&start_time=14:00&end_time=15:00", s.date.String())},
		{"malformed room_id", fmt.Sprintf("/conflicts?date=%s&start_time=14:00&end_time=15:00&room_id=abc", s.date.String())},
		{"malformed staff_ids", "/conflicts?" + base + "&staff_ids=1,abc"},
		{"malformed student_ids", "/conflicts?" + base + "&student_ids=0"},
		{"malformed exclude_instance_id", "/conflicts?" + base + "&exclude_instance_id=abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doPlannedConflicts(t, router, tc.path)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestConflicts_EmptyWarnings(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:00&end_time=15:00&room_id=%d", s.date.String(), s.roomID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	assert.Equal(t, s.date.String(), got.Date)
	assert.Equal(t, "14:00", got.StartTime)
	assert.Equal(t, "15:00", got.EndTime)
	assert.NotNil(t, got.Warnings)
	assert.Empty(t, got.Warnings)
}

func TestConflicts_RoomOverlapAloneYieldsNoWarning(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	// #2139: several groups may share a room — a probe that only shares the
	// room with an overlapping instance answers 200 with zero warnings.
	testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conf-Raum"})

	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:30&end_time=15:30&room_id=%d", s.date.String(), s.roomID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	assert.Empty(t, got.Warnings, "a pure room overlap must not warn")
}

func TestConflicts_StaffSameRoomYieldsNoWarning(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	inst := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conf-Parallel"})
	staff := testpkg.CreateTestStaff(t, s.db, "Conf", fmt.Sprintf("SameRoom-%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{})

	// Same staff, overlapping window, SAME room — sanctioned (#2139).
	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:30&end_time=15:30&room_id=%d&staff_ids=%d",
		s.date.String(), s.roomID, staff.ID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	assert.Empty(t, got.Warnings, "same-room staff overlap must not warn")
}

func TestConflicts_StaffWarning(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	inst := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conf-Personal"})
	staff := testpkg.CreateTestStaff(t, s.db, "Conf", fmt.Sprintf("Staff-%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{})

	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:30&end_time=15:30&staff_ids=%d", s.date.String(), staff.ID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	require.Len(t, got.Warnings, 1)
	assert.Equal(t, scheduleSvc.ConflictKindStaff, got.Warnings[0].Kind)
	assert.Equal(t, staff.ID, got.Warnings[0].ResourceID)
	assert.Equal(t, inst.ID, got.Warnings[0].ConflictingInstanceID)
}

func TestConflicts_StudentWarning(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	inst := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conf-Kind"})
	student := testpkg.CreateTestStudent(t, s.db, "Conf-Kid", fmt.Sprintf("One-%d", time.Now().UnixNano()), "2b")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student.ID, "")

	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:30&end_time=15:30&student_ids=%d", s.date.String(), student.ID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	require.Len(t, got.Warnings, 1)
	assert.Equal(t, scheduleSvc.ConflictKindStudent, got.Warnings[0].Kind)
	assert.Equal(t, student.ID, got.Warnings[0].ResourceID)
	assert.Equal(t, inst.ID, got.Warnings[0].ConflictingInstanceID)
}

func TestConflicts_ExcludeInstance(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := plannedConflictsRouter(s.ctx, s.res)

	inst := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conf-Selbst"})

	w := doPlannedConflicts(t, router, fmt.Sprintf(
		"/conflicts?date=%s&start_time=14:30&end_time=15:30&room_id=%d&exclude_instance_id=%d",
		s.date.String(), s.roomID, inst.ID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodePlannedConflicts(t, w)
	assert.Empty(t, got.Warnings)
}
