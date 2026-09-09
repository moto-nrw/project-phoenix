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
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Mock workforce.WorkSessions ---

type mockWorkSessionService struct {
	checkInFn           func(ctx context.Context, staffID int64, status, source, reason string) (*workforce.WorkSession, error)
	checkOutFn          func(ctx context.Context, staffID int64, reason string) (*workforce.WorkSession, error)
	startBreakFn        func(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*workforce.WorkSessionBreak, error)
	endBreakFn          func(ctx context.Context, staffID int64) (*workforce.WorkSession, error)
	getSessionBreaksFn  func(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionBreak, error)
	updateSessionFn     func(ctx context.Context, staffID int64, sessionID int64, updates workforce.SessionUpdateRequest) (*workforce.WorkSession, error)
	getCurrentSessionFn func(ctx context.Context, staffID int64) (*workforce.WorkSession, error)
	getHistoryFn        func(ctx context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error)
	// Set only where a test needs to tell the two history reads apart; the
	// default keeps both returning the same fixture.
	getHistoryIntersectingFn func(ctx context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error)
	getSessionEditsFn        func(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error)
	getTodayPresenceFn       func(ctx context.Context) (map[int64]string, error)
	exportSessionsFn         func(ctx context.Context, staffID int64, from, to, format string) (*workforce.ExportFile, error)
}

