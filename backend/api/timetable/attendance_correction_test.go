package timetable

// Attendance correction of a completed block (#2898).
//
// The point of these tests is the integrity contract, not just the happy path:
// the ordinary PATCH must STAY frozen, a correction must be impossible without
// a reason, and every accepted correction must leave a trail carrying the
// before/after values.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// correctionRouter mounts the correction routes next to the PATCH route, so a
// single test can assert that the two behave differently on the same instance.
func correctionRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Patch("/instances/{instance_id}/students/{student_id}", res.patchInstanceStudent)
	r.Post("/instances/{instance_id}/students/{student_id}/correction", res.correctInstanceStudent)
	r.Get("/instances/{instance_id}/students/{student_id}/corrections", res.getInstanceStudentCorrections)
	return r
}

func doJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		jb, err := json.Marshal(b)
		require.NoError(t, err)
		reader = bytes.NewReader(jb)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// completeInstance moves the setup's instance into the state a correction
// applies to.
func completeInstance(t *testing.T, s *patchSetup, status string) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, err := db.NewUpdate().
		TableExpr("schedule.activity_instances").
		Set("status = ?", status).
		Where("id = ?", s.instanceID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func correctionPath(s *patchSetup) string {
	return fmt.Sprintf("/instances/%d/students/%d/correction", s.instanceID, s.studentID)
}

func loadTrail(t *testing.T, s *patchSetup) []*auditModel.AttendanceCorrection {
	t.Helper()
	rows, err := auditRepo.NewAttendanceCorrectionRepository(testpkg.SetupTestDB(t)).
		ListByInstanceAndStudent(s.ctx, s.instanceID, s.studentID)
	require.NoError(t, err)
	return rows
}

func loadRow(t *testing.T, s *patchSetup) *schedule.InstanceStudent {
	t.Helper()
	row, err := scheduleRepo.NewInstanceStudentRepository(testpkg.SetupTestDB(t)).
		FindByInstanceAndStudent(s.ctx, s.instanceID, s.studentID)
	require.NoError(t, err)
	require.NotNil(t, row)
	return row
}

func TestCorrectAttendance_CompletedInstance_WritesRowAndTrail(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"status": "absent",
		"note":   "war doch nicht da",
		"reason": "Verwechslung mit Geschwisterkind",
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	row := loadRow(t, s)
	assert.Equal(t, schedule.AttendanceStatusAbsent, row.Status)
	require.NotNil(t, row.Note)
	assert.Equal(t, "war doch nicht da", *row.Note)

	trail := loadTrail(t, s)
	require.Len(t, trail, 2, "one row per changed field")
	byField := map[string]*auditModel.AttendanceCorrection{}
	for _, entry := range trail {
		byField[entry.FieldName] = entry
		assert.Equal(t, "Verwechslung mit Geschwisterkind", entry.Reason,
			"every field row of one correction carries the same reason")
	}

	statusEntry := byField[auditModel.AttendanceFieldStatus]
	require.NotNil(t, statusEntry)
	require.NotNil(t, statusEntry.OldValue)
	assert.Equal(t, schedule.AttendanceStatusPresent, *statusEntry.OldValue, "the before value must be preserved")
	require.NotNil(t, statusEntry.NewValue)
	assert.Equal(t, schedule.AttendanceStatusAbsent, *statusEntry.NewValue)

	noteEntry := byField[auditModel.AttendanceFieldNote]
	require.NotNil(t, noteEntry)
	assert.Nil(t, noteEntry.OldValue, "the note was unset before the correction")
	require.NotNil(t, noteEntry.NewValue)
	assert.Equal(t, "war doch nicht da", *noteEntry.NewValue)
}

// The completion snapshot is the evidence of what the day meant when it was
// closed. A correction changes the live row beside it and must never rewrite
// it.
func TestCorrectAttendance_LeavesCompletionSnapshotUntouched(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	snapshot := `{"active_group_id":42,"attendance":[{"row_id":1,"status":"present"}]}`
	db := testpkg.SetupTestDB(t)
	_, err := db.NewUpdate().
		TableExpr("schedule.activity_instances").
		Set("status = ?", schedule.InstanceStatusCompleted).
		Set("completion_snapshot = ?::jsonb", snapshot).
		Where("id = ?", s.instanceID).
		Exec(s.ctx)
	require.NoError(t, err)

	router := correctionRouter(testpkg.Ctx(t), s.res)
	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"note":   "nachgetragen",
		"reason": "Nachtrag aus dem Papierprotokoll",
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var stored string
	require.NoError(t, db.NewSelect().
		TableExpr("schedule.activity_instances").
		ColumnExpr("completion_snapshot::text").
		Where("id = ?", s.instanceID).
		Scan(s.ctx, &stored))
	assert.JSONEq(t, snapshot, stored, "the completion snapshot must survive a correction unchanged")
}

func TestCorrectAttendance_RejectsMissingReason(t *testing.T) {
	t.Parallel()

	for name, reason := range map[string]any{
		"absent": nil,
		"empty":  "",
		"blank":  "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := buildPatchSetup(t)
			completeInstance(t, s, schedule.InstanceStatusCompleted)
			router := correctionRouter(testpkg.Ctx(t), s.res)

			body := map[string]any{"note": "ohne Grund"}
			if reason != nil {
				body["reason"] = reason
			}
			w := doJSON(t, router, http.MethodPost, correctionPath(s), body)
			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
			assert.Contains(t, w.Body.String(), "reason")

			assert.Nil(t, loadRow(t, s).Note, "a rejected correction must not touch the row")
			assert.Empty(t, loadTrail(t, s))
		})
	}
}

