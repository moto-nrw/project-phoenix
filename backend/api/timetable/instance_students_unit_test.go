// Unit tests for the WP-B10 attendance PATCH handler that don't require a
// database. The DB-backed tests in instance_students_test.go cover the happy
// path and cross-field rule end-to-end; these tests close the coverage gap on
// the branches SonarQube flagged as missing: repo-not-wired guard, path
// parsing errors, body decode errors, response mapping, and the
// DatabaseError-wrapped-ErrNoRows branch in isNotFoundDBError.
package timetable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelsBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Fake InstanceStudentRepository
// -----------------------------------------------------------------------------

type fakeRepo struct {
	findByInstanceAndStudent func(ctx context.Context, instanceID, studentID int64) (*schedule.InstanceStudent, error)
	updateAttendanceFields   func(ctx context.Context, id int64, patch schedule.AttendanceFieldPatch) error

	findCalls    int
	updateCalls  int
	recentID     int64
	recentPatch  schedule.AttendanceFieldPatch
	currentState *schedule.InstanceStudent
}

func (f *fakeRepo) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*schedule.InstanceStudent, error) {
	f.findCalls++
	if f.findByInstanceAndStudent != nil {
		return f.findByInstanceAndStudent(ctx, instanceID, studentID)
	}
	return f.currentState, nil
}

func (f *fakeRepo) UpdateAttendanceFields(ctx context.Context, id int64, patch schedule.AttendanceFieldPatch) error {
	f.updateCalls++
	f.recentID = id
	f.recentPatch = patch
	if f.updateAttendanceFields != nil {
		return f.updateAttendanceFields(ctx, id, patch)
	}
	return nil
}

// Unused interface methods — panic so accidental dependence fails loudly.
func (f *fakeRepo) Create(context.Context, *schedule.InstanceStudent) error { panic("unused") }
func (f *fakeRepo) FindByID(context.Context, interface{}) (*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) Update(context.Context, *schedule.InstanceStudent) error { panic("unused") }
func (f *fakeRepo) Delete(context.Context, interface{}) error               { panic("unused") }
func (f *fakeRepo) List(context.Context, *modelsBase.QueryOptions) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) FindByInstanceID(context.Context, int64) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) FindByInstanceIDs(context.Context, []int64) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) FindExpectedByInstanceIDs(context.Context, []int64) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) FindNotScheduledCandidatesByInstanceIDs(context.Context, []int64) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	panic("unused")
}
func (f *fakeRepo) FindByStudentAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) FindPlannedStudentIDsByDate(context.Context, []int64, timezone.Date) ([]int64, error) {
	panic("unused")
}
func (f *fakeRepo) DeleteByInstanceID(context.Context, int64) error { panic("unused") }
func (f *fakeRepo) ArchivePlannedByStudentIDsFrom(context.Context, int64, []int64, timezone.Date, time.Time) (int, error) {
	panic("unused")
}
func (f *fakeRepo) RestoreArchivedByTransition(context.Context, int64, []int64, timezone.Date) (int, error) {
	panic("unused")
}
func (f *fakeRepo) BulkUpdateStatus(context.Context, int64, string, string, []int64) (int, error) {
	panic("unused")
}

func (f *fakeRepo) MarkNotScheduled(context.Context, []schedule.StudentInstanceRef) error {
	panic("unused")
}

func (f *fakeRepo) FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*schedule.ScheduledInstanceRow, error) {
	panic("unused")
}

func (f *fakeRepo) HasPlannedSlotsInRange(context.Context, timezone.Date, timezone.Date) (bool, error) {
	panic("unused")
}

