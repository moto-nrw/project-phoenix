// Tests for the WP-B10 three-field attendance PATCH handler.
//
// The validation rules are owned by timetable.ValidateAttendancePatch
// (attendance_patch_test.go); the handler's 400 branches run without a
// database in instance_students_unit_test.go. This file keeps the tri-state
// decoder unit test and the integration tests for the full handler — real
// DB + real repo, the router without JWT, using the production tenant
// transaction middleware.
//
// The integration tests intentionally avoid the authentication stack. Permission
// gating is enforced at the router level in api.go and exercised by the
// existing permission-middleware tests — duplicating that here would not
// cover a distinct behavior.
package timetablehttp

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
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// -----------------------------------------------------------------------------
// Unit tests: decodeNullableString
// -----------------------------------------------------------------------------

func TestDecodeNullableString(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		got, err := decodeNullableString(nil)
		require.NoError(t, err)
		assert.False(t, got.present)
	})
	t.Run("explicit null", func(t *testing.T) {
		got, err := decodeNullableString(json.RawMessage("null"))
		require.NoError(t, err)
		assert.True(t, got.present)
		assert.True(t, got.isNull)
	})
	t.Run("string value", func(t *testing.T) {
		got, err := decodeNullableString(json.RawMessage(`"late"`))
		require.NoError(t, err)
		assert.True(t, got.present)
		assert.False(t, got.isNull)
		assert.Equal(t, "late", got.value)
	})
	t.Run("non-string rejects", func(t *testing.T) {
		_, err := decodeNullableString(json.RawMessage(`5`))
		require.Error(t, err)
	})
}

// -----------------------------------------------------------------------------
// Integration tests: full PATCH handler
// -----------------------------------------------------------------------------

// patchSetup mirrors attendanceSyncSetup from services/schedule but is local
// to this package so we don't cross the internal boundary. db is retained as
// *bun.DB so tests can drive repo-level state changes outside the handler.
type patchSetup struct {
	res        *Resource
	db         *bun.DB
	ctx        context.Context
	rowID      int64
	instanceID int64
	studentID  int64
	data       timetable.TimetableDataCapability
}

func buildPatchSetup(t *testing.T) *patchSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("P-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("P-Act-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "P-Stu", fmt.Sprintf("One-%d", suffix), "3a")

	// Insert a planned instance for this tenant.
	inst := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2026, 4, 22), room.ID, testpkg.ActivityInstanceOpts{
		Status:          timetable.InstanceStatusPlanned,
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("P-Inst-%d", suffix),
		StartHHMM:       "14:00",
		EndHHMM:         "15:00",
	})

	// start in 'present' so PATCH can mutate freely
	row := testpkg.CreateTestInstanceStudent(t, db, inst.ID, student.ID, timetable.SlotAttendancePresent)

	data := testTimetableData(db)
	res := NewResource(Dependencies{Templates: data, AttendanceCorrections: data.AttendanceCorrections(), TimetableData: data.TimetableData()})

	return &patchSetup{
		res:        res,
		db:         db,
		ctx:        ctx,
		rowID:      row.ID,
		instanceID: inst.ID,
		studentID:  student.ID,
		data:       data.TimetableData(),
	}
}

// patchRouter mounts the PATCH route with its tenant transaction but no auth.
// The tenant ID is extracted from the passed-in context (built via
// testpkg.TenantContext) and re-applied to the request context so chi's
// routing context survives. A prior version replaced the whole request
// context and stripped chi's routing state, which nil-panicked deep inside.
func patchRouter(parentCtx context.Context, res *Resource, db *bun.DB) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Use(testpkg.TenantTxMiddleware(db))
	r.Patch("/instances/{instance_id}/students/{student_id}", res.patchInstanceStudent)
	return r
}

func doPatch(t *testing.T, router chi.Router, path string, body any) *httptest.ResponseRecorder {
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
	req := httptest.NewRequest(http.MethodPatch, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestPatchInstanceStudent_HappyPath(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res, s.db)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"status":    "absent",
		"substatus": "excused",
		"note":      "Arztbesuch",
	})

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "absent", data["status"])
	assert.Equal(t, "excused", data["substatus"])
	assert.Equal(t, "Arztbesuch", data["note"])
}

func TestPatchInstanceStudent_ClearNoteWithExplicitNull(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	// Pre-populate note so the clear is observable.
	initial := "pre"
	_, _ = mustTimetableTestRepositories(s.db).NativeParticipants().UpdateAttendanceFromCheckin(s.ctx, s.instanceID, s.studentID, time.Now())
	// Directly set note via the owner's update-fields path to be sure.
	require.NoError(t, s.data.PatchSlotAttendance(s.ctx, s.rowID, timetable.AttendancePatch{
		Note: &initial,
	}))

	router := patchRouter(testpkg.Ctx(t), s.res, s.db)
	// Raw JSON body so we can emit explicit null for the note field.
	body := `{"note": null}`
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), body)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Nil(t, data["note"], "note should be cleared to null in response")
}

func TestPatchInstanceStudent_409_CompletedInstance(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	testpkg.SetActivityInstanceLifecycle(t, s.ctx, s.db, s.instanceID, timetable.InstanceStatusCompleted)

	router := patchRouter(testpkg.Ctx(t), s.res, s.db)
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"note": "nach Abschluss",
	})
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "frozen")

	row, err := s.data.FindBlockParticipant(s.ctx, s.instanceID, s.studentID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Nil(t, row.Note, "completed instance must not accept a late attendance write")
}

func TestPatchInstanceStudent_404_Unknown(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res, s.db)

	w := doPatch(t, router, "/instances/999999999/students/999999999", map[string]any{
		"status": "absent",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPatchInstanceStudent_404_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	// Row created in tenant 1. Fire PATCH with tenant 2 — RLS hides the row.
	router := patchRouter(testpkg.TenantContext(2), s.res, s.db)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"status": "absent",
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "wrong tenant must be 404, not 403")
}
