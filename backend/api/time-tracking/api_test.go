package timetracking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Mock WorkSessionService ---

type mockWorkSessionService struct {
	checkInFn            func(ctx context.Context, staffID int64, status, source, reason string) (*activeModels.WorkSession, error)
	checkOutFn           func(ctx context.Context, staffID int64, reason string) (*activeModels.WorkSession, error)
	startBreakFn         func(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	endBreakFn           func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getSessionBreaksFn   func(ctx context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	updateSessionFn      func(ctx context.Context, staffID int64, sessionID int64, updates activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error)
	getCurrentSessionFn  func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getHistoryFn         func(ctx context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error)
	getSessionEditsFn    func(ctx context.Context, staffID, sessionID int64) ([]*activeSvc.WorkSessionEditView, error)
	getTodayPresenceFn   func(ctx context.Context) (map[int64]string, error)
	exportSessionsFn     func(ctx context.Context, staffID int64, from, to timezone.Date, format string) ([]byte, string, error)
	autoEndExpiredBreaks func(ctx context.Context) (int, error)
}

func (m *mockWorkSessionService) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*activeModels.WorkSession, error) {
	if m.checkInFn != nil {
		return m.checkInFn(ctx, staffID, status, source, reason)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) CheckInOn(ctx context.Context, staffID int64, _ timezone.Date, status, source, reason string) (*activeModels.WorkSession, error) {
	return m.CheckIn(ctx, staffID, status, source, reason)
}
func (m *mockWorkSessionService) CheckOut(ctx context.Context, staffID int64, reason string) (*activeModels.WorkSession, error) {
	if m.checkOutFn != nil {
		return m.checkOutFn(ctx, staffID, reason)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
	if m.startBreakFn != nil {
		return m.startBreakFn(ctx, staffID, plannedDurationMinutes)
	}
	return &activeModels.WorkSessionBreak{}, nil
}
func (m *mockWorkSessionService) EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.endBreakFn != nil {
		return m.endBreakFn(ctx, staffID)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) CheckOutOn(ctx context.Context, staffID int64, _ timezone.Date, reason string) (*activeModels.WorkSession, error) {
	return m.CheckOut(ctx, staffID, reason)
}

func (m *mockWorkSessionService) StartBreakOn(ctx context.Context, staffID int64, _ timezone.Date, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
	return m.StartBreak(ctx, staffID, plannedDurationMinutes)
}

func (m *mockWorkSessionService) EndBreakOn(ctx context.Context, staffID int64, _ timezone.Date) (*activeModels.WorkSession, error) {
	return m.EndBreak(ctx, staffID)
}

func (m *mockWorkSessionService) GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	if m.getSessionBreaksFn != nil {
		return m.getSessionBreaksFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
	if m.updateSessionFn != nil {
		return m.updateSessionFn(ctx, staffID, sessionID, updates)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.getCurrentSessionFn != nil {
		return m.getCurrentSessionFn(ctx, staffID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) GetLatestOpenSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	return m.GetCurrentSession(ctx, staffID)
}
func (m *mockWorkSessionService) GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error) {
	if m.getHistoryFn != nil {
		return m.getHistoryFn(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockWorkSessionService) GetSessionEdits(ctx context.Context, staffID, sessionID int64) ([]*activeSvc.WorkSessionEditView, error) {
	if m.getSessionEditsFn != nil {
		return m.getSessionEditsFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) GetSessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*activeSvc.WorkSessionEditView, error) {
	if m.getSessionEditsFn != nil {
		return m.getSessionEditsFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayPresenceFn != nil {
		return m.getTodayPresenceFn(ctx)
	}
	return map[int64]string{}, nil
}
func (m *mockWorkSessionService) CleanupOpenSessions(_ context.Context) (int, error) { return 0, nil }
func (m *mockWorkSessionService) AutoCheckoutDueSessions(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (m *mockWorkSessionService) SetStaffShiftRepo(_ scheduleModels.StaffShiftRepository) {}
func (m *mockWorkSessionService) EnsureCheckedIn(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (m *mockWorkSessionService) ExportSessions(ctx context.Context, staffID int64, from, to timezone.Date, format string) ([]byte, string, error) {
	if m.exportSessionsFn != nil {
		return m.exportSessionsFn(ctx, staffID, from, to, format)
	}
	return []byte("data"), "export.csv", nil
}
func (m *mockWorkSessionService) AutoEndExpiredBreaks(ctx context.Context) (int, error) {
	if m.autoEndExpiredBreaks != nil {
		return m.autoEndExpiredBreaks(ctx)
	}
	return 0, nil
}

func (m *mockWorkSessionService) GetStaffIDsWithSupervisionToday(_ context.Context) ([]int64, error) {
	return nil, nil
}

func (m *mockWorkSessionService) GetWorkTimeModelByID(_ context.Context, _ int64) (*configModels.WorkTimeModel, error) {
	return nil, nil
}

func (m *mockWorkSessionService) GetCurrentScheduleRows(_ context.Context, _ int64) ([]*configModels.StaffWorkSchedule, error) {
	return nil, nil
}

func (m *mockWorkSessionService) UpdateSchedule(_ context.Context, _ *userModels.Staff, _ activeSvc.ScheduleUpdateInput) error {
	return nil
}

func (m *mockWorkSessionService) AssignScheduleTemplate(_ context.Context, _ *userModels.Staff, _ int64) error {
	return nil
}

func (m *mockWorkSessionService) ApplyCustomScheduleRows(_ context.Context, _ *userModels.Staff, _ []*configModels.StaffWorkSchedule, _ timezone.Date) error {
	return nil
}

func (m *mockWorkSessionService) SaveCustomScheduleAsTemplate(_ context.Context, _ *userModels.Staff, _ string, _ int, _ timezone.Date, _ []*configModels.WorkTimeModelEntry) error {
	return nil
}

// UpdateSessionAsAdmin + CreateSessionAsAdmin are part of the WorkSessionService
// interface added for Tranche 1b (admin nachtragen / edit). No-op defaults so
// the MA-side tests still satisfy the interface.
func (m *mockWorkSessionService) UpdateSessionAsAdmin(_ context.Context, _, _, _ int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (m *mockWorkSessionService) CreateSessionAsAdmin(_ context.Context, _, _ int64, _ activeSvc.AdminCreateSessionRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}

// --- Mock StaffAbsenceService ---

type mockStaffAbsenceService struct {
	createAbsenceFn     func(ctx context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error)
	updateAbsenceFn     func(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error)
	deleteAbsenceFn     func(ctx context.Context, staffID int64, absenceID int64) error
	getAbsencesForRange func(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeSvc.StaffAbsenceResponse, error)
	hasAbsenceOnDateFn  func(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error)
}

func (m *mockStaffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
	if m.createAbsenceFn != nil {
		return m.createAbsenceFn(ctx, staffID, req)
	}
	return &activeSvc.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
	if m.updateAbsenceFn != nil {
		return m.updateAbsenceFn(ctx, staffID, actorAccountID, absenceID, req)
	}
	return &activeSvc.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	if m.deleteAbsenceFn != nil {
		return m.deleteAbsenceFn(ctx, staffID, absenceID)
	}
	return nil
}

// The #1843 *For variants delegate to the existing fn fields so the tests
// written against CreateAbsence/DeleteAbsence keep exercising their hooks.
func (m *mockStaffAbsenceService) CreateAbsenceFor(ctx context.Context, subjectStaffID, _ int64, _ *int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
	return m.CreateAbsence(ctx, subjectStaffID, req)
}

func (m *mockStaffAbsenceService) DeleteAbsenceFor(ctx context.Context, subjectStaffID, _ int64, _ *int64, absenceID int64) error {
	return m.DeleteAbsence(ctx, subjectStaffID, absenceID)
}

func (m *mockStaffAbsenceService) SetShiftPlanSyncer(activeSvc.ShiftPlanSyncer) {}
func (m *mockStaffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeSvc.StaffAbsenceResponse, error) {
	if m.getAbsencesForRange != nil {
		return m.getAbsencesForRange(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockStaffAbsenceService) GetTodayAbsenceMap(_ context.Context) (map[int64]string, error) {
	return nil, nil
}

func (m *mockStaffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error) {
	if m.hasAbsenceOnDateFn != nil {
		return m.hasAbsenceOnDateFn(ctx, staffID, date)
	}
	return false, nil, nil
}

// Vacation workflow methods (Tranche 4): no-op defaults so MA-side tests
// satisfy the StaffAbsenceService interface without exercising the flow.
func (m *mockStaffAbsenceService) RequestVacation(_ context.Context, _ int64, _ activeSvc.RequestVacationRequest) (*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) ApproveAbsence(_ context.Context, _ int64, _ int64, _ int64, _ string) (*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) DenyAbsence(_ context.Context, _ int64, _ int64, _ int64, _ string) (*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) QuestionAbsence(_ context.Context, _ int64, _ int64, _ string) (*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}

func (m *mockStaffAbsenceService) ResubmitAbsence(_ context.Context, _ int64, _ int64, _ int64, _ string) (*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}

func (m *mockStaffAbsenceService) CancelAbsence(_ context.Context, _ int64, _ int64, _ int64) error {
	return nil
}
func (m *mockStaffAbsenceService) GetVacationQuotaSummary(_ context.Context, _ int64, _ int) (*activeSvc.VacationQuotaSummary, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) UpsertVacationQuota(_ context.Context, _ int64, _ int, _, _ float64) error {
	return nil
}
func (m *mockStaffAbsenceService) ListPendingRequests(_ context.Context) ([]*activeSvc.StaffAbsenceResponse, error) {
	return nil, nil
}

// newMockSettingsService builds a configtest.Mock reproducing the former
// hand-rolled mockSettingsService stub: Resolve/ResolveString/
// ResolveStringForTenant all return the same (stringVal, stringErr) pair,
// and HasTenantOverride reports true whenever stringVal is non-empty
// (independent of stringErr).
func newMockSettingsService(stringVal string, stringErr error) *configtest.Mock {
	resolveString := func(context.Context, string) (string, error) {
		return stringVal, stringErr
	}
	return &configtest.Mock{
		ResolveFn: func(context.Context, string) (any, error) {
			return stringVal, stringErr
		},
		ResolveStringFn: resolveString,
		ResolveStringForTenantFn: func(ctx context.Context, _ int64, key string) (string, error) {
			return resolveString(ctx, key)
		},
		HasTenantOverrideFn: func(context.Context, string) (bool, error) {
			return stringVal != "", nil
		},
	}
}

// --- Test helpers ---

func defaultPersonSvc() *userstest.PersonServiceMock {
	staffRepo := &testpkg.StaffRepoMock{
		FindByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			s := &userModels.Staff{}
			s.ID = 100
			return s, nil
		},
	}
	return &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			p := &userModels.Person{}
			p.ID = 10
			return p, nil
		},
		GetStaffByPersonIDFn: staffRepo.FindByPersonID,
	}
}

func testResource(wsSvc *mockWorkSessionService, absSvc *mockStaffAbsenceService, pSvc *userstest.PersonServiceMock, db *bun.DB) *Resource {
	return NewResource(wsSvc, absSvc, pSvc, nil, nil, nil, nil, db)
}

func withClaims(r *http.Request, claims jwt.AppClaims) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxClaims, claims)
	if claims.TenantID != 0 {
		ctx = tenant.WithTenantID(ctx, claims.TenantID)
	}
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func validClaims() jwt.AppClaims {
	return jwt.AppClaims{ID: 1, TenantID: 1}
}

type apiResponse struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
}

func parseAPIResponse(t *testing.T, w *httptest.ResponseRecorder) apiResponse {
	t.Helper()
	var resp apiResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// --- Interface compliance ---

var (
	_ activeSvc.WorkSessionService  = (*mockWorkSessionService)(nil)
	_ activeSvc.StaffAbsenceService = (*mockStaffAbsenceService)(nil)
)

// --- CheckInRequest.Bind ---

func TestCheckInRequest_Bind(t *testing.T) {
	t.Run("valid present", func(t *testing.T) {
		assert.NoError(t, (&CheckInRequest{Status: "present"}).Bind(nil))
	})
	t.Run("valid home_office", func(t *testing.T) {
		assert.NoError(t, (&CheckInRequest{Status: "home_office"}).Bind(nil))
	})
	t.Run("invalid status", func(t *testing.T) {
		err := (&CheckInRequest{Status: "invalid"}).Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status must be")
	})
	t.Run("empty status", func(t *testing.T) {
		err := (&CheckInRequest{Status: ""}).Bind(nil)
		require.Error(t, err)
	})
}

// --- NewResource ---

func TestNewResource(t *testing.T) {
	rs := NewResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), newMockSettingsService("", nil), nil, nil, nil, nil)
	assert.NotNil(t, rs)
	assert.NotNil(t, rs.WorkSessionService)
	assert.NotNil(t, rs.StaffAbsenceService)
	assert.NotNil(t, rs.PersonService)
	assert.NotNil(t, rs.SettingsService)
}

// --- Router ---

func TestRouter(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	router := rs.Router()
	assert.NotNil(t, router)
}

// --- getStaffIDFromClaims ---

func TestGetStaffIDFromClaims_Success(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	claims := jwt.AppClaims{ID: 1, TenantID: 1}
	staffID, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.NoError(t, err)
	assert.Equal(t, int64(100), staffID)
}

func TestGetStaffIDFromClaims_ZeroID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	claims := jwt.AppClaims{ID: 0}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestGetStaffIDFromClaims_PersonNotFound(t *testing.T) {
	pSvc := &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc, nil)
	claims := jwt.AppClaims{ID: 1, TenantID: 1}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "person not found")
}

func TestGetStaffIDFromClaims_StaffNotFound(t *testing.T) {
	pSvc := &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			p := &userModels.Person{}
			p.ID = 10
			return p, nil
		},
		GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			return nil, errors.New("not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc, nil)
	claims := jwt.AppClaims{ID: 1, TenantID: 1}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staff record not found")
}

// --- checkIn handler ---

func TestCheckIn_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, staffID int64, status, source, _ string) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "present", status)
			assert.Equal(t, activeModels.WorkSessionSourceApp, source)
			ws := &activeModels.WorkSession{Status: "present", Source: source}
			ws.ID = 1
			return ws, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	assert.Equal(t, "success", resp.Status)
}

func TestCheckIn_InvalidBody(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"status":"invalid"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckIn_InvalidClaims(t *testing.T) {
	pSvc := &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc, nil)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckIn_ServiceConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*activeModels.WorkSession, error) {
			return nil, errors.New("already checked in")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestCheckIn_ReopenStatusConflict locks in the HTTP-boundary contract for
// Issue #1368: when the service returns *ReopenStatusConflictError, the
// handler must respond 409 with a stable {"code":"reopen_status_conflict"}
// body so the frontend can branch into the "change status with reason" flow
// without parsing message strings. The errors.As wiring in
// classifyServiceError is what would silently break if someone wrapped the
// error or moved the type — service-level tests alone don't catch that.
func TestCheckIn_ReopenStatusConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*activeModels.WorkSession, error) {
			return nil, &activeSvc.ReopenStatusConflictError{
				SessionID:       42,
				ExistingStatus:  activeModels.WorkSessionStatusPresent,
				RequestedStatus: activeModels.WorkSessionStatusHomeOffice,
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"home_office"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	require.Equal(t, http.StatusConflict, w.Code)

	var resp struct {
		Status  string         `json:"status"`
		Code    string         `json:"code"`
		Error   string         `json:"error"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "reopen_status_conflict", resp.Code,
		"frontend branches on this code via REOPEN_STATUS_CONFLICT_CODE")

	// Details payload drives the reopen-with-status-change modal directly.
	// Without it the frontend would have to look up the session in local
	// history — which fails when the user is viewing a past week and today's
	// session isn't in the fetched range.
	require.NotNil(t, resp.Details, "details must carry the conflicting session id and statuses")
	assert.Equal(t, "42", resp.Details["session_id"],
		"session_id is an int64; serialize as string so the frontend can use it as-is")
	assert.Equal(t, activeModels.WorkSessionStatusPresent, resp.Details["existing_status"])
	assert.Equal(t, activeModels.WorkSessionStatusHomeOffice, resp.Details["requested_status"])
}

func TestCheckIn_PlannedStartNotReached(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*activeModels.WorkSession, error) {
			return nil, &activeSvc.PlannedStartNotReachedError{
				PlannedStartTime: "09:00",
				CurrentTime:      "08:45",
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	require.Equal(t, http.StatusConflict, w.Code)

	var resp struct {
		Status  string         `json:"status"`
		Code    string         `json:"code"`
		Error   string         `json:"error"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "planned_start_not_reached", resp.Code)
	require.NotNil(t, resp.Details)
	assert.Equal(t, "09:00", resp.Details["planned_start_time"])
	assert.Equal(t, "08:45", resp.Details["current_time"])
}

// --- checkOut handler ---

func TestCheckOut_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, staffID int64, _ string) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			ws := &activeModels.WorkSession{}
			ws.ID = 1
			return ws, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckOut_NoActiveSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			return nil, errors.New("no active session found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCheckOut_Unauthorized(t *testing.T) {
	pSvc := &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc, nil)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- getCurrent handler ---

func TestGetCurrent_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, staffID int64) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			ws := &activeModels.WorkSession{Status: "present"}
			ws.ID = 1
			return ws, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCurrent_NoSession(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
			return nil, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCurrent_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getConfig handler ---

func TestGetConfig_EmptyDefault(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	var data ConfigResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "", data.AccountStartDate)
}