func TestCorrectAttendance_RejectsOverlongReason(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	long := make([]rune, auditModel.CorrectionReasonMaxLength+1)
	for i := range long {
		long[i] = 'ä'
	}
	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"note":   "zu langer Grund",
		"reason": string(long),
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Empty(t, loadTrail(t, s))
}

// A correction is an intervention into a CLOSED record. While the block is
// still planned or running the ordinary attendance paths apply, and no
// correction trail should be produced.
func TestCorrectAttendance_RejectsInstanceThatIsNotCompleted(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t) // setup leaves the instance planned
	router := correctionRouter(testpkg.Ctx(t), s.res)

	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"note":   "zu früh",
		"reason": "noch nicht abgeschlossen",
	})
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "completed")
	assert.Empty(t, loadTrail(t, s))
}

// A cancelled block did not take place. Correcting attendance there would
// record a child as present at something that never happened.
func TestCorrectAttendance_RejectsCancelledInstance(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCancelled)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"status": "present",
		"reason": "war doch da",
	})
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "cancelled")
	assert.Empty(t, loadTrail(t, s))
}

func TestCorrectAttendance_UnknownStudentIs404(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	other := testpkg.CreateTestStudent(t, testpkg.SetupTestDB(t), "C-Stu", "Unrelated", "4b")
	w := doJSON(t, router, http.MethodPost,
		fmt.Sprintf("/instances/%d/students/%d/correction", s.instanceID, other.ID),
		map[string]any{"note": "x", "reason": "kein Eintrag vorhanden"})
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

// A correction that requests the value a field already holds changes nothing
// and must not inflate the trail.
func TestCorrectAttendance_NoOpWritesNoTrail(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	w := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"status": schedule.AttendanceStatusPresent, // already the current value
		"reason": "versehentlich abgeschickt",
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Empty(t, loadTrail(t, s), "an unchanged value must not produce a trail entry")
}

// THE regression guard for the preserved invariant: relaxing the correction
// path must not relax the operational one. A supervisor on duty still cannot
// rewrite a finished block through the PATCH route.
func TestCorrectAttendance_PatchRouteStaysFrozenAfterCompletion(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	w := doJSON(t, router, http.MethodPatch,
		fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID),
		map[string]any{"note": "über die PATCH-Route"})
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "frozen")

	assert.Nil(t, loadRow(t, s).Note, "the frozen route must not write")
	assert.Empty(t, loadTrail(t, s), "and must not produce a correction trail either")
}

func TestGetAttendanceCorrections_ReturnsTrailNewestFirst(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	completeInstance(t, s, schedule.InstanceStatusCompleted)
	router := correctionRouter(testpkg.Ctx(t), s.res)

	first := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"note":   "erste Fassung",
		"reason": "Nachtrag aus dem Papierprotokoll",
	})
	require.Equal(t, http.StatusOK, first.Code, "body: %s", first.Body.String())

	second := doJSON(t, router, http.MethodPost, correctionPath(s), map[string]any{
		"note":   "zweite Fassung",
		"reason": "Tippfehler im Nachtrag",
	})
	require.Equal(t, http.StatusOK, second.Code, "body: %s", second.Body.String())

	w := doJSON(t, router, http.MethodGet,
		fmt.Sprintf("/instances/%d/students/%d/corrections", s.instanceID, s.studentID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var payload struct {
		Data struct {
			Corrections []struct {
				FieldName string  `json:"field_name"`
				OldValue  *string `json:"old_value"`
				NewValue  *string `json:"new_value"`
				Reason    string  `json:"reason"`
			} `json:"corrections"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload), "body: %s", w.Body.String())
	require.Len(t, payload.Data.Corrections, 2)

	newest := payload.Data.Corrections[0]
	assert.Equal(t, "Tippfehler im Nachtrag", newest.Reason, "newest correction comes first")
	require.NotNil(t, newest.OldValue)
	assert.Equal(t, "erste Fassung", *newest.OldValue, "the trail chains: the second correction's before value is the first one's after value")
	require.NotNil(t, newest.NewValue)
	assert.Equal(t, "zweite Fassung", *newest.NewValue)
}
