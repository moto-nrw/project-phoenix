// Tests for WP-B9 instance lifecycle handlers (startInstance, completeInstance,
// cancelInstance) and re-plan-week handler. Pure HTTP unit tests using mock
// services — behavioural coverage of the state machine lives in
// modules/timetable/compose.
package timetablehttp

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
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Mock InstanceService — drives the handler's service-layer contract.
// -----------------------------------------------------------------------------

type mockInstanceService struct {
	startResult         *timetable.StartInstanceResult
	startErr            error
	completeRes         *timetable.LifecycleInstance
	completeErr         error
	cancelRes           *timetable.LifecycleInstance
	cancelErr           error
	deleteErr           error
	replanRes           *timetable.ReplanWeekResult
	replanErr           error
	createRes           *timetable.LifecycleInstance
	createErr           error
	updateRes           *timetable.LifecycleInstance
	updateErr           error
	ackRes              *timetable.LifecycleInstance
	ackErr              error
	clearAckErr         error
	lastCancelReason    *string
	lastAckID           int64
	lastAckValue        bool
	lastAckNote         *string
	lastStartID         int64
	lastStartedBy       int64
	lastReopenAccountID int64
	lastReopenIsAdmin   bool
	lastFrom            calendar.Date
	lastTo              calendar.Date
	lastReplanGID       *int64
	lastCreate          *timetable.CreateInstanceInput
	lastUpdate          *timetable.UpdateInstanceInput
	// real, when set, receives the deviation writes (#1886) so DB-backed
	// handler tests keep asserting real row effects.
	real timetable.InstanceLifecycleCapability
}

func (m *mockInstanceService) GetPlannedStudentIDsByDate(_ context.Context, _ []int64, _ calendar.Date) ([]int64, error) {
	return nil, nil
}

func (m *mockInstanceService) Start(_ context.Context, id, startedBy int64) (*timetable.StartInstanceResult, error) {
	m.lastStartID = id
	m.lastStartedBy = startedBy
	if m.startErr != nil {
		return nil, m.startErr
	}
	return m.startResult, nil
}

func (m *mockInstanceService) Complete(_ context.Context, _ int64) (*timetable.LifecycleInstance, error) {
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	return m.completeRes, nil
}

func (m *mockInstanceService) Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*timetable.StartInstanceResult, error) {
	m.lastReopenAccountID = accountID
	m.lastReopenIsAdmin = isAdmin
	if m.real != nil {
		return m.real.Reopen(ctx, instanceID, accountID, isAdmin)
	}
	return m.startResult, m.startErr
}

func (m *mockInstanceService) CancelWithNotice(ctx context.Context, in timetable.CancelInstanceInput) (*timetable.CancelInstanceResult, error) {
	instance, err := m.Cancel(ctx, in.InstanceID, in.Reason, in.ActorAccountID)
	if err != nil {
		return nil, err
	}
	return &timetable.CancelInstanceResult{Instance: instance}, nil
}

func (m *mockInstanceService) GuardianNoticeReachFor(context.Context, int64) (*timetable.GuardianNoticeReach, error) {
	return &timetable.GuardianNoticeReach{}, nil
}

func (m *mockInstanceService) Cancel(_ context.Context, _ int64, reason *string, _ *int64) (*timetable.LifecycleInstance, error) {
	m.lastCancelReason = reason
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return m.cancelRes, nil
}

func (m *mockInstanceService) DeleteCancelled(_ context.Context, _ int64) error {
	return m.deleteErr
}

func (m *mockInstanceService) BulkCancelPlanned(ctx context.Context, from, to calendar.Date, opts timetable.BulkCancelOptions, actor *int64) (*timetable.BulkCancelResult, error) {
	if m.real != nil {
		return m.real.BulkCancelPlanned(ctx, from, to, opts, actor)
	}
	return nil, nil
}

func (m *mockInstanceService) SetUnderstaffedAck(_ context.Context, instanceID int64, ack bool, note *string, _ *int64) (*timetable.LifecycleInstance, error) {
	m.lastAckID = instanceID
	m.lastAckValue = ack
	m.lastAckNote = note
	if m.ackErr != nil {
		return nil, m.ackErr
	}
	return m.ackRes, nil
}

func (m *mockInstanceService) ClearUnderstaffedAckIfStaffed(_ context.Context, _ int64, _ *int64) error {
	return m.clearAckErr
}

