// Integration tests for the Sammel-Vertretung save (#2284):
//
//	POST /api/timetable/substitutions/bulk
//
// The point of the endpoint is that a multi-day absence/substitution lands in
// ONE atomic save. The tests drive the wired handler with real DB writes
// (buildDevSetup delegates the deviation writes to the real service), then
// read the DB back to prove:
//   - a substitute covers every selected day (and only those),
//   - the absence-only variant marks the rows without creating cover,
//   - a per-day validation failure writes NOTHING on the other days,
//   - past dates are rejected.
package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bulkRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/substitutions/bulk", res.applyBulkSubstitution)
	return r
}

func doBulk(t *testing.T, router chi.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/substitutions/bulk", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// bulkStaffState summarizes one instance's staff rows for assertions.
func bulkStaffState(t *testing.T, s *devSetup, instanceID int64) (absentA bool, subRowY *scheduleModel.InstanceStaff, extraIDs []int64) {
	t.Helper()
	for _, r := range devInstanceStaff(t, s.db, s.ctx, instanceID) {
		if r.StaffID == s.staffA && !r.IsSubstitute {
			absentA = r.IsAbsent
		}
		if r.StaffID == s.staffY && r.IsSubstitute {
			subRowY = r
			extraIDs = append(extraIDs, r.ID)
		}
	}
	return absentA, subRowY, extraIDs
}

// One save must cover every selected day with the substitute — and leave a
// same-week day that was NOT selected untouched.
func TestBulkSubstitution_MultiDayWithSubstitute(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := bulkRouter(s.ctx, s.res)
	d1Str, d1 := futureSubDate(1)
	d2Str, d2 := futureSubDate(2)
	_, d3 := futureSubDate(3)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "Tag1"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, d2, s.roomID, testpkg.ActivityInstanceOpts{Title: "Tag2"})
	inst3 := testpkg.CreateTestActivityInstance(t, s.db, d3, s.roomID, testpkg.ActivityInstanceOpts{Title: "Tag3-unselected"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst3.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffY,
		"dates":               []string{d1Str, d2Str},
		"reason":              "krank",
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	for _, instID := range []int64{inst1.ID, inst2.ID} {
		absentA, subY, _ := bulkStaffState(t, s, instID)
		assert.True(t, absentA, "A must be absent on instance %d", instID)
		require.NotNil(t, subY, "Y substitute row must exist on instance %d", instID)
		assert.False(t, subY.IsAbsent)
	}

	// The unselected third day stays untouched.
	absentA3, subY3, _ := bulkStaffState(t, s, inst3.ID)
	assert.False(t, absentA3, "unselected day must stay untouched")
	assert.Nil(t, subY3, "no substitute row on the unselected day")

	var resp struct {
		Data BulkSubstitutionResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Data.TotalAffected)
	require.Len(t, resp.Data.Days, 2)
	assert.Equal(t, d1Str, resp.Data.Days[0].Date)
	assert.Equal(t, d2Str, resp.Data.Days[1].Date)
}

// Without a substitute the save is a multi-day absence-only marking: rows are
// flagged absent, no cover rows appear.
func TestBulkSubstitution_AbsenceOnly(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := bulkRouter(s.ctx, s.res)
	d1Str, d1 := futureSubDate(1)
	d2Str, d2 := futureSubDate(2)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "Krank1"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, d2, s.roomID, testpkg.ActivityInstanceOpts{Title: "Krank2"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doBulk(t, router, map[string]any{
		"absent_staff_id": s.staffA,
		"dates":           []string{d1Str, d2Str},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	for _, instID := range []int64{inst1.ID, inst2.ID} {
		rows := devInstanceStaff(t, s.db, s.ctx, instID)
		require.Len(t, rows, 1, "no substitute row may be created")
		assert.True(t, rows[0].IsAbsent, "A must be absent on instance %d", instID)
	}
}

// All-or-nothing: when the substitute is themselves absent on the SECOND
// selected day, the whole save is rejected and the first day keeps its
// original state (Phase A runs for every day before any write).
func TestBulkSubstitution_AtomicRejectAcrossDays(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := bulkRouter(s.ctx, s.res)
	d1Str, d1 := futureSubDate(1)
	d2Str, d2 := futureSubDate(2)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "Ok-Tag"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, d2, s.roomID, testpkg.ActivityInstanceOpts{Title: "Konflikt-Tag"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffA, testpkg.InstanceStaffOpts{})
	// Y is planned on the second day and already marked absent there — Y cannot
	// cover that day, so the whole save must be rejected.
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffY, testpkg.InstanceStaffOpts{IsAbsent: true})

	w := doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffY,
		"dates":               []string{d1Str, d2Str},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), d2.Format("02.01.2006"), "error must name the failing day")

	// Day 1 untouched: A still present, no Y row.
	absentA1, subY1, _ := bulkStaffState(t, s, inst1.ID)
	assert.False(t, absentA1, "day 1 must stay untouched after the atomic reject")
	assert.Nil(t, subY1, "no substitute row on day 1 after the atomic reject")
}

// Past dates are historical record — the request is rejected before any lock
// or write, mirroring the single-day past-block guard.
func TestBulkSubstitution_RejectsPastAndInvalidInput(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := bulkRouter(s.ctx, s.res)
	past := futureSubDateOffset(-1)

	w := doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffY,
		"dates":               []string{past},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// Empty dates.
	w = doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffY,
		"dates":               []string{},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// Self-substitution.
	d1Str, _ := futureSubDate(1)
	w = doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffA,
		"dates":               []string{d1Str},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// Malformed date.
	w = doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": s.staffY,
		"dates":               []string{"31.08.2026"},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// Unknown substitute staff id → 404.
	w = doBulk(t, router, map[string]any{
		"absent_staff_id":     s.staffA,
		"substitute_staff_id": int64(999999999),
		"dates":               []string{d1Str},
	})
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// futureSubDateOffset mirrors futureSubDate for negative offsets.
func futureSubDateOffset(offsetDays int) string {
	d, _ := futureSubDate(offsetDays)
	return d
}