func (f *fakeRepo) UpdateAttendanceFromCheckin(context.Context, int64, int64, time.Time) (bool, error) {
	panic("unused")
}
func (f *fakeRepo) CreateUnplannedPresentIfAbsent(context.Context, int64, int64, time.Time) (*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) UpdateAttendanceCheckout(context.Context, int64, int64, time.Time) error {
	panic("unused")
}
func (f *fakeRepo) ReconcileAttendanceInterval(context.Context, int64, int64, time.Time, *time.Time, time.Time, *time.Time) (bool, error) {
	panic("unused")
}
func (f *fakeRepo) FindCurrentCandidates(context.Context, int64, timezone.Date, time.Time) ([]*schedule.InstanceStudent, error) {
	panic("unused")
}
func (f *fakeRepo) ApplyStatusDay(context.Context, int64, timezone.Date, int64, string) (int, error) {
	panic("unused")
}
func (f *fakeRepo) ReleaseStatusDay(context.Context, int64) (int, error) {
	panic("unused")
}
func (f *fakeRepo) ApplyActiveStatusDaysForInstance(context.Context, int64, timezone.Date) (int, error) {
	panic("unused")
}
func (f *fakeRepo) ApplyPartialAbsence(context.Context, int64) (int, error) {
	panic("unused")
}
func (f *fakeRepo) ReleasePartialAbsence(context.Context, int64) (int, error) {
	panic("unused")
}
func (f *fakeRepo) ApplyActivePartialAbsencesForInstance(context.Context, int64, timezone.Date) (int, error) {
	panic("unused")
}

// -----------------------------------------------------------------------------
// Router helpers
// -----------------------------------------------------------------------------

func unitRouter(res *Resource) chi.Router {
	r := chi.NewRouter()
	r.Patch("/instances/{instance_id}/students/{student_id}", res.patchInstanceStudent)
	return r
}