func (m *mockInstanceService) ReplanWeek(_ context.Context, from, to calendar.Date, activityGroupID *int64, _ *int64) (*timetable.ReplanWeekResult, error) {
	m.lastFrom = from
	m.lastTo = to
	m.lastReplanGID = activityGroupID
	if m.replanErr != nil {
		return nil, m.replanErr
	}
	return m.replanRes, nil
}

func (m *mockInstanceService) CreateInstance(_ context.Context, req timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
	reqCopy := req
	m.lastCreate = &reqCopy
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createRes, nil
}

func (m *mockInstanceService) UpdatePlanned(_ context.Context, _ int64, req timetable.UpdateInstanceInput, _ *int64) (*timetable.LifecycleInstance, error) {
	reqCopy := req
	m.lastUpdate = &reqCopy
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	return m.updateRes, nil
}

// -----------------------------------------------------------------------------
// Router helpers — mirror setupMaterializeRouter, parameterised for each route.
// -----------------------------------------------------------------------------

func setupLifecycleRouter(rs *Resource, route string, handler http.HandlerFunc) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post(route, handler)
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
	t.Parallel()

	startedAt := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	mock := &mockInstanceService{
		startResult: &timetable.StartInstanceResult{
			Instance: &timetable.LifecycleInstance{
				Status:    timetable.InstanceStatusActive,
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
	t.Parallel()

	mock := &mockInstanceService{
		startResult: &timetable.StartInstanceResult{
			Instance:      &timetable.LifecycleInstance{Status: timetable.InstanceStatusActive},
			ActiveGroupID: 1,
			Warnings: []timetable.InstanceConflictWarning{
				{Kind: timetable.ConflictKindStaff, ResourceID: 5, Message: "Mitarbeiter doppelt eingeplant", CanOverride: true},
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
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/not-a-number/start", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartInstance_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/1/start", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStartInstance_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{startErr: timetable.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/99/start", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStartInstance_InvalidTransition(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{startErr: errors.New("wrap: " + timetable.ErrInvalidInstanceTransition.Error())}
	// Use an error that errors.Is matches via wrapping.
	mock.startErr = &wrappedErr{inner: timetable.ErrInvalidInstanceTransition}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/start", rs.startInstance)

	w := doPost(t, router, "/instances/1/start", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestStartInstance_InternalError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	completed := time.Date(2026, 4, 20, 12, 30, 0, 0, time.UTC)
	instance := &timetable.LifecycleInstance{
		Status:      timetable.InstanceStatusCompleted,
		CompletedAt: &completed,
	}
	instance.ID = int64(5)

	mock := &mockInstanceService{completeRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/5/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(5), data["instance_id"])
	assert.Equal(t, "completed", data["status"])
	assert.Equal(t, "2026-04-20T12:30:00Z", data["completed_at"])
}

func TestCompleteInstance_NoCompletedAt(t *testing.T) {
	t.Parallel()

	// When service returns an instance without CompletedAt (data anomaly),
	// handler must still return 200 with omitted completed_at field.
	instance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusCompleted}
	instance.ID = int64(5)

	mock := &mockInstanceService{completeRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/5/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "completed_at")
}

func TestCompleteInstance_InvalidID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/bad/complete", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReopenInstance_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/reopen", rs.reopenInstance)

	w := doPost(t, router, "/instances/1/reopen", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestReopenInstance_InvalidID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/reopen", rs.reopenInstance)

	w := doPost(t, router, "/instances/bad/reopen", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReopenInstance_EffectiveAdminScope(t *testing.T) {
	t.Parallel()

	started := &timetable.LifecycleInstance{Status: timetable.InstanceStatusActive}
	started.ID = 7
	mock := &mockInstanceService{
		startResult: &timetable.StartInstanceResult{
			Instance:      started,
			ActiveGroupID: 42,
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/reopen", rs.reopenInstance)

	cases := []struct {
		name        string
		isAdmin     bool
		permissions []string
		wantAdmin   bool
	}{
		{name: "wildcard admin without admin role", permissions: []string{"admin:*"}, wantAdmin: true},
		{name: "full-access wildcard", permissions: []string{"*:*"}, wantAdmin: true},
		{name: "literal admin role", isAdmin: true, permissions: []string{"schedules:manage"}, wantAdmin: true},
		{name: "ordinary staff", permissions: []string{"schedules:manage"}, wantAdmin: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock.lastReopenIsAdmin = false
			req := httptest.NewRequest(http.MethodPost, "/instances/7/reopen", bytes.NewReader(nil))
			testutil.WithClaims(t, jwt.AppClaims{ID: 88, IsAdmin: tc.isAdmin, TenantID: testpkg.Tenant(t)})(req)
			testutil.WithPermissions(tc.permissions...)(req)
			attachTestPrincipal(t, req, jwt.AppClaims{ID: 88, IsAdmin: tc.isAdmin, TenantID: testpkg.Tenant(t)}, tc.permissions)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
			assert.Equal(t, int64(88), mock.lastReopenAccountID)
			assert.Equal(t, tc.wantAdmin, mock.lastReopenIsAdmin)
		})
	}
}

func TestCompleteInstance_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCompleteInstance_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{completeErr: timetable.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCompleteInstance_StaleConfirmation(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{completeErr: timetable.ErrCompletionConfirmationStale}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", map[string]any{"confirmed_present_student_ids": []int64{7}})

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "completion_confirmation_stale")
}

func TestCompleteInstance_InvalidTransition(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{completeErr: &wrappedErr{inner: timetable.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestCompleteInstance_InternalError(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{completeErr: errors.New("db error")}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/complete", rs.completeInstance)

	w := doPost(t, router, "/instances/1/complete", map[string]any{"confirmed_present_student_ids": []int64{}})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -----------------------------------------------------------------------------
// cancelInstance handler tests.
// -----------------------------------------------------------------------------

func TestCancelInstance_Success(t *testing.T) {
	t.Parallel()

	completed := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	instance := &timetable.LifecycleInstance{
		Status:      timetable.InstanceStatusCancelled,
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

// #1840: cancel accepts an optional reason and forwards it trimmed.
func TestCancelInstance_WithReason(t *testing.T) {
	t.Parallel()

	instance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusCancelled}
	instance.ID = int64(21)
	mock := &mockInstanceService{cancelRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/21/cancel", map[string]any{"reason": "  Ausflug  "})
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mock.lastCancelReason)
	assert.Equal(t, "Ausflug", *mock.lastCancelReason)
}

func TestCancelInstance_NoCompletedAt(t *testing.T) {
	t.Parallel()

	instance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusCancelled}
	instance.ID = int64(12)
	mock := &mockInstanceService{cancelRes: instance}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/12/cancel", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCancelInstance_InvalidID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/nope/cancel", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelInstance_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCancelInstance_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{cancelErr: timetable.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelInstance_InvalidTransition(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{cancelErr: &wrappedErr{inner: timetable.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := setupLifecycleRouter(rs, "/instances/{id}/cancel", rs.cancelInstance)

	w := doPost(t, router, "/instances/1/cancel", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCancelInstance_InternalError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	mock := &mockInstanceService{}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/11")

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestDeleteInstance_InvalidID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/nope")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteInstance_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteInstance_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{deleteErr: timetable.ErrInstanceNotFound}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteInstance_InvalidTransition(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{deleteErr: &wrappedErr{inner: timetable.ErrInvalidInstanceTransition}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_transition")
}

func TestDeleteInstance_AmbiguousTemplateInstanceDelete(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{deleteErr: &wrappedErr{inner: timetable.ErrAmbiguousTemplateInstanceDelete}}
	rs := NewResource(Dependencies{InstanceService: mock})
	router := chi.NewRouter()
	router.Delete("/instances/{id}", rs.deleteInstance)

	w := doDelete(t, router, "/instances/1")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "ambiguous_template_instance_delete")
	assert.Contains(t, w.Body.String(), "mehrere Termine")
}

func TestDeleteInstance_InternalError(t *testing.T) {
	t.Parallel()

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

func TestResolveStartedByStaffID_NilPeople(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}})
	staffID := rs.resolveStartedByStaffID(context.Background())
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_NoClaimsInContext(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: &fakePeople{}})
	staffID := rs.resolveStartedByStaffID(context.Background())
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_PersonNotFound(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, errors.New("person not found")
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_PersonIsNil(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffRepoNil(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 55, nil
		},
		// PersonStaffIDFn left nil — the person resolves to no staff member
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffNotFound(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 55, nil
		},
		PersonStaffIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, errors.New("not found")
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_StaffIsNil(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 55, nil
		},
		PersonStaffIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(0), staffID)
}

func TestResolveStartedByStaffID_Success(t *testing.T) {
	t.Parallel()

	people := &fakePeople{
		AccountPersonIDFn: func(_ context.Context, accountID int64) (int64, error) {
			assert.Equal(t, int64(123), accountID)
			return 55, nil
		},
		PersonStaffIDFn: func(_ context.Context, personID int64) (int64, error) {
			assert.Equal(t, int64(55), personID)
			return 777, nil
		},
	}
	rs := NewResource(Dependencies{InstanceService: &mockInstanceService{}, People: people, Logger: slog.Default()})
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	staffID := rs.resolveStartedByStaffID(ctx)
	assert.Equal(t, int64(777), staffID)
}

// startInstance drives resolveStartedByStaffID with JWT claims threaded into
// the request — ensures the handler wires the context through correctly.
func TestStartInstance_PassesStartedByFromJWT(t *testing.T) {
	t.Parallel()

	people := staffAccountPeople(55, 888)

	instance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusActive}
	instance.ID = int64(1)
	mock := &mockInstanceService{
		startResult: &timetable.StartInstanceResult{
			Instance:      instance,
			ActiveGroupID: 1,
		},
	}
	rs := NewResource(Dependencies{InstanceService: mock, People: people, Logger: slog.Default()})

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
	t.Parallel()

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
	t.Parallel()

	req := &replanWeekRequest{}
	err := req.Bind(nil)
	assert.NoError(t, err, "Bind is intentionally a no-op")
}

func TestReplanWeek_NilService(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupLifecycleRouter(rs, "/instances/re-plan-week", rs.replanWeek)

	w := doPost(t, router, "/instances/re-plan-week", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestReplanWeek_NoBody_DefaultsToNextWeek(t *testing.T) {
	t.Parallel()

	mock := &mockInstanceService{
		replanRes: &timetable.ReplanWeekResult{
			From:             calendar.NewDate(2026, 4, 27),
			To:               calendar.NewDate(2026, 5, 3),
			DeletedInstances: 5,
			Materialization: &timetable.MaterializationResult{
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
	t.Parallel()

	mock := &mockInstanceService{
		replanRes: &timetable.ReplanWeekResult{
			From:             calendar.NewDate(2026, 4, 27),
			To:               calendar.NewDate(2026, 5, 3),
			DeletedInstances: 2,
			Materialization:  &timetable.MaterializationResult{InstancesCreated: 3},
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
	t.Parallel()

	// Result with nil Materialization should still render a 200 with zero-valued
	// counts — defensive branch in the handler.
	mock := &mockInstanceService{
		replanRes: &timetable.ReplanWeekResult{
			From:             calendar.NewDate(2026, 4, 27),
			To:               calendar.NewDate(2026, 5, 3),
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	t.Run("not-found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, timetable.ErrInstanceNotFound)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid-transition", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, timetable.ErrInvalidInstanceTransition)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "invalid_transition")
	})

	t.Run("ambiguous-template-instance-delete", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, timetable.ErrAmbiguousTemplateInstanceDelete)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "ambiguous_template_instance_delete")
	})

	t.Run("instance-moved", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, timetable.ErrInstanceMoved)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "instance_moved")
	})

	t.Run("stale-completion-confirmation", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		renderInstanceLifecycleError(w, r, timetable.ErrCompletionConfirmationStale)
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "completion_confirmation_stale")
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
	t.Parallel()

	mock := &mockInstanceService{
		replanRes: &timetable.ReplanWeekResult{
			From: calendar.NewDate(2026, 4, 27),
			To:   calendar.NewDate(2026, 5, 3),
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

func (m *mockInstanceService) QueueActivityUpdates(ctx context.Context, touched timetable.TouchedActivities) {
	if m.real != nil {
		m.real.QueueActivityUpdates(ctx, touched)
	}
}

// MoveStaffBetweenBlocks delegates to a real InstanceService when wired
// (interface completeness for #1884; the mock-only path is unused today).
func (m *mockInstanceService) MoveStaffBetweenBlocks(ctx context.Context, targetID int64, in timetable.MoveStaffInput) (*timetable.MoveStaffResult, error) {
	if m.real != nil {
		return m.real.MoveStaffBetweenBlocks(ctx, targetID, in)
	}
	return &timetable.MoveStaffResult{}, nil
}

// AcknowledgeUnderstaffed records the forwarded args for the mock-only handler
// tests, or delegates to a real service (the DB-backed past-block guard test).
func (m *mockInstanceService) AcknowledgeUnderstaffed(ctx context.Context, id int64, ack bool, note *string, actor *int64) (*timetable.LifecycleInstance, error) {
	if m.real != nil {
		return m.real.AcknowledgeUnderstaffed(ctx, id, ack, note, actor)
	}
	m.lastAckID = id
	m.lastAckValue = ack
	m.lastAckNote = note
	if m.ackErr != nil {
		return nil, m.ackErr
	}
	return m.ackRes, nil
}
