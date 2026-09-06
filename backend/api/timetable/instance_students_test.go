// Tests for the WP-B10 three-field attendance PATCH handler.
//
// Split into two halves:
//
//  1. Pure unit tests for validateAttendancePatch — every 400 branch,
//     cross-field rule included. No DB.
//  2. Integration tests for the full handler — real DB + real repo, the
//     router without JWT, using the production tenant transaction middleware.
//
// The integration tests intentionally avoid the authentication stack. Permission
// gating is enforced at the router level in api.go and exercised by the
// existing permission-middleware tests — duplicating that here would not
// cover a distinct behavior.
package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// -----------------------------------------------------------------------------
// Unit tests: validateAttendancePatch
// -----------------------------------------------------------------------------

func TestValidateAttendancePatch_CrossFieldRule(t *testing.T) {
	t.Parallel()

	excused := schedule.AttendanceSubstatusExcused
	tests := []struct {
		name         string
		current      *schedule.InstanceStudent
		patch        schedule.AttendanceFieldPatch
		wantErrField string
		wantOK       bool
	}{
		{
			name:    "expected + new substatus without status change → reject",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
			patch: schedule.AttendanceFieldPatch{
				Substatus: testpkg.StrPtr(schedule.AttendanceSubstatusLate),
			},
			wantErrField: "substatus",
		},
		{
			name:    "expected → present + substatus → ok",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
			patch: schedule.AttendanceFieldPatch{
				Status:    testpkg.StrPtr(schedule.AttendanceStatusPresent),
				Substatus: testpkg.StrPtr(schedule.AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "expected → absent + substatus → ok",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
			patch: schedule.AttendanceFieldPatch{
				Status:    testpkg.StrPtr(schedule.AttendanceStatusAbsent),
				Substatus: testpkg.StrPtr(schedule.AttendanceSubstatusSick),
			},
			wantOK: true,
		},
		{
			name:    "present + substatus → ok (status unchanged)",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent},
			patch: schedule.AttendanceFieldPatch{
				Substatus: testpkg.StrPtr(schedule.AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "present → expected with substatus still set → reject",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent, Substatus: &excused},
			patch: schedule.AttendanceFieldPatch{
				Status: testpkg.StrPtr(schedule.AttendanceStatusExpected),
			},
			wantErrField: "substatus",
		},
		{
			name:    "present → expected clearing substatus → ok",
			current: &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent, Substatus: &excused},
			patch: schedule.AttendanceFieldPatch{
				Status:         testpkg.StrPtr(schedule.AttendanceStatusExpected),
				SubstatusClear: true,
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateAttendancePatch(tc.patch, tc.current)
			if tc.wantOK {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			found := false
			for _, e := range errs {
				if e.Field == tc.wantErrField {
					found = true
					break
				}
			}
			assert.True(t, found, "expected field error on %q, got %+v", tc.wantErrField, errs)
		})
	}
}

func TestValidateAttendancePatch_PerFieldErrors(t *testing.T) {
	t.Parallel()

	cur := &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}

	t.Run("invalid status", func(t *testing.T) {
		errs := validateAttendancePatch(schedule.AttendanceFieldPatch{Status: testpkg.StrPtr("ghost")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "status", errs[0].Field)
	})

	t.Run("invalid substatus", func(t *testing.T) {
		errs := validateAttendancePatch(schedule.AttendanceFieldPatch{Substatus: testpkg.StrPtr("banana")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "substatus", errs[0].Field)
	})

	t.Run("note too long", func(t *testing.T) {
		tooLong := strings.Repeat("x", schedule.InstanceStudentNoteMaxLength+1)
		errs := validateAttendancePatch(schedule.AttendanceFieldPatch{Note: &tooLong}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "note", errs[0].Field)
	})

	t.Run("two per-field errors returned together", func(t *testing.T) {
		tooLong := strings.Repeat("y", schedule.InstanceStudentNoteMaxLength+1)
		errs := validateAttendancePatch(schedule.AttendanceFieldPatch{
			Status: testpkg.StrPtr("ghost"),
			Note:   &tooLong,
		}, cur)
		require.Len(t, errs, 2)
	})
}

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
	row        *schedule.InstanceStudent
	instanceID int64
	studentID  int64
	repo       schedule.InstanceStudentRepository
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
	inst := &schedule.ActivityInstance{
		Date:            schedule.NewDate(2026, 4, 22),
		ActivityGroupID: &activity.ID,
		Title:           fmt.Sprintf("P-Inst-%d", suffix),
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          room.ID,
		Status:          schedule.InstanceStatusPlanned,
	}
	inst.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(inst).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	isRepo := mustTimetableTestRepositories(db).InstanceStudent
	row := &schedule.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     schedule.AttendanceStatusPresent, // start in 'present' so PATCH can mutate freely
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, isRepo.Create(ctx, row))

	res := NewResource(Dependencies{TimetableData: testTimetableData(db), DB: db})

	return &patchSetup{
		res:        res,
		db:         db,
		ctx:        ctx,
		row:        row,
		instanceID: inst.ID,
		studentID:  student.ID,
		repo:       isRepo,
	}
}

// patchRouter mounts the PATCH route with its tenant transaction but no auth.
// The tenant ID is extracted from the passed-in context (built via
// testpkg.TenantContext) and re-applied to the request context so chi's
// routing context survives. A prior version replaced the whole request
// context and stripped chi's routing state, which nil-panicked deep inside.
func patchRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Use(testpkg.TenantTxMiddleware(res.DB))
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
	router := patchRouter(testpkg.Ctx(t), s.res)

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
	s.row.Note = &initial
	s.row.Status = schedule.AttendanceStatusPresent
	_, _ = s.repo.UpdateAttendanceFromCheckin(s.ctx, s.instanceID, s.studentID, time.Now())
	// Directly set note via the repo's update-fields path to be sure.
	require.NoError(t, s.repo.UpdateAttendanceFields(s.ctx, s.row.ID, schedule.AttendanceFieldPatch{
		Note: &initial,
	}))

	router := patchRouter(testpkg.Ctx(t), s.res)
	// Raw JSON body so we can emit explicit null for the note field.
	body := `{"note": null}`
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), body)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Nil(t, data["note"], "note should be cleared to null in response")
}