func patchRequest(t *testing.T, path string, body any) *http.Request {
	t.Helper()
	var buf *bytes.Reader
	switch b := body.(type) {
	case nil:
		buf = bytes.NewReader(nil)
	case string:
		buf = bytes.NewReader([]byte(b))
	default:
		jb, err := json.Marshal(b)
		require.NoError(t, err)
		buf = bytes.NewReader(jb)
	}
	req := httptest.NewRequest(http.MethodPatch, path, buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func run(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// -----------------------------------------------------------------------------
// patchInstanceStudent — guard / wiring branches
// -----------------------------------------------------------------------------

func TestPatchHandler_500_RepoNotWired(t *testing.T) {
	// Resource with instanceStudentRepo == nil must return 500 rather than crash.
	res := &Resource{}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "attendance PATCH not wired")
}

// -----------------------------------------------------------------------------
// parseAttendancePathIDs — 400 branches
// -----------------------------------------------------------------------------

func TestPatchHandler_400_InvalidInstanceID(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/abc/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid instance_id or student_id")
}

func TestPatchHandler_400_InvalidStudentID(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/xyz", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid instance_id or student_id")
}

func TestPatchHandler_400_ZeroInstanceID(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/0/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchHandler_400_NegativeIDs(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/-1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// -----------------------------------------------------------------------------
// decodePatchBody — error / empty / non-string branches
// -----------------------------------------------------------------------------

func TestPatchHandler_400_MalformedJSON(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", "{not json"))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid JSON body")
}

func TestPatchHandler_400_EmptyBody_MeansNoChanges(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	// Empty body → parser returns zero patch → HasChanges false → 400.
	req := httptest.NewRequest(http.MethodPatch, "/instances/1/students/2", bytes.NewReader(nil))
	w := run(router, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least one of")
}

func TestPatchHandler_400_BodyReadError(t *testing.T) {
	// A broken reader forces io.ReadAll to return an error, exercising the
	// "failed to read request body" branch.
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	req := httptest.NewRequest(http.MethodPatch, "/instances/1/students/2", brokenReader{})
	req.Header.Set("Content-Type", "application/json")
	w := run(router, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "failed to read request body")
}

func TestPatchHandler_400_NonStringNote(t *testing.T) {
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: &fakeRepo{}})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", `{"note": 5}`))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "note")
}

// brokenReader always fails on Read, used to exercise the body-read error branch.
type brokenReader struct{}

func (brokenReader) Read(_ []byte) (int, error) { return 0, errors.New("read disrupted") }
func (brokenReader) Close() error               { return nil }

// -----------------------------------------------------------------------------
// Full handler flow via fake repo (no DB)
// -----------------------------------------------------------------------------

func TestPatchHandler_404_NotFound_ErrNoRows(t *testing.T) {
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			return nil, sql.ErrNoRows
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "instance student not found")
}

func TestPatchHandler_404_NotFound_WrappedDatabaseError(t *testing.T) {
	// DatabaseError wrapping sql.ErrNoRows — matches the wrapping style used
	// by the repo layer. Covered by isNotFoundDBError's errors.As branch.
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			return nil, &modelsBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPatchHandler_404_NotFound_NilRowNilError(t *testing.T) {
	// Repo returns (nil, nil) — row genuinely absent rather than DB error.
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			return nil, nil
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPatchHandler_500_FindError_NotNotFound(t *testing.T) {
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			return nil, errors.New("connection reset")
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "load instance student failed")
}

type fakeRecoveryRepo struct {
	lockAttendance func(ctx context.Context, instanceID int64) error
}

func (f *fakeRecoveryRepo) LockAttendance(ctx context.Context, instanceID int64) error {
	if f.lockAttendance != nil {
		return f.lockAttendance(ctx, instanceID)
	}
	return nil
}

func (f *fakeRecoveryRepo) LockOpenVisits(context.Context, int64) error { panic("unused") }
func (f *fakeRecoveryRepo) LockOpenSupervisors(context.Context, int64) error {
	panic("unused")
}
func (f *fakeRecoveryRepo) LockSupervisors(context.Context, []int64) error { panic("unused") }
func (f *fakeRecoveryRepo) Restore(context.Context, int64, schedule.ActivityCompletionSnapshot, time.Time) error {
	panic("unused")
}

func TestPatchHandler_500_LockAttendanceFails(t *testing.T) {
	current := &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}
	current.ID = 7
	repo := &fakeRepo{currentState: current}
	recovery := &fakeRecoveryRepo{
		lockAttendance: func(context.Context, int64) error {
			return errors.New("could not acquire lock")
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
		InstanceStudentRepo: repo,
		RecoveryRepo:        recovery,
	})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "lock attendance failed")
	assert.Equal(t, 0, repo.updateCalls, "lock failure must short-circuit before UPDATE")
}

func TestPatchHandler_500_UpdateError(t *testing.T) {
	current := &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}
	current.ID = 7
	updateErr := errors.New("FK violation")

	repo := &fakeRepo{
		currentState: current,
		updateAttendanceFields: func(context.Context, int64, schedule.AttendanceFieldPatch) error {
			return updateErr
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "update attendance failed")
}

func TestPatchHandler_500_ReloadAfterUpdateFails(t *testing.T) {
	// The handler re-reads after update so the response body reflects post-
	// write state. If that second read fails, the handler must return 500.
	current := &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}
	current.ID = 7

	callCount := 0
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			callCount++
			if callCount == 1 {
				return current, nil
			}
			return nil, errors.New("reload failure")
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "reload updated attendance failed")
	assert.Equal(t, 2, repo.findCalls)
}

func TestPatchHandler_500_ReloadReturnsNil(t *testing.T) {
	// Reload returns (nil, nil) — the handler guards against this edge case
	// because mapAttendanceToResponse would panic on a nil row.
	current := &schedule.InstanceStudent{Status: schedule.AttendanceStatusPresent}
	current.ID = 7

	callCount := 0
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			callCount++
			if callCount == 1 {
				return current, nil
			}
			return nil, nil
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"status": "absent"}))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "reload updated attendance failed")
}

