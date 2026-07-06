// Tests for WP-B9 instance lifecycle handlers (startInstance, completeInstance,
// cancelInstance) and re-plan-week handler. Pure HTTP unit tests using mock
// services — behavioural coverage of the state machine lives in
// services/schedule/instance_service_integration_test.go.
package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Mock InstanceService — drives the handler's service-layer contract.
// -----------------------------------------------------------------------------

type mockInstanceService struct {
	startResult   *scheduleSvc.StartInstanceResult
	startErr      error
	completeRes   *scheduleModel.ActivityInstance
	completeErr   error
	cancelRes     *scheduleModel.ActivityInstance
	cancelErr     error
	deleteErr     error
	replanRes     *scheduleSvc.ReplanWeekResult
	replanErr     error
	createRes     *scheduleModel.ActivityInstance
	createErr     error
	updateRes     *scheduleModel.ActivityInstance
	updateErr     error
	lastStartID   int64
	lastStartedBy int64
	lastFrom      timezone.Date
	lastTo        timezone.Date
	lastReplanGID *int64
	lastCreate    *scheduleSvc.CreateInstanceInput
	lastUpdate    *scheduleSvc.UpdateInstanceInput
}

func (m *mockInstanceService) GetPlannedStudentIDsByDate(_ context.Context, _ []int64, _ timezone.Date) ([]int64, error) {
	return nil, nil
}

func (m *mockInstanceService) Start(_ context.Context, id, startedBy int64) (*scheduleSvc.StartInstanceResult, error) {
	m.lastStartID = id
	m.lastStartedBy = startedBy
	if m.startErr != nil {
		return nil, m.startErr
	}
	return m.startResult, nil
}

func (m *mockInstanceService) Complete(_ context.Context, _ int64) (*scheduleModel.ActivityInstance, error) {
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	return m.completeRes, nil
}

func (m *mockInstanceService) Cancel(_ context.Context, _ int64) (*scheduleModel.ActivityInstance, error) {
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return m.cancelRes, nil
}

func (m *mockInstanceService) DeleteCancelled(_ context.Context, _ int64) error {
	return m.deleteErr
}

func (m *mockInstanceService) ReplanWeek(_ context.Context, from, to timezone.Date, activityGroupID *int64) (*scheduleSvc.ReplanWeekResult, error) {
	m.lastFrom = from
	m.lastTo = to
	m.lastReplanGID = activityGroupID
	if m.replanErr != nil {
		return nil, m.replanErr
	}
	return m.replanRes, nil
}

func (m *mockInstanceService) Create(_ context.Context, req scheduleSvc.CreateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	reqCopy := req
	m.lastCreate = &reqCopy
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createRes, nil
}

func (m *mockInstanceService) UpdatePlanned(_ context.Context, _ int64, req scheduleSvc.UpdateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	reqCopy := req
	m.lastUpdate = &reqCopy
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.updateRes, nil
}

// -----------------------------------------------------------------------------
// Mock StaffRepository — needed for resolveStartedByStaffID coverage.
// -----------------------------------------------------------------------------

type instMockStaffRepo struct {
	findByPersonIDFn func(ctx context.Context, personID int64) (*userModels.Staff, error)
}