func TestPatchInstanceStudent_400_EmptyBody(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{})
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "at least one of")
}

func TestPatchInstanceStudent_400_InvalidStatus(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"status": "ghost",
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "status")
}

func TestPatchInstanceStudent_400_InvalidSubstatus(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"substatus": "banana",
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "substatus")
}

func TestPatchInstanceStudent_400_NonStringSubstatus(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	// Substatus is a number, not a string — must be rejected at parse time.
	body := `{"substatus": 5}`
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), body)
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "substatus")
}

func TestPatchInstanceStudent_400_NoteTooLong(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	tooLong := strings.Repeat("x", schedule.InstanceStudentNoteMaxLength+1)
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"note": tooLong,
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "note")
}

func TestPatchInstanceStudent_400_SubstatusOnExpected(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	// Move the row into expected so the cross-field rule fires.
	require.NoError(t, s.repo.UpdateAttendanceFields(s.ctx, s.row.ID, schedule.AttendanceFieldPatch{
		Status: testpkg.StrPtr(schedule.AttendanceStatusExpected),
	}))
	router := patchRouter(testpkg.Ctx(t), s.res)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"substatus": "late",
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "cannot be set when status is expected")
}

func TestPatchInstanceStudent_409_CompletedInstance(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	_, err := s.db.NewUpdate().
		TableExpr("schedule.activity_instances").
		Set("status = ?", schedule.InstanceStatusCompleted).
		Where("id = ?", s.instanceID).
		Exec(s.ctx)
	require.NoError(t, err)

	router := patchRouter(testpkg.Ctx(t), s.res)
	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"note": "nach Abschluss",
	})
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "frozen")

	row, err := s.repo.FindByInstanceAndStudent(s.ctx, s.instanceID, s.studentID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Nil(t, row.Note, "completed instance must not accept a late attendance write")
}

func TestPatchInstanceStudent_404_Unknown(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	router := patchRouter(testpkg.Ctx(t), s.res)

	w := doPatch(t, router, "/instances/999999999/students/999999999", map[string]any{
		"status": "absent",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPatchInstanceStudent_404_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := buildPatchSetup(t)
	// Row created in tenant 1. Fire PATCH with tenant 2 — RLS hides the row.
	router := patchRouter(testpkg.TenantContext(2), s.res)

	w := doPatch(t, router, fmt.Sprintf("/instances/%d/students/%d", s.instanceID, s.studentID), map[string]any{
		"status": "absent",
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "wrong tenant must be 404, not 403")
}