func TestPatchHandler_400_CrossFieldRuleAfterFind(t *testing.T) {
	// The validation branch after FindByInstanceAndStudent only fires when
	// the parsed patch is individually valid but violates the cross-field
	// rule against the loaded current row. Seeding an 'expected' row and
	// patching just a substatus triggers the "substatus cannot be set when
	// status is expected" response path without any parse-level rejection.
	current := &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected}
	current.ID = 11

	repo := &fakeRepo{currentState: current}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/1/students/2", map[string]any{"substatus": "late"}))
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "cannot be set when status is expected")
	assert.Equal(t, 0, repo.updateCalls, "validation failure must short-circuit before UPDATE")
}

func TestPatchHandler_200_HappyPath(t *testing.T) {
	// First read returns "present". Second read reflects the applied patch —
	// mimicking the repo writing fields then re-reading them.
	checkedInAt := time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC)
	current := &schedule.InstanceStudent{
		InstanceID:  100,
		StudentID:   200,
		Status:      schedule.AttendanceStatusPresent,
		CheckedInAt: &checkedInAt,
	}
	current.ID = 7

	absent := schedule.AttendanceStatusAbsent
	sick := schedule.AttendanceSubstatusSick
	note := "Arztbesuch"

	updated := &schedule.InstanceStudent{
		InstanceID:  100,
		StudentID:   200,
		Status:      absent,
		Substatus:   &sick,
		Note:        &note,
		CheckedInAt: &checkedInAt,
	}
	updated.ID = 7

	callCount := 0
	repo := &fakeRepo{
		findByInstanceAndStudent: func(context.Context, int64, int64) (*schedule.InstanceStudent, error) {
			callCount++
			if callCount == 1 {
				return current, nil
			}
			return updated, nil
		},
	}
	res := &Resource{Dependencies: Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{InstanceStudentRepo: repo})}}
	router := unitRouter(res)

	w := run(router, patchRequest(t, "/instances/100/students/200", map[string]any{
		"status":    "absent",
		"substatus": "sick",
		"note":      "Arztbesuch",
	}))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "absent", data["status"])
	assert.Equal(t, "sick", data["substatus"])
	assert.Equal(t, "Arztbesuch", data["note"])
	assert.NotEmpty(t, data["checked_in_at"])
	assert.Equal(t, 1, repo.updateCalls)
	assert.Equal(t, int64(7), repo.recentID)
}

// -----------------------------------------------------------------------------
// mapAttendanceToResponse — cover both CheckedInAt branches
// -----------------------------------------------------------------------------

func TestMapAttendanceToResponse_WithCheckedInAt(t *testing.T) {
	ts := time.Date(2026, 4, 20, 14, 15, 30, 0, time.UTC)
	excused := schedule.AttendanceSubstatusExcused
	note := "Bus-Verspätung"
	row := &schedule.InstanceStudent{
		InstanceID:  11,
		StudentID:   22,
		Status:      schedule.AttendanceStatusPresent,
		Substatus:   &excused,
		Note:        &note,
		CheckedInAt: &ts,
	}
	row.ID = 99

	resp := mapAttendanceToResponse(row)
	assert.Equal(t, int64(99), resp.ID)
	assert.Equal(t, int64(11), resp.InstanceID)
	assert.Equal(t, int64(22), resp.StudentID)
	assert.Equal(t, schedule.AttendanceStatusPresent, resp.Status)
	require.NotNil(t, resp.Substatus)
	assert.Equal(t, schedule.AttendanceSubstatusExcused, *resp.Substatus)
	require.NotNil(t, resp.CheckedInAt)
	assert.Equal(t, "2026-04-20T14:15:30Z", *resp.CheckedInAt)
}

func TestMapAttendanceToResponse_WithoutCheckedInAt(t *testing.T) {
	row := &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected}
	resp := mapAttendanceToResponse(row)
	assert.Nil(t, resp.CheckedInAt)
	assert.Nil(t, resp.Substatus)
	assert.Nil(t, resp.Note)
}

