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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Gap-Room-%d", suffix))

	cleanup := func() {
	}

	res := NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		DB:            db,
	})

	return &gapsSetup{res: res, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func gapsRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
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
func futureDate(offsetDays int) (string, timezone.Date) {
	d := timezone.TodayDate().AddDays(offsetDays)
	return d.String(), d
}

func TestGaps_Empty(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gap-1",
	})

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
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gap-All-Absent",
	})

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, s.db, "GapA", fmt.Sprintf("One-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, s.db, "GapA", fmt.Sprintf("Two-%d", suffix))

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff1.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff2.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 1)
	assert.Equal(t, 2, got.Gaps[0].AbsentStaffCount)
	assert.Equal(t, 2, got.Gaps[0].AssignedStaffCount)
}

// #1840: a block planned for two people with one absent and no replacement is a
// partial shortfall — a position deliberately (or not) left unfilled. It must
// surface as a gap even though one person is still present, and report present
// vs planned so the UI can show "1 von 2".
func TestGaps_PartiallyStaffed_IsAGap(t *testing.T) {
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gap-Partial",
	})

	suffix := time.Now().UnixNano()
	present := testpkg.CreateTestStaff(t, s.db, "GapP", fmt.Sprintf("Present-%d", suffix))
	absent := testpkg.CreateTestStaff(t, s.db, "GapP", fmt.Sprintf("Absent-%d", suffix))

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, present.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, absent.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 1)
	assert.Equal(t, inst.ID, got.Gaps[0].InstanceID)
	assert.Equal(t, 2, got.Gaps[0].AssignedStaffCount)
	assert.Equal(t, 1, got.Gaps[0].AbsentStaffCount)
	assert.Equal(t, 1, got.Gaps[0].PresentStaffCount)
	assert.Equal(t, 2, got.Gaps[0].PlannedStaffCount)
}

// A block covered by a substitute (one planned absent, one substitute present)
// is fully staffed again — it must NOT be a gap.
func TestGaps_SubstituteCovers_NotAGap(t *testing.T) {
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Covered",
	})

	suffix := time.Now().UnixNano()
	planned := testpkg.CreateTestStaff(t, s.db, "GapC", fmt.Sprintf("Planned-%d", suffix))
	sub := testpkg.CreateTestStaff(t, s.db, "GapC", fmt.Sprintf("Sub-%d", suffix))

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, planned.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, sub.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps)
}

func TestGaps_NonAbsentStaff_NotAGap(t *testing.T) {
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Not-A-Gap",
	})

	staff := testpkg.CreateTestStaff(t, s.db, "NotGap", fmt.Sprintf("%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps)
}

func TestGaps_CompletedAndCancelled_Excluded(t *testing.T) {
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: schedule.InstanceStatusCompleted, StartHHMM: "14:00", EndHHMM: "15:00",
	})
	testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Status: schedule.InstanceStatusCancelled, StartHHMM: "15:00", EndHHMM: "16:00",
	})

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps)
}

// #1840: an instance with zero staff but understaffed_ack=true is partitioned
// into Acknowledged, not Gaps — the shortfall stays visible but stops nagging.
func TestGaps_UnderstaffedAck_MovedToAcknowledged(t *testing.T) {
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Open-Gap",
	})
	ack := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Acknowledged-Gap",
	})

	// Mark the second instance as deliberately unstaffed.
	_, err := s.db.NewUpdate().
		Model((*schedule.ActivityInstance)(nil)).
		Set("understaffed_ack = TRUE").
		Set("understaffed_note = ?", "bewusst ohne Personal").
		Where("id = ?", ack.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	router := gapsRouter(s.ctx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeGaps(t, w)
	require.Len(t, got.Gaps, 1, "only the open gap stays in gaps")
	assert.Equal(t, "Open-Gap", got.Gaps[0].Title)

	require.Len(t, got.Acknowledged, 1, "the acknowledged block moves to its own bucket")
	assert.Equal(t, ack.ID, got.Acknowledged[0].InstanceID)
	require.NotNil(t, got.Acknowledged[0].UnderstaffedNote)
	assert.Equal(t, "bewusst ohne Personal", *got.Acknowledged[0].UnderstaffedNote)
}

func TestGaps_DateValidation(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	day1Str, day1 := futureDate(1)
	day2Str, day2 := futureDate(2)

	// Day 2, 14:00 should sort AFTER Day 1's two instances.
	testpkg.CreateTestActivityInstance(t, s.db, day1, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "D1-Late",
	})
	testpkg.CreateTestActivityInstance(t, s.db, day1, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "D1-Early",
	})
	testpkg.CreateTestActivityInstance(t, s.db, day2, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "D2",
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
	t.Parallel()

	s := buildGapsSetup(t)
	defer s.cleanupFn()

	dateStr, date := futureDate(1)
	testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00",
	})

	otherTenantCtx := testpkg.TenantContext(999)
	router := gapsRouter(otherTenantCtx, s.res)
	w := doGaps(t, router, fmt.Sprintf("/gaps?date=%s", dateStr))
	require.Equal(t, http.StatusOK, w.Code)
	got := decodeGaps(t, w)
	assert.Empty(t, got.Gaps, "tenant isolation: tenant 999 must not see tenant 1 instance")
}