func (m *mockWorkSessionService) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*workforce.WorkSession, error) {
	if m.checkInFn != nil {
		return m.checkInFn(ctx, staffID, status, source, reason)
	}
	return &workforce.WorkSession{}, nil
}
func (m *mockWorkSessionService) CheckOut(ctx context.Context, staffID int64, reason string) (*workforce.WorkSession, error) {
	if m.checkOutFn != nil {
		return m.checkOutFn(ctx, staffID, reason)
	}
	return &workforce.WorkSession{}, nil
}
func (m *mockWorkSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*workforce.WorkSessionBreak, error) {
	if m.startBreakFn != nil {
		return m.startBreakFn(ctx, staffID, plannedDurationMinutes)
	}
	return &workforce.WorkSessionBreak{}, nil
}
func (m *mockWorkSessionService) EndBreak(ctx context.Context, staffID int64) (*workforce.WorkSession, error) {
	if m.endBreakFn != nil {
		return m.endBreakFn(ctx, staffID)
	}
	return &workforce.WorkSession{}, nil
}
func (m *mockWorkSessionService) LatestOpenSession(ctx context.Context, staffID int64) (*workforce.WorkSession, error) {
	if m.getCurrentSessionFn != nil {
		return m.getCurrentSessionFn(ctx, staffID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) SessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionBreak, error) {
	if m.getSessionBreaksFn != nil {
		return m.getSessionBreaksFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
	if m.updateSessionFn != nil {
		return m.updateSessionFn(ctx, staffID, sessionID, updates)
	}
	return &workforce.WorkSession{}, nil
}
func (m *mockWorkSessionService) UpdateSessionAsAdmin(_ context.Context, _, _, _ int64, _ workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
	return nil, nil
}
func (m *mockWorkSessionService) CreateSessionAsAdmin(_ context.Context, _, _ int64, _ workforce.AdminCreateSessionRequest) (*workforce.WorkSession, error) {
	return nil, nil
}

// HistoryIntersecting is the only history read the contract exposes: the
// handler must read by interval, never by stored date (#2402). getHistoryFn
// stays the shared default so tests that do not care keep one hook.
func (m *mockWorkSessionService) HistoryIntersecting(ctx context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error) {
	if m.getHistoryIntersectingFn != nil {
		return m.getHistoryIntersectingFn(ctx, staffID, from, to)
	}
	if m.getHistoryFn != nil {
		return m.getHistoryFn(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockWorkSessionService) SessionEdits(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error) {
	if m.getSessionEditsFn != nil {
		return m.getSessionEditsFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) SessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error) {
	if m.getSessionEditsFn != nil {
		return m.getSessionEditsFn(ctx, staffID, sessionID)
	}
	return nil, nil
}
func (m *mockWorkSessionService) TodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	if m.getTodayPresenceFn != nil {
		return m.getTodayPresenceFn(ctx)
	}
	return map[int64]string{}, nil
}
func (m *mockWorkSessionService) ExportSessions(ctx context.Context, staffID int64, from, to, format string) (*workforce.ExportFile, error) {
	if m.exportSessionsFn != nil {
		return m.exportSessionsFn(ctx, staffID, from, to, format)
	}
	return &workforce.ExportFile{Data: []byte("data"), Filename: "export.csv", ContentType: "text/csv; charset=utf-8"}, nil
}
func (m *mockWorkSessionService) UpdateStaffSchedule(_ context.Context, _ int64, _ workforce.ScheduleUpdateInput) error {
	return nil
}

// --- Mock workforce.StaffAbsences ---

type mockStaffAbsenceService struct {
	createAbsenceFn     func(ctx context.Context, staffID int64, req workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	updateAbsenceFn     func(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error)
	deleteAbsenceFn     func(ctx context.Context, staffID int64, absenceID int64) error
	getAbsencesForRange func(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffAbsenceResponse, error)
	listAbsencesFn      func(ctx context.Context, staffID int64, filter workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error)
	resubmitAbsenceFn   func(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*workforce.StaffAbsenceResponse, error)
}

func (m *mockStaffAbsenceService) createAbsence(ctx context.Context, staffID int64, req workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	if m.createAbsenceFn != nil {
		return m.createAbsenceFn(ctx, staffID, req)
	}
	return &workforce.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) CreateOwnAbsence(ctx context.Context, staffID int64, _ *int64, req workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return m.createAbsence(ctx, staffID, req)
}

// The #1843 *For variants delegate to the existing fn fields so the tests
// written against createAbsenceFn/deleteAbsenceFn keep exercising their hooks.
func (m *mockStaffAbsenceService) CreateAbsenceFor(ctx context.Context, subjectStaffID, _ int64, _ *int64, req workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	return m.createAbsence(ctx, subjectStaffID, req)
}
func (m *mockStaffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
	if m.updateAbsenceFn != nil {
		return m.updateAbsenceFn(ctx, staffID, actorAccountID, absenceID, req)
	}
	return &workforce.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) deleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	if m.deleteAbsenceFn != nil {
		return m.deleteAbsenceFn(ctx, staffID, absenceID)
	}
	return nil
}
func (m *mockStaffAbsenceService) DeleteOwnAbsence(ctx context.Context, staffID int64, _ *int64, absenceID int64) error {
	return m.deleteAbsence(ctx, staffID, absenceID)
}
func (m *mockStaffAbsenceService) DeleteAbsenceFor(ctx context.Context, subjectStaffID, _ int64, _ *int64, absenceID int64) error {
	return m.deleteAbsence(ctx, subjectStaffID, absenceID)
}
func (m *mockStaffAbsenceService) PreviewCompTimeBalance(_ context.Context, _ int64, _, _ string, _ bool) (*workforce.CompTimeBalancePreview, error) {
	return &workforce.CompTimeBalancePreview{}, nil
}
func (m *mockStaffAbsenceService) AbsencesForRange(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffAbsenceResponse, error) {
	if m.getAbsencesForRange != nil {
		return m.getAbsencesForRange(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockStaffAbsenceService) ListAbsences(ctx context.Context, staffID int64, filter workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error) {
	if m.listAbsencesFn != nil {
		return m.listAbsencesFn(ctx, staffID, filter)
	}
	if m.getAbsencesForRange != nil && filter.From != "" && filter.To != "" && filter.Status == "" {
		return m.getAbsencesForRange(ctx, staffID, filter.From, filter.To)
	}
	return nil, nil
}

// Vacation workflow methods (Tranche 4): no-op defaults so MA-side tests
// satisfy the StaffAbsences contract without exercising the flow.
func (m *mockStaffAbsenceService) RequestVacation(_ context.Context, _ int64, _ workforce.RequestVacationRequest) (*workforce.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) ApproveAbsence(_ context.Context, _ int64, _ int64, _ int64, _ string) (*workforce.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) DenyAbsence(_ context.Context, _ int64, _ int64, _ int64, _ string) (*workforce.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) QuestionAbsence(_ context.Context, _ int64, _ int64, _ string) (*workforce.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) ResubmitAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	if m.resubmitAbsenceFn != nil {
		return m.resubmitAbsenceFn(ctx, staffID, actorAccountID, absenceID, note)
	}
	return &workforce.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) CancelAbsence(_ context.Context, _ int64, _ int64, _ int64) error {
	return nil
}
func (m *mockStaffAbsenceService) VacationQuotaSummary(_ context.Context, _ int64, _ int) (*workforce.VacationQuotaSummary, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) UpsertVacationQuota(_ context.Context, _ int64, _ int, _, _ float64) error {
	return nil
}

// Vacation takeover (#2132). The staff-facing time-tracking API never calls
// these — the takeover is admin-only — so the mock just satisfies the
// contract.
func (m *mockStaffAbsenceService) SetVacationOpening(_ context.Context, _, _ int64, _ workforce.SetVacationOpeningRequest) (*workforce.StaffVacationOpening, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) DeleteVacationOpening(_ context.Context, _, _ int64, _ int) error {
	return nil
}
func (m *mockStaffAbsenceService) ListPendingRequests(_ context.Context) ([]*workforce.StaffAbsenceResponse, error) {
	return nil, nil
}
func (m *mockStaffAbsenceService) ListAbsenceRequests(_ context.Context, _ workforce.AbsenceRequestListQuery) ([]*workforce.StaffAbsenceRequestItem, error) {
	return nil, nil
}

// --- Mock workforce.StaffDirectory ---

// stubStaffDirectory is the package-local double for workforce.StaffDirectory.
// Only the lookups the MA-facing resource performs are wired; the embedded
// contract keeps the stub honest when the interface grows.
type stubStaffDirectory struct {
	workforce.StaffDirectory
	personByAccountIDFn func(context.Context, int64) (*workforce.Person, error)
	staffByPersonIDFn   func(context.Context, int64) (*workforce.StaffProfile, error)
	staffByIDFn         func(context.Context, int64) (*workforce.StaffProfile, error)
}

func (s *stubStaffDirectory) PersonByAccountID(ctx context.Context, accountID int64) (*workforce.Person, error) {
	if s.personByAccountIDFn == nil {
		return &workforce.Person{ID: accountID}, nil
	}
	return s.personByAccountIDFn(ctx, accountID)
}

func (s *stubStaffDirectory) StaffByPersonID(ctx context.Context, personID int64) (*workforce.StaffProfile, error) {
	if s.staffByPersonIDFn == nil {
		return &workforce.StaffProfile{ID: personID, PersonID: personID}, nil
	}
	return s.staffByPersonIDFn(ctx, personID)
}

func (s *stubStaffDirectory) StaffByID(ctx context.Context, staffID int64) (*workforce.StaffProfile, error) {
	if s.staffByIDFn == nil {
		return &workforce.StaffProfile{ID: staffID}, nil
	}
	return s.staffByIDFn(ctx, staffID)
}

// --- Test helpers ---

// testAccountID is the caller the test resource's identity resolves to; it is
// what the composition root would derive from the session claims.
const testAccountID int64 = 1

func defaultPersonSvc() *stubStaffDirectory {
	return &stubStaffDirectory{
		personByAccountIDFn: func(_ context.Context, _ int64) (*workforce.Person, error) {
			return &workforce.Person{ID: 10}, nil
		},
		staffByPersonIDFn: func(_ context.Context, _ int64) (*workforce.StaffProfile, error) {
			return &workforce.StaffProfile{ID: 100, PersonID: 10}, nil
		},
	}
}

func testResource(wsSvc *mockWorkSessionService, absSvc *mockStaffAbsenceService, pSvc *stubStaffDirectory, db *bun.DB) *Resource {
	return &Resource{
		WorkSessionService:  wsSvc,
		StaffAbsenceService: absSvc,
		PersonService:       pSvc,
		identity:            func(context.Context) Identity { return Identity{AccountID: testAccountID} },
		db:                  db,
	}
}

// withCaller puts the request into the package's tenant runtime and scopes it
// to tenantID; the caller itself comes from the resource's identity func.
func withCaller(r *http.Request, tenantID int64) *http.Request {
	ctx := testpkg.WithPackageTenantRuntime(r.Context())
	if tenantID != 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
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
	_ workforce.WorkSessions   = (*mockWorkSessionService)(nil)
	_ workforce.StaffAbsences  = (*mockStaffAbsenceService)(nil)
	_ workforce.StaffDirectory = (*stubStaffDirectory)(nil)
)

// --- CheckInRequest.Bind ---

func TestCheckInRequest_Bind(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	rs := NewResource(Dependencies{
		WorkSessions:     &mockWorkSessionService{},
		StaffAbsences:    &mockStaffAbsenceService{},
		Staff:            defaultPersonSvc(),
		AccountStartDate: func(context.Context) (string, error) { return "", nil },
		Identity:         func(context.Context) Identity { return Identity{AccountID: testAccountID} },
		DB:               db,
	})
	assert.NotNil(t, rs)
	assert.NotNil(t, rs.WorkSessionService)
	assert.NotNil(t, rs.StaffAbsenceService)
	assert.NotNil(t, rs.PersonService)
	assert.NotNil(t, rs.accountStartDate)
}

// --- Router ---

func TestRouter(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	router := rs.Router()
	assert.NotNil(t, router)
}

// --- getStaffIDFromClaims ---

func TestGetStaffIDFromClaims_Success(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	staffID, err := rs.getStaffIDFromClaims(context.Background(), Identity{AccountID: testAccountID})
	require.NoError(t, err)
	assert.Equal(t, int64(100), staffID)
}

func TestGetStaffIDFromClaims_ZeroID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	_, err := rs.getStaffIDFromClaims(context.Background(), Identity{AccountID: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestGetStaffIDFromClaims_PersonNotFound(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, personNotFoundSvc(), nil)
	_, err := rs.getStaffIDFromClaims(context.Background(), Identity{AccountID: testAccountID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "person not found")
}

func personNotFoundSvc() *stubStaffDirectory {
	return &stubStaffDirectory{
		personByAccountIDFn: func(_ context.Context, _ int64) (*workforce.Person, error) {
			return nil, errors.New("not found")
		},
	}
}

func TestGetStaffIDFromClaims_StaffNotFound(t *testing.T) {
	t.Parallel()

	pSvc := &stubStaffDirectory{
		personByAccountIDFn: func(_ context.Context, _ int64) (*workforce.Person, error) {
			return &workforce.Person{ID: 10}, nil
		},
		staffByPersonIDFn: func(_ context.Context, _ int64) (*workforce.StaffProfile, error) {
			return nil, errors.New("not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc, nil)
	_, err := rs.getStaffIDFromClaims(context.Background(), Identity{AccountID: testAccountID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staff record not found")
}

// --- checkIn handler ---

func TestCheckIn_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, staffID int64, status, source, _ string) (*workforce.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "present", status)
			assert.Equal(t, workforce.WorkSessionSourceApp, source)
			return &workforce.WorkSession{ID: 1, Status: "present", Source: source}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	assert.Equal(t, "success", resp.Status)
}

func TestCheckIn_InvalidBody(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"status":"invalid"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckIn_InvalidClaims(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, personNotFoundSvc(), nil)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckIn_ServiceConflict(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*workforce.WorkSession, error) {
			return nil, errors.New("already checked in")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCheckIn_PlannedStartNotReached(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*workforce.WorkSession, error) {
			return nil, &workforce.PlannedStartNotReachedError{
				PlannedStartTime: "09:00",
				CurrentTime:      "08:45",
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, staffID int64, _ string) (*workforce.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			return &workforce.WorkSession{ID: 1}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckOut_NoActiveSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*workforce.WorkSession, error) {
			return nil, errors.New("no active session found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCheckOut_Unauthorized(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, personNotFoundSvc(), nil)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- getCurrent handler ---

func TestGetCurrent_Success(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, staffID int64) (*workforce.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			return &workforce.WorkSession{ID: 1, Status: "present"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCurrent_NoSession(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, _ int64) (*workforce.WorkSession, error) {
			return nil, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCurrent_ServiceError(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getCurrentSessionFn: func(_ context.Context, _ int64) (*workforce.WorkSession, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getConfig handler ---

func TestGetConfig_EmptyDefault(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	var data ConfigResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "", data.AccountStartDate)
}

func TestGetConfig_ReturnsAccountStartDate(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	rs.accountStartDate = func(context.Context) (string, error) { return "2026-08-01", nil }

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseAPIResponse(t, w)
	var data ConfigResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, "2026-08-01", data.AccountStartDate)
}

func TestGetConfig_AccountStartDateSettingsError(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)
	rs.accountStartDate = func(context.Context) (string, error) {
		return "", errors.New("settings unavailable")
	}

	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getConfig(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getHistory handler ---

func TestGetHistory_Success(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "2026-01-01", from)
			assert.Equal(t, "2026-01-31", to)
			return &workforce.HistoryResponse{
				Sessions:        []*workforce.SessionResponse{},
				WeeklySummaries: []workforce.WorkWeekSummary{},
			}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

// The table lays a block's minutes out on the Berlin days they fall on, so a
// block that started the evening before `from` still belongs to the range
// (#2402). The handler therefore reads by interval, not by stored date — the
// contract exposes no by-stored-date read at all, and the mock fails the test
// if the handler ever falls back to the shared default hook.
func TestGetHistory_ReadsBlocksIntersectingTheRange(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, _ int64, _, _ string) (*workforce.HistoryResponse, error) {
			return nil, errors.New("history must be read by interval, not by stored date")
		},
		getHistoryIntersectingFn: func(_ context.Context, staffID int64, from, to string) (*workforce.HistoryResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "2026-01-01", from)
			assert.Equal(t, "2026-01-31", to)
			return &workforce.HistoryResponse{
				Sessions:        []*workforce.SessionResponse{},
				WeeklySummaries: []workforce.WorkWeekSummary{},
			}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_MissingDateParams(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidFromDate(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=bad&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidToDate(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=bad", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_ServiceError(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, _ int64, _, _ string) (*workforce.HistoryResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- updateSession handler ---

func TestUpdateSession_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, staffID int64, sessionID int64, _ workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(42), sessionID)
			return &workforce.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"notes":"updated"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateSession_InvalidID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"notes":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/abc", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSession_InvalidBody(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSession_Forbidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, _ int64, _ int64, _ workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
			return nil, errors.New("can only update own sessions")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"notes":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, _ int64, _ int64, _ workforce.SessionUpdateRequest) (*workforce.WorkSession, error) {
			return nil, errors.New("notes required when changing status")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"home_office"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"missing reason on status change must classify as 400, not 500")
}

// --- startBreak handler ---

func TestStartBreak_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, staffID int64, _ *int) (*workforce.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID)
			return &workforce.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartBreak_WithPlannedDuration(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, staffID int64, plannedDurationMinutes *int) (*workforce.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID)
			require.NotNil(t, plannedDurationMinutes)
			assert.Equal(t, 90, *plannedDurationMinutes)
			return &workforce.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	durationMinutes := 90
	body, err := json.Marshal(StartBreakRequest{PlannedDurationMinutes: &durationMinutes})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/break/start", bytes.NewReader(body))
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartBreak_AlreadyActive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, _ int64, _ *int) (*workforce.WorkSessionBreak, error) {
			return nil, errors.New("break already active")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- endBreak handler ---

func TestEndBreak_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, staffID int64) (*workforce.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			return &workforce.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndBreak_NoActiveBreak(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, _ int64) (*workforce.WorkSession, error) {
			return nil, errors.New("no active break found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- getBreaks handler ---

func TestGetBreaks_Success(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID) // from defaultPersonSvc
			assert.Equal(t, int64(42), sessionID)
			return []*workforce.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetBreaks_InvalidSessionID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/abc", nil)
	r = withChiParam(r, "sessionId", "abc")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetBreaks_ServiceError(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, _, _ int64) ([]*workforce.WorkSessionBreak, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getSessionEdits handler ---

func TestGetSessionEdits_Success(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getSessionEditsFn: func(_ context.Context, staffID, sessionID int64) ([]*workforce.WorkSessionEditView, error) {
			assert.Equal(t, int64(100), staffID) // from defaultPersonSvc
			assert.Equal(t, int64(42), sessionID)
			return []*workforce.WorkSessionEditView{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/42/edits", nil)
	r = withChiParam(r, "id", "42")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSessionEdits_InvalidID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/abc/edits", nil)
	r = withChiParam(r, "id", "abc")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- exportSessions handler ---

func TestExportSessions_CSV(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, staffID int64, _, _, format string) (*workforce.ExportFile, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "csv", format)
			return &workforce.ExportFile{Data: []byte("date,time\n"), Filename: "export.csv", ContentType: "text/csv; charset=utf-8"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31&format=csv", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "export.csv")
}

func TestExportSessions_XLSX(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _, format string) (*workforce.ExportFile, error) {
			assert.Equal(t, "xlsx", format)
			return &workforce.ExportFile{Data: []byte{0x50, 0x4B}, Filename: "export.xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31&format=xlsx", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "spreadsheetml")
}

func TestExportSessions_DefaultCSV(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _, format string) (*workforce.ExportFile, error) {
			assert.Equal(t, "csv", format)
			return &workforce.ExportFile{Data: []byte("data"), Filename: "export.csv", ContentType: "text/csv; charset=utf-8"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportSessions_MissingDates(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportSessions_ServiceError(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _, _ string) (*workforce.ExportFile, error) {
			return nil, errors.New("export failed")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- listAbsences handler ---

func TestListAbsences_Success(t *testing.T) {
	t.Parallel()

	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, staffID int64, from, to string) ([]*workforce.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			return []*workforce.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAbsences_MissingDates(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAbsences_StatusOnly(t *testing.T) {
	t.Parallel()

	absSvc := &mockStaffAbsenceService{
		listAbsencesFn: func(_ context.Context, staffID int64, filter workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Empty(t, filter.From)
			assert.Empty(t, filter.To)
			assert.Equal(t, workforce.AbsenceStatusQuestion, filter.Status)
			return []*workforce.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?status=question", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAbsences_IncompleteDateRange(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&status=question", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAbsences_InvalidStatus(t *testing.T) {
	t.Parallel()

	absSvc := &mockStaffAbsenceService{
		listAbsencesFn: func(_ context.Context, _ int64, _ workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("invalid absence status")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?status=unknown", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAbsences_ServiceError(t *testing.T) {
	t.Parallel()

	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, _ int64, _, _ string) ([]*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createAbsence handler ---

func TestCreateAbsence_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, staffID int64, req workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "sick", req.AbsenceType)
			return &workforce.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"absence_type":"sick","date_start":"2026-01-10","date_end":"2026-01-12"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateAbsence_InvalidBody(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAbsence_Conflict(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, _ int64, _ workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("absence overlaps with existing absence")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"absence_type":"sick","date_start":"2026-01-10","date_end":"2026-01-12"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- updateAbsence handler ---

func TestUpdateAbsence_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, staffID int64, actorAccountID *int64, absenceID int64, _ workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			require.NotNil(t, actorAccountID)
			assert.Equal(t, testAccountID, *actorAccountID)
			assert.Equal(t, int64(77), absenceID)
			return &workforce.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"updated note"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/77", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateAbsence_InvalidID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/abc", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateAbsence_Forbidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, _ int64, _ *int64, _ int64, _ workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("can only update own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/7", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- deleteAbsence handler ---

func TestDeleteAbsence_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, staffID int64, absenceID int64) error {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(77), absenceID)
			return nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/77", nil)
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteAbsence_InvalidID(t *testing.T) {
	t.Parallel()

	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodDelete, "/absences/abc", nil)
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAbsence_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("absence not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteAbsence_Forbidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("can only delete own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAbsenceMutations_RollBackWritesOnConflictResponses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)

	profileRepo := repositories.NewGuardianProfileTestRepository(db)
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
			// A committed row of the same shape is the only model-free way to
			// obtain a writable probe here; the copy written inside the failed
			// mutation is the one whose rollback this test observes.
			probe := testpkg.CreateTestGuardianProfileForTenant(t, db, scope.TenantID, "Absence", "Rollback",
				fmt.Sprintf("absence-rollback-%s-%d", tc.name, time.Now().UnixNano()))
			probe.ID = 0
			email := fmt.Sprintf("absence-rollback-probe-%s-%d@test.local", tc.name, time.Now().UnixNano())
			probe.Email = &email

			writeThenConflict := func(ctx context.Context) error {
				require.NoError(t, profileRepo.Create(ctx, probe))
				require.Positive(t, probe.ID)
				return fmt.Errorf("sick cascade conflict: %w", workforce.ErrStaffShiftOverlap)
			}

			absSvc := &mockStaffAbsenceService{}
			switch tc.method {
			case http.MethodPost:
				absSvc.createAbsenceFn = func(ctx context.Context, _ int64, _ workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
					return nil, writeThenConflict(ctx)
				}
			case http.MethodPut:
				absSvc.updateAbsenceFn = func(ctx context.Context, _ int64, _ *int64, _ int64, _ workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, error) {
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
					ctx := tenant.WithTenantID(r.Context(), scope.TenantID)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Use(testpkg.TenantTxMiddleware(db))
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
			assert.Error(t, err, "the rolled-back probe row must be gone")
			count, countErr := db.NewSelect().Table("users.guardian_profiles").
				Where("id = ?", probe.ID).Count(scope.Context())
			require.NoError(t, countErr)
			assert.Zero(t, count,
				"conflict responses must roll back writes from the failed absence mutation")
		})
	}
}

// --- getPresenceMap handler ---

func TestGetPresenceMap_Success(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getTodayPresenceFn: func(_ context.Context) (map[int64]string, error) {
			return map[int64]string{1: "present", 2: "home_office"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/presence-map", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getPresenceMap(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPresenceMap_ServiceError(t *testing.T) {
	t.Parallel()

	wsSvc := &mockWorkSessionService{
		getTodayPresenceFn: func(_ context.Context) (map[int64]string, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), nil)

	r := httptest.NewRequest(http.MethodGet, "/presence-map", nil)
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.getPresenceMap(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Error classifier tests ---

func TestClassifyServiceError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

	t.Run("manager-controlled absence returns coded forbidden", func(t *testing.T) {
		renderer := classifyAbsenceError(workforce.ErrManagerControlledAbsence)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		err := render.Render(w, r, renderer)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, `{"status":"error","error":"absence type is manager-controlled","code":"manager_controlled_absence"}`, w.Body.String())
	})

}

// --- parseDateRange tests ---

func TestParseDateRange_ValidDates(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01&to=2026-01-31", nil)
	from, to, ok := parseDateRange(w, r)
	assert.True(t, ok)
	assert.Equal(t, "2026-01-01", from.String())
	assert.Equal(t, "2026-01-31", to.String())
}

func TestParseDateRange_MissingFrom(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?to=2026-01-31", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_MissingTo(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_InvalidFromFormat(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=01-01-2026&to=2026-01-31", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_InvalidToFormat(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01&to=31-01-2026", nil)
	_, _, ok := parseDateRange(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDateRange_BothMissing(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, _ string) (*workforce.WorkSession, error) {
			return nil, &workforce.DeviationReasonRequiredError{
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
	r = withCaller(r, testpkg.Tenant(t))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*workforce.WorkSession, error) {
			return nil, &workforce.DeviationReasonRequiredError{
				Action:           "check_out",
				PlannedTime:      "16:00",
				ActualTime:       "16:30",
				DeviationMinutes: 30,
			}
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withCaller(r, testpkg.Tenant(t))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	var gotReason string
	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _, _, reason string) (*workforce.WorkSession, error) {
			gotReason = reason
			return &workforce.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"status":"present","reason":"Frühdienst übernommen"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Frühdienst übernommen", gotReason)
}

func TestCheckOut_ForwardsReason(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	var gotReason string
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, reason string) (*workforce.WorkSession, error) {
			gotReason = reason
			return &workforce.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"reason":"Elterngespräch lief länger"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-out", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Elterngespräch lief länger", gotReason)
}

func TestCheckOut_MalformedBodyRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	called := false
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64, _ string) (*workforce.WorkSession, error) {
			called = true
			return &workforce.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"reason":`)
	r := httptest.NewRequest(http.MethodPost, "/check-out", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, called, "a malformed body must not reach the service")
}

// --- resubmitAbsence (#1419 Rückfrage answer) ---

func TestResubmitAbsence_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		resubmitAbsenceFn: func(_ context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*workforce.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, testAccountID, actorAccountID)
			assert.Equal(t, int64(77), absenceID)
			assert.Equal(t, "Vertretung geklärt", note)
			return &workforce.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"Vertretung geklärt"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences/77/resubmit", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.resubmitAbsence(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResubmitAbsence_NotOwn(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		resubmitAbsenceFn: func(_ context.Context, _ int64, _ int64, _ int64, _ string) (*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("can only resubmit own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences/77/resubmit", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.resubmitAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestResubmitAbsence_WrongStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	absSvc := &mockStaffAbsenceService{
		resubmitAbsenceFn: func(_ context.Context, _ int64, _ int64, _ int64, _ string) (*workforce.StaffAbsenceResponse, error) {
			return nil, errors.New("only absences with a question can be resubmitted")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc(), db)

	body := bytes.NewBufferString(`{"note":"x"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences/77/resubmit", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCaller(r, testpkg.Tenant(t))
	r = withChiParam(r, "id", "77")
	w := httptest.NewRecorder()

	rs.resubmitAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