// -----------------------------------------------------------------------------
// parseAttendancePatchRequest — ensure each state reached
// -----------------------------------------------------------------------------

func TestParseAttendancePatchRequest_AllFields(t *testing.T) {
	status := schedule.AttendanceStatusAbsent
	req := &PatchAttendanceRequest{
		Status:    &status,
		Substatus: json.RawMessage(`"sick"`),
		Note:      json.RawMessage(`"note text"`),
	}
	patch, errs := parseAttendancePatchRequest(req)
	require.Empty(t, errs)
	require.NotNil(t, patch.Status)
	assert.Equal(t, schedule.AttendanceStatusAbsent, *patch.Status)
	require.NotNil(t, patch.Substatus)
	assert.Equal(t, schedule.AttendanceSubstatusSick, *patch.Substatus)
	require.NotNil(t, patch.Note)
	assert.Equal(t, "note text", *patch.Note)
	assert.False(t, patch.SubstatusClear)
	assert.False(t, patch.NoteClear)
}

func TestParseAttendancePatchRequest_ClearsViaNull(t *testing.T) {
	req := &PatchAttendanceRequest{
		Substatus: json.RawMessage(`null`),
		Note:      json.RawMessage(`null`),
	}
	patch, errs := parseAttendancePatchRequest(req)
	require.Empty(t, errs)
	assert.True(t, patch.SubstatusClear)
	assert.True(t, patch.NoteClear)
	assert.Nil(t, patch.Substatus)
	assert.Nil(t, patch.Note)
}

func TestParseAttendancePatchRequest_TypeErrorsOnBothFields(t *testing.T) {
	req := &PatchAttendanceRequest{
		Substatus: json.RawMessage(`5`),
		Note:      json.RawMessage(`{"x":1}`),
	}
	_, errs := parseAttendancePatchRequest(req)
	require.Len(t, errs, 2)
	// The order mirrors the parse order — substatus first, then note.
	assert.Equal(t, "substatus", errs[0].Field)
	assert.Equal(t, "note", errs[1].Field)
}

// -----------------------------------------------------------------------------
// decodePatchBody — direct call edge cases
// -----------------------------------------------------------------------------

func TestDecodePatchBody_Direct(t *testing.T) {
	t.Run("valid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"status":"present"}`))
		w := httptest.NewRecorder()
		body, ok := decodePatchBody(w, req)
		require.True(t, ok)
		require.NotNil(t, body)
		require.NotNil(t, body.Status)
		assert.Equal(t, "present", *body.Status)
	})

	t.Run("oversized body gets truncated", func(t *testing.T) {
		// Send more than 16 KiB — io.LimitReader caps it so the decoder sees
		// a truncated input. That should surface as "invalid JSON body".
		huge := strings.Repeat("x", 17*1024)
		req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(fmt.Sprintf(`{"note":"%s"`, huge)))
		w := httptest.NewRecorder()
		_, ok := decodePatchBody(w, req)
		assert.False(t, ok)
	})

	t.Run("empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(""))
		w := httptest.NewRecorder()
		body, ok := decodePatchBody(w, req)
		require.True(t, ok, "empty body is valid — handler interprets as no changes")
		require.NotNil(t, body)
	})

	t.Run("broken reader", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/", io.NopCloser(brokenReader{}))
		w := httptest.NewRecorder()
		_, ok := decodePatchBody(w, req)
		assert.False(t, ok)
	})
}

// Stubs for the issue #585 cleanup refactor interface additions — unused by
// these tests.
func (f *fakeRepo) MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []schedule.StudentInstanceRef) error {
	panic("unused")
}

func (f *fakeRepo) CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error) {
	panic("unused")
}

func (f *fakeRepo) ListStudentInstanceRefsBefore(context.Context, timezone.Date) ([]schedule.StudentInstanceRef, error) {
	panic("unused")
}