func TestGetConfig_ReturnsAccountStartDate(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	rs.SettingsService = newMockSettingsService("2026-08-01", nil)

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	var data ConfigResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "2026-08-01", data.AccountStartDate)
}

func TestGetConfig_AccountStartDateSettingsError(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	rs.SettingsService = newMockSettingsService("", errors.New("settings unavailable"))

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getHistory handler ---

func TestGetHistory_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, staffID int64, from, to timezone.Date) (*activeSvc.HistoryResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "2026-01-01", from.Format("2006-01-02"))
			assert.Equal(t, "2026-01-31", to.Format("2006-01-02"))
			return &activeSvc.HistoryResponse{
				Sessions:        []*activeSvc.SessionResponse{},
				WeeklySummaries: []activeSvc.WeeklySummary{},
			}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_MissingDateParams(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidFromDate(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=bad&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidToDate(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=bad", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, _ int64, _, _ timezone.Date) (*activeSvc.HistoryResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- updateSession handler ---

func TestUpdateSession_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, staffID int64, sessionID int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(42), sessionID)
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"notes":"updated"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateSession_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"notes":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/abc", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSession_InvalidBody(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSession_Forbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, _ int64, _ int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
			return nil, errors.New("can only update own sessions")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"notes":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestUpdateSession_NotesRequiredOnStatusChange locks in the HTTP-boundary
// contract for the service error introduced in Issue #1368: a status flip
// without a reason in `notes` is a client validation failure and must
// surface as HTTP 400, not 500. Without a matching case in
// classifyServiceError the error would fall through to ErrorInternalServer
// and leak the raw message — this test guards the classifier wiring.
func TestUpdateSession_NotesRequiredOnStatusChange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, _ int64, _ int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
			return nil, errors.New("notes required when changing status")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"home_office"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"missing reason on status change must classify as 400, not 500")
}