func (m *instMockStaffRepo) Create(_ context.Context, _ *userModels.Staff) error { return nil }
func (m *instMockStaffRepo) FindByID(_ context.Context, _ any) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	if m.findByPersonIDFn != nil {
		return m.findByPersonIDFn(ctx, personID)
	}
	return nil, nil
}
func (m *instMockStaffRepo) Update(_ context.Context, _ *userModels.Staff) error { return nil }
func (m *instMockStaffRepo) Delete(_ context.Context, _ any) error               { return nil }
func (m *instMockStaffRepo) List(_ context.Context, _ map[string]any) ([]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) ListAllWithPerson(_ context.Context) ([]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) UpdateNotes(_ context.Context, _ int64, _ string) error { return nil }
func (m *instMockStaffRepo) ClearWorkTimeModel(_ context.Context, _ int64) error    { return nil }
func (m *instMockStaffRepo) FindWithPerson(_ context.Context, _ int64) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) FindByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) FindWithPersonByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockStaffRepo) ListStaffByRoles(_ context.Context, _ []string) ([]*userModels.StaffWithRoleInfo, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// Mock PersonService — only the two methods the handler touches carry logic.
// -----------------------------------------------------------------------------

type instMockPersonService struct {
	findByAccountIDFn func(ctx context.Context, accountID int64) (*userModels.Person, error)
	staffRepo         userModels.StaffRepository
}

func (m *instMockPersonService) Get(_ context.Context, _ any) (*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) GetByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) Create(_ context.Context, _ *userModels.Person) error { return nil }
func (m *instMockPersonService) Update(_ context.Context, _ *userModels.Person) error { return nil }
func (m *instMockPersonService) Delete(_ context.Context, _ any) error                { return nil }
func (m *instMockPersonService) DeleteStaff(_ context.Context, _ int64) error         { return nil }
func (m *instMockPersonService) List(_ context.Context, _ *base.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) FindByTagID(_ context.Context, _ string) (*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	if m.findByAccountIDFn != nil {
		return m.findByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}
func (m *instMockPersonService) FindByName(_ context.Context, _, _ string) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) LinkToAccount(_ context.Context, _ int64, _ int64) error { return nil }
func (m *instMockPersonService) UnlinkFromAccount(_ context.Context, _ int64) error      { return nil }
func (m *instMockPersonService) LinkToRFIDCard(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *instMockPersonService) UnlinkFromRFIDCard(_ context.Context, _ int64) error { return nil }
func (m *instMockPersonService) GetFullProfile(_ context.Context, _ int64) (*userModels.Person, error) {
	return nil, nil
}
func (m *instMockPersonService) ListAvailableRFIDCards(_ context.Context) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (m *instMockPersonService) ValidateStaffPIN(_ context.Context, _ string) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) ValidateStaffPINForSpecificStaff(_ context.Context, _ int64, _ string) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentsByTeacher(_ context.Context, _ int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentsWithGroupsByTeacher(_ context.Context, _ int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (m *instMockPersonService) GetAllStudentsWithGroups(_ context.Context) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStaffByID(_ context.Context, _ int64) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	if m.staffRepo == nil {
		return nil, nil
	}
	return m.staffRepo.FindByPersonID(ctx, personID)
}
func (m *instMockPersonService) GetStaffWithPerson(_ context.Context, _ int64) (*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStaffWithPersonByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) ListStaffWithPerson(_ context.Context) ([]*userModels.Staff, error) {
	return nil, nil
}
func (m *instMockPersonService) ListStaffByRoles(_ context.Context, _ []string) ([]*userModels.StaffWithRoleInfo, error) {
	return nil, nil
}
func (m *instMockPersonService) GetTeacherByStaffID(_ context.Context, _ int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (m *instMockPersonService) GetTeachersByStaffIDs(_ context.Context, _ []int64) (map[int64]*userModels.Teacher, error) {
	return nil, nil
}
func (m *instMockPersonService) ListTeachersWithStaffAndPerson(_ context.Context) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentByID(_ context.Context, _ int64) (*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentByPersonID(_ context.Context, _ int64) (*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentsByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentsByGroupID(_ context.Context, _ int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetStudentsByGroupIDs(_ context.Context, _ []int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (m *instMockPersonService) GetTeachersBySpecialization(_ context.Context, _ string) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *instMockPersonService) GetTeacherWithStaffAndPerson(_ context.Context, _ int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (m *instMockPersonService) CreateStaffWithTeacher(_ context.Context, _ usersSvc.CreateStaffInput) (*userModels.Staff, *userModels.Teacher, bool, error) {
	return nil, nil, false, nil
}
func (m *instMockPersonService) UpdateStaffWithTeacher(_ context.Context, _ *userModels.Staff, _ bool, _, _, _ string) (*userModels.Teacher, usersSvc.TeacherAction, error) {
	return nil, usersSvc.TeacherActionNone, nil
}
func (m *instMockPersonService) CountStudentsByGroupIDs(_ context.Context, _ []int64) (map[int64]int, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// Router helpers — mirror setupMaterializeRouter, parameterised for each route.
// -----------------------------------------------------------------------------

func setupLifecycleRouter(rs *Resource, route string, handler http.HandlerFunc) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post(route, handler)
	_ = rs
	return r
}

func doPost(t *testing.T, router chi.Router, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doDelete(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// -----------------------------------------------------------------------------
// startInstance handler tests.
// -----------------------------------------------------------------------------

func TestStartInstance_Success(t *testing.T) {
	startedAt := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	mock := &mockInstanceService{
		startResult: &scheduleSvc.StartInstanceResult{
			Instance: &scheduleModel.ActivityInstance{
				Status:    scheduleModel.InstanceStatusActive,
				StartedAt: &startedAt,
			},
			ActiveGroupID: 42,
			Warnings:      nil,
		},
	}
	mock.startResult.Instance.ID = int64(7)

	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/7/start", nil)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, int64(7), mock.lastStartID)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(7), data["instance_id"])
	assert.Equal(t, "active", data["status"])
	assert.Equal(t, float64(42), data["active_group_id"])
	// Warnings must always be an array, never null.
	warnings, ok := data["warnings"].([]any)
	require.True(t, ok, "warnings must be an array")
	assert.Empty(t, warnings)
	assert.Equal(t, "2026-04-20T10:00:00Z", data["started_at"])
}

func TestStartInstance_WithWarnings(t *testing.T) {
	mock := &mockInstanceService{
		startResult: &scheduleSvc.StartInstanceResult{
			Instance:      &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusActive},
			ActiveGroupID: 1,
			Warnings: []scheduleSvc.InstanceConflictWarning{
				{Kind: scheduleSvc.ConflictKindRoom, ResourceID: 5, Message: "Raum belegt", CanOverride: true},
			},
		},
	}
	mock.startResult.Instance.ID = int64(3)

	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/3/start", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	warnings := data["warnings"].([]any)
	assert.Len(t, warnings, 1)
}

func TestStartInstance_InvalidID(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/not-a-number/start", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartInstance_NilService(t *testing.T) {
	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/1/start", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStartInstance_NotFound(t *testing.T) {
	mock := &mockInstanceService{startErr: scheduleSvc.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/99/start", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStartInstance_InvalidTransition(t *testing.T) {
	mock := &mockInstanceService{startErr: errors.New("wrap: " + scheduleSvc.ErrInvalidInstanceTransition.Error())}
	// Use an error that errors.Is matches via wrapping.
	mock.startErr = &wrappedErr{inner: scheduleSvc.ErrInvalidInstanceTransition}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/1/start", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestStartInstance_InternalError(t *testing.T) {
	mock := &mockInstanceService{startErr: errors.New("db connection lost")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/1/start", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// wrappedErr exists so we can wrap a sentinel in an unexported type without
// dragging the services.ScheduleError into this test package.
type wrappedErr struct{ inner error }

func (e *wrappedErr) Error() string { return "wrapped: " + e.inner.Error() }
func (e *wrappedErr) Unwrap() error { return e.inner }

// -----------------------------------------------------------------------------
// completeInstance handler tests.
// -----------------------------------------------------------------------------

func TestCompleteInstance_Success(t *testing.T) {
	completed := time.Date(2026, 4, 20, 12, 30, 0, 0, time.UTC)
	instance := &scheduleModel.ActivityInstance{
		Status:      scheduleModel.InstanceStatusCompleted,
		CompletedAt: &completed,
	}
	instance.ID = int64(5)

	mock := &mockInstanceService{completeRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/5/complete", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(5), data["instance_id"])
	assert.Equal(t, "completed", data["status"])
	assert.Equal(t, "2026-04-20T12:30:00Z", data["completed_at"])
}

func TestCompleteInstance_NoCompletedAt(t *testing.T) {
	// When service returns an instance without CompletedAt (data anomaly),
	// handler must still return 200 with omitted completed_at field.
	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusCompleted}
	instance.ID = int64(5)

	mock := &mockInstanceService{completeRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/5/complete", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "completed_at")
}

func TestCompleteInstance_InvalidID(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/bad/complete", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompleteInstance_NilService(t *testing.T) {
	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCompleteInstance_NotFound(t *testing.T) {
	mock := &mockInstanceService{completeErr: scheduleSvc.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCompleteInstance_InvalidTransition(t *testing.T) {
	mock := &mockInstanceService{completeErr: &wrappedErr{inner: scheduleSvc.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestCompleteInstance_InternalError(t *testing.T) {
	mock := &mockInstanceService{completeErr: errors.New("db error")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -----------------------------------------------------------------------------
// cancelInstance handler tests.
// -----------------------------------------------------------------------------

func TestCancelInstance_Success(t *testing.T) {
	completed := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	instance := &scheduleModel.ActivityInstance{
		Status:      scheduleModel.InstanceStatusCancelled,
		CompletedAt: &completed,
	}
	instance.ID = int64(11)

	mock := &mockInstanceService{cancelRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/11/cancel", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(11), data["instance_id"])
	assert.Equal(t, "cancelled", data["status"])
	assert.Equal(t, "2026-04-20T15:00:00Z", data["completed_at"])
}

func TestCancelInstance_NoCompletedAt(t *testing.T) {
	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusCancelled}
	instance.ID = int64(12)
	mock := &mockInstanceService{cancelRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/12/cancel", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCancelInstance_InvalidID(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/nope/cancel", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelInstance_NilService(t *testing.T) {
	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCancelInstance_NotFound(t *testing.T) {
	mock := &mockInstanceService{cancelErr: scheduleSvc.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelInstance_InvalidTransition(t *testing.T) {
	mock := &mockInstanceService{cancelErr: &wrappedErr{inner: scheduleSvc.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCancelInstance_InternalError(t *testing.T) {
	mock := &mockInstanceService{cancelErr: errors.New("db failure")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -----------------------------------------------------------------------------
// deleteInstance handler tests.
// -----------------------------------------------------------------------------

func TestDeleteInstance_Success(t *testing.T) {
	mock := &mockInstanceService{}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/11")

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestDeleteInstance_InvalidID(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/nope")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteInstance_NilService(t *testing.T) {
	rs := NewResource(Dependencies{})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteInstance_NotFound(t *testing.T) {
	mock := &mockInstanceService{deleteErr: scheduleSvc.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteInstance_InvalidTransition(t *testing.T) {
	mock := &mockInstanceService{deleteErr: &wrappedErr{inner: scheduleSvc.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestDeleteInstance_AmbiguousTemplateInstanceDelete(t *testing.T) {
	mock := &mockInstanceService{deleteErr: &wrappedErr{inner: scheduleSvc.ErrAmbiguousTemplateInstanceDelete}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "ambiguous_template_instance_delete")
	assert.Contains(t, w.Body.String(), "mehrere Termine")
}

func TestDeleteInstance_InternalError(t *testing.T) {
	mock := &mockInstanceService{deleteErr: errors.New("db failure")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -----------------------------------------------------------------------------
// resolveStartedByStaffID tests — exercised directly because the other paths
// only drive the JWT-absent branch through the handler.
// -----------------------------------------------------------------------------

func TestResolveStartedByStaffID_NilPersonService(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	staffID := rs.resolveStartedByStaffID(context.Background())
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_NoClaimsInContext(t *testing.T) {
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: &instMockPersonService{}})
	staffID := rs.resolveStartedByStaffID(context.Background())
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_PersonNotFound(t *testing.T) {
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("person not found")
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_PersonIsNil(t *testing.T) {
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, nil
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffRepoNil(t *testing.T) {
	person := &userModels.Person{}
	person.ID = int64(55)
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return person, nil
		},
		staffRepo: nil, // GetStaffByPersonID resolves no staff
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffNotFound(t *testing.T) {
	person := &userModels.Person{}
	person.ID = int64(55)
	staffRepo := &instMockStaffRepo{
		findByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			return nil, errors.New("not found")
		},
	}
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return person, nil
		},
		staffRepo: staffRepo,
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffIsNil(t *testing.T) {
	person := &userModels.Person{}
	person.ID = int64(55)
	staffRepo := &instMockStaffRepo{
		findByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			return nil, nil
		},
	}
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return person, nil
		},
		staffRepo: staffRepo,
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_Success(t *testing.T) {
	person := &userModels.Person{}
	person.ID = int64(55)
	staff := &userModels.Staff{}
	staff.ID = int64(777)
	staffRepo := &instMockStaffRepo{
		findByPersonIDFn: func(_ context.Context, personID int64) (*userModels.Staff, error) {
			assert.Equal(t, int64(55), personID)
			return staff, nil
		},
	}
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, accountID int64) (*userModels.Person, error) {
			assert.Equal(t, int64(123), accountID)
			return person, nil
		},
		staffRepo: staffRepo,
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, PersonService: ps, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(777), staffID)
}

// startInstance drives resolveStartedByStaffID with JWT claims threaded into
// the request — ensures the handler wires the context through correctly.
func TestStartInstance_PassesStartedByFromJWT(t *testing.T) {
	person := &userModels.Person{}
	person.ID = int64(55)
	staff := &userModels.Staff{}
	staff.ID = int64(888)
	staffRepo := &instMockStaffRepo{
		findByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			return staff, nil
		},
	}
	ps := &instMockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return person, nil
		},
		staffRepo: staffRepo,
	}

	instance := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusActive}
	instance.ID = int64(1)
	mock := &mockInstanceService{
		startResult: &scheduleSvc.StartInstanceResult{
			Instance:      instance,
			ActiveGroupID: 1,
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock, PersonService: ps, Logger: slog.Default()})

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	// Inject JWT claims middleware before the handler.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			claims := jwt.AppClaims{ID: 123}
			ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/instances/{id}/start", rs.startInstance)

	w := doPost(t, r, "/instances/1/start", nil)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, int64(888), mock.lastStartedBy)
}

// -----------------------------------------------------------------------------
// getLogger tests.
// -----------------------------------------------------------------------------

func TestResource_getLogger(t *testing.T) {
	t.Run("returns injected logger", func(t *testing.T) {
		custom := slog.Default()
		rs := NewResource(Dependencies{Logger: custom})
		assert.NotNil(t, rs.getLogger())
	})

	t.Run("falls back to slog.Default when nil", func(t *testing.T) {
		rs := NewResource(Dependencies{})
		assert.NotNil(t, rs.getLogger(), "must never return nil")
	})
}

// -----------------------------------------------------------------------------
// replanWeek handler tests.
// -----------------------------------------------------------------------------

func TestReplanWeekRequest_Bind_NoOp(t *testing.T) {
	req := &replanWeekRequest{}
	err := req.Bind(nil)
	assert.NoError(t, err, "Bind is intentionally a no-op")
}

func TestReplanWeek_NilService(t *testing.T) {
	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	w := doPost(t, router, "/instances/re-plan-week", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestReplanWeek_NoBody_DefaultsToNextWeek(t *testing.T) {
	mock := &mockInstanceService{
		replanRes: &scheduleSvc.ReplanWeekResult{
			From:             timezone.NewDate(2026, 4, 27),
			To:               timezone.NewDate(2026, 5, 3),
			DeletedInstances: 5,
			Materialization: &scheduleSvc.MaterializationResult{
				InstancesCreated:          8,
				CandidatesSkippedExisting: 0,
				InstanceStaffCreated:      4,
				InstanceStudentsCreated:   16,
				DurationMS:                42,
			},
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	w := doPost(t, router, "/instances/re-plan-week", nil)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "2026-04-27", data["from"])
	assert.Equal(t, "2026-05-03", data["to"])
	assert.Equal(t, float64(5), data["deleted_instances"])
	assert.Equal(t, float64(8), data["instances_created"])
	assert.Equal(t, float64(4), data["instance_staff_created"])
	assert.Equal(t, float64(16), data["instance_students_created"])
	assert.Equal(t, float64(42), data["duration_ms"])
}

func TestReplanWeek_ValidBody(t *testing.T) {
	mock := &mockInstanceService{
		replanRes: &scheduleSvc.ReplanWeekResult{
			From:             timezone.NewDate(2026, 4, 27),
			To:               timezone.NewDate(2026, 5, 3),
			DeletedInstances: 2,
			Materialization:  &scheduleSvc.MaterializationResult{InstancesCreated: 3},
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	body := map[string]string{
		"from_date": "2026-04-27",
		"to_date":   "2026-05-03",
	}
	w := doPost(t, router, "/instances/re-plan-week", body)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "2026-04-27", mock.lastFrom.Format("2006-01-02"))
	assert.Equal(t, "2026-05-03", mock.lastTo.Format("2006-01-02"))
}

func TestReplanWeek_NilMaterialization(t *testing.T) {
	// Result with nil Materialization should still render a 200 with zero-valued
	// counts — defensive branch in the handler.
	mock := &mockInstanceService{
		replanRes: &scheduleSvc.ReplanWeekResult{
			From:             timezone.NewDate(2026, 4, 27),
			To:               timezone.NewDate(2026, 5, 3),
			DeletedInstances: 1,
			Materialization:  nil,
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	w := doPost(t, router, "/instances/re-plan-week", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(0), data["instances_created"])
}

func TestReplanWeek_InvalidWindow(t *testing.T) {
	mock := &mockInstanceService{}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	body := map[string]string{
		"from_date": "2026-05-03",
		"to_date":   "2026-04-27", // to < from
	}
	w := doPost(t, router, "/instances/re-plan-week", body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReplanWeek_InvalidJSON(t *testing.T) {
	mock := &mockInstanceService{}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	req := httptest.NewRequest(http.MethodPost, "/instances/re-plan-week", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 9
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReplanWeek_ServiceError(t *testing.T) {
	mock := &mockInstanceService{replanErr: errors.New("db exploded")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	w := doPost(t, router, "/instances/re-plan-week", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -----------------------------------------------------------------------------
// renderInstanceLifecycleError — covered indirectly through the handler tests
// above; this test pins the three branches with a minimal http.ResponseWriter.
// -----------------------------------------------------------------------------

func TestRenderInstanceLifecycleError(t *testing.T) {
	t.Run("not-found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, scheduleSvc.ErrInstanceNotFound)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid-transition", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, scheduleSvc.ErrInvalidInstanceTransition)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_transition")
	})

	t.Run("ambiguous-template-instance-delete", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, scheduleSvc.ErrAmbiguousTemplateInstanceDelete)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "ambiguous_template_instance_delete")
	})

	t.Run("unknown-error-500", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, errors.New("something broke"))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// WP-B3: the optional activity_group_id body field must reach the service.
func TestReplanWeek_ActivityGroupIDPassThrough(t *testing.T) {
	mock := &mockInstanceService{
		replanRes: &scheduleSvc.ReplanWeekResult{
			From: timezone.NewDate(2026, 4, 27),
			To:   timezone.NewDate(2026, 5, 3),
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	body := map[string]any{
		"from_date":         "2026-04-27",
		"to_date":           "2026-05-03",
		"activity_group_id": 4711,
	}
	w := doPost(t, router, "/instances/re-plan-week", body)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	require.NotNil(t, mock.lastReplanGID)
	assert.Equal(t, int64(4711), *mock.lastReplanGID)

	// Omitted field stays nil — whole-grid behavior unchanged.
	w = doPost(t, router, "/instances/re-plan-week", map[string]string{
		"from_date": "2026-04-27",
		"to_date":   "2026-05-03",
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Nil(t, mock.lastReplanGID)
}