// --- startBreak handler ---

func TestStartBreak_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, staffID int64, _ *int) (*activeModels.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID)
			return &activeModels.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartBreak_WithPlannedDuration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID)
			require.NotNil(t, plannedDurationMinutes)
			assert.Equal(t, 90, *plannedDurationMinutes)
			return &activeModels.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	durationMinutes := 90
	body, err := json.Marshal(StartBreakRequest{PlannedDurationMinutes: &durationMinutes})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/break/start", bytes.NewReader(body))
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartBreak_AlreadyActive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, _ int64, _ *int) (*activeModels.WorkSessionBreak, error) {
			return nil, errors.New("break already active")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- endBreak handler ---

func TestEndBreak_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, staffID int64) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndBreak_NoActiveBreak(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
			return nil, errors.New("no active break found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- getBreaks handler ---

func TestGetBreaks_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID) // from defaultPersonSvc
			assert.Equal(t, int64(42), sessionID)
			return []*activeModels.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetBreaks_InvalidSessionID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/abc", nil)
	r = withChiParam(r, "sessionId", "abc")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetBreaks_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, _, _ int64) ([]*activeModels.WorkSessionBreak, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getSessionEdits handler ---

func TestGetSessionEdits_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionEditsFn: func(_ context.Context, staffID, sessionID int64) ([]*activeSvc.WorkSessionEditView, error) {
			assert.Equal(t, int64(100), staffID) // from defaultPersonSvc
			assert.Equal(t, int64(42), sessionID)
			return []*activeSvc.WorkSessionEditView{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/42/edits", nil)
	r = withChiParam(r, "id", "42")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSessionEdits_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/abc/edits", nil)
	r = withChiParam(r, "id", "abc")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- exportSessions handler ---

func TestExportSessions_CSV(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, staffID int64, _, _ timezone.Date, format string) ([]byte, string, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "csv", format)
			return []byte("date,time\n"), "export.csv", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31&format=csv", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "export.csv")
}

func TestExportSessions_XLSX(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _ timezone.Date, format string) ([]byte, string, error) {
			assert.Equal(t, "xlsx", format)
			return []byte{0x50, 0x4B}, "export.xlsx", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31&format=xlsx", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "spreadsheetml")
}

func TestExportSessions_DefaultCSV(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _ timezone.Date, format string) ([]byte, string, error) {
			assert.Equal(t, "csv", format)
			return []byte("data"), "export.csv", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportSessions_MissingDates(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportSessions_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _ timezone.Date, _ string) ([]byte, string, error) {
			return nil, "", errors.New("export failed")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- listAbsences handler ---

func TestListAbsences_Success(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, staffID int64, from, to timezone.Date) ([]*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			return []*activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAbsences_MissingDates(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAbsences_ServiceError(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createAbsence handler ---

func TestCreateAbsence_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "sick", req.AbsenceType)
			return &activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"absence_type":"sick","date_start":"2026-01-10","date_end":"2026-01-12"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateAbsence_InvalidBody(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAbsence_Conflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, _ int64, _ activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("absence overlaps with existing absence")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"absence_type":"sick","date_start":"2026-01-10","date_end":"2026-01-12"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- updateAbsence handler ---

func TestUpdateAbsence_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, staffID int64, actorAccountID *int64, absenceID int64, _ activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			require.NotNil(t, actorAccountID)
			assert.Equal(t, int64(validClaims().ID), *actorAccountID)
			assert.Equal(t, int64(77), absenceID)
			return &activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"updated note"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/77", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateAbsence_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/abc", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateAbsence_Forbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, _ int64, _ *int64, _ int64, _ activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("can only update own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/7", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- deleteAbsence handler ---

func TestDeleteAbsence_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, staffID int64, absenceID int64) error {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(77), absenceID)
			return nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/77", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteAbsence_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodDelete, "/absences/abc", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAbsence_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("absence not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteAbsence_Forbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("can only delete own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAbsenceMutations_RollBackWritesOnConflictResponses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	scope := testpkg.NewTenantScope(t, db)
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("platform.schools").Where("id = ?", scope.TenantID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", scope.TenantID).Exec(context.Background())
	})

	profileRepo := repositories.NewFactory(db).GuardianProfile
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/absences", body: `{"absence_type":"sick","date_start":"2026-07-20","date_end":"2026-07-20"}`},
		{name: "update", method: http.MethodPut, path: "/absences/77", body: `{"date_end":"2026-07-21"}`},
		{name: "delete", method: http.MethodDelete, path: "/absences/77"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			email := fmt.Sprintf("absence-rollback-%s-%d@test.local", tc.name, time.Now().UnixNano())
			probe := &userModels.GuardianProfile{
				FirstName:              "Absence",
				LastName:               "Rollback",
				Email:                  &email,
				PreferredContactMethod: "email",
				LanguagePreference:     "de",
			}
			writeThenConflict := func(ctx context.Context) error {
				require.NoError(t, profileRepo.Create(ctx, probe))
				require.Positive(t, probe.ID)
				return fmt.Errorf("sick cascade conflict: %w", scheduleSvc.ErrShiftOverlap)
			}

			absSvc := &mockStaffAbsenceService{}
			switch tc.method {
			case http.MethodPost:
				absSvc.createAbsenceFn = func(ctx context.Context, _ int64, _ activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
					return nil, writeThenConflict(ctx)
				}
			case http.MethodPut:
				absSvc.updateAbsenceFn = func(ctx context.Context, _ int64, _ *int64, _ int64, _ activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
					return nil, writeThenConflict(ctx)
				}
			case http.MethodDelete:
				absSvc.deleteAbsenceFn = func(ctx context.Context, _ int64, _ int64) error {
					return writeThenConflict(ctx)
				}
			}

			rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)
			router := chi.NewRouter()
			router.Use(render.SetContentType(render.ContentTypeJSON))
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					claims := jwt.AppClaims{ID: 101, TenantID: scope.TenantID}
					ctx := context.WithValue(r.Context(), jwt.CtxClaims, claims)
					ctx = tenant.WithTenantID(ctx, scope.TenantID)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Use(tenant.TenantTxMiddleware(db))
			switch tc.method {
			case http.MethodPost:
				router.Post("/absences", rs.createAbsence)
			case http.MethodPut:
				router.Put("/absences/{id}", rs.updateAbsence)
			case http.MethodDelete:
				router.Delete("/absences/{id}", rs.deleteAbsence)
			}

			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusConflict, w.Code)
			_, err := profileRepo.FindByID(scope.Context(), probe.ID)
			assert.ErrorIs(t, err, userModels.ErrGuardianProfileNotFound,
				"conflict responses must roll back writes from the failed absence mutation")
		})
	}
}

// --- getPresenceMap handler ---

func TestGetPresenceMap_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getTodayPresenceFn: func(_ context.Context) (map[int64]string, error) {
			return map[int64]string{1: "present", 2: "home_office"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/presence-map", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getPresenceMap(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPresenceMap_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getTodayPresenceFn: func(_ context.Context) (map[int64]string, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/presence-map", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getPresenceMap(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Error classifier tests ---

func TestClassifyServiceError(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		wantStatus int
	}{
		{"conflict - already checked in", "already checked in", http.StatusConflict},
		{"conflict - already checked out", "already checked out today", http.StatusConflict},
		{"conflict - break active", "break already active", http.StatusConflict},
		{"not found - no active session", "no active session found", http.StatusNotFound},
		{"not found - no session today", "no session found for today", http.StatusNotFound},
		{"not found - session not found", "session not found", http.StatusNotFound},
		{"not found - no active break", "no active break found", http.StatusNotFound},
		{"forbidden - own sessions", "can only update own sessions", http.StatusForbidden},
		{"bad request - status", "status must be present or home_office", http.StatusBadRequest},
		{"bad request - break minutes", "break minutes cannot be negative", http.StatusBadRequest},
		{"bad request - invalid session data", "invalid session data: check-in time must be before check-out time", http.StatusBadRequest},
		{"bad request - admin create time range", "check_out_time must be after check_in_time", http.StatusBadRequest},
		{"internal server - invalid session missing creator", "invalid session data: created_by is required", http.StatusInternalServerError},
		{"internal server - invalid session missing staff", "invalid session data: staff ID is required", http.StatusInternalServerError},
		{"internal server - unknown", "some unknown error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := classifyServiceError(errors.New(tt.errMsg))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			err := render.Render(w, r, renderer)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestClassifyAbsenceError(t *testing.T) {
	tests := []struct {
		name       string
		errMsg     string
		wantStatus int
	}{
		{"not found", "absence not found", http.StatusNotFound},
		{"forbidden - update", "can only update own absences", http.StatusForbidden},
		{"forbidden - delete", "can only delete own absences", http.StatusForbidden},
		{"conflict - overlaps", "absence overlaps with existing", http.StatusConflict},
		{"conflict - updated overlaps", "updated dates overlap with existing", http.StatusConflict},
		{"bad request - invalid", "invalid absence type", http.StatusBadRequest},
		{"bad request - invalid status", "invalid absence status", http.StatusBadRequest},
		{"bad request - invalid prefix", "invalid date format", http.StatusBadRequest},
		{"internal server - unknown", "some unknown error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := classifyAbsenceError(errors.New(tt.errMsg))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			err := render.Render(w, r, renderer)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// --- parseDateRange tests ---

func TestParseDateRange_ValidDates(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01&to=2026-01-31", nil)
	from, to, ok := parseDateRange(w, r)
	assert.True(t, ok)
	assert.Equal(t, "2026-01-01", from.Format("2006-01-02"))
	assert.Equal(t, "2026-01-31", to.Format("2006-01-02"))
}

func TestParseDateRange_MissingFrom(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?to=2026-01-31", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_MissingTo(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_InvalidFromFormat(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=01-01-2026&to=2026-01-31", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_InvalidToFormat(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01&to=31-01-2026", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_BothMissing(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- F9 deviation-reason wire format ---

// The frontend branches on this code via DEVIATION_REASON_REQUIRED_CODE and
// drives the reason dialog from the details payload; both are wire contracts.
func TestCheckIn_DeviationReasonRequired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*activeModels.WorkSession, error) {
			return nil, &activeSvc.DeviationReasonRequiredError{
				Action:           "check_in",
				PlannedTime:      "08:00",
				ActualTime:       "07:30",
				DeviationMinutes: 30,
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	require.Equal(t, http.StatusConflict, w.Code)

	var resp struct {
		Status  string         `json:"status"`
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "deviation_reason_required", resp.Code)
	require.NotNil(t, resp.Details)
	assert.Equal(t, "check_in", resp.Details["action"])
	assert.Equal(t, "08:00", resp.Details["planned_time"])
	assert.Equal(t, "07:30", resp.Details["actual_time"])
	assert.Equal(t, "30", resp.Details["deviation_minutes"],
		"minutes are serialized as string, consistent with the reopen-conflict details")
}

func TestCheckOut_DeviationReasonRequired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			return nil, &activeSvc.DeviationReasonRequiredError{
				Action:           "check_out",
				PlannedTime:      "16:00",
				ActualTime:       "16:30",
				DeviationMinutes: 30,
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	require.Equal(t, http.StatusConflict, w.Code)

	var resp struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "deviation_reason_required", resp.Code)
	require.NotNil(t, resp.Details)
	assert.Equal(t, "check_out", resp.Details["action"])
	assert.Equal(t, "16:00", resp.Details["planned_time"])
	assert.Equal(t, "16:30", resp.Details["actual_time"])
	assert.Equal(t, "30", resp.Details["deviation_minutes"])
}

func TestCheckIn_ForwardsReason(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var gotReason string
	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, reason string) (*activeModels.WorkSession, error) {
			gotReason = reason
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present","reason":"Frühdienst übernommen"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Frühdienst übernommen", gotReason)
}

func TestCheckOut_ForwardsReason(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var gotReason string
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, reason string) (*activeModels.WorkSession, error) {
			gotReason = reason
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"reason":"Elterngespräch lief länger"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-out", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Elterngespräch lief länger", gotReason)
}

func TestCheckOut_MalformedBodyRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	called := false
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"reason":`)
	r := httptest.NewRequest(http.MethodPost, "/check-out", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, called, "a malformed body must not reach the service")
}
