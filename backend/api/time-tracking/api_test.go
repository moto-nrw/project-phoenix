package timetracking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- Mock StaffRepository ---

type mockStaffRepo struct {
	findByPersonIDFn func(ctx context.Context, personID int64) (*userModels.Staff, error)
}

func (m *mockStaffRepo) Create(_ context.Context, _ *userModels.Staff) error { return nil }
func (m *mockStaffRepo) FindByID(_ context.Context, _ any) (*userModels.Staff, error) {
	return nil, nil
}
func (m *mockStaffRepo) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	if m.findByPersonIDFn != nil {
		return m.findByPersonIDFn(ctx, personID)
	}
	return &userModels.Staff{}, nil
}
func (m *mockStaffRepo) Update(_ context.Context, _ *userModels.Staff) error { return nil }
func (m *mockStaffRepo) Delete(_ context.Context, _ any) error               { return nil }
func (m *mockStaffRepo) List(_ context.Context, _ map[string]any) ([]*userModels.Staff, error) {
	return nil, nil
}
func (m *mockStaffRepo) ListAllWithPerson(_ context.Context) ([]*userModels.Staff, error) {
	return nil, nil
}
func (m *mockStaffRepo) UpdateNotes(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockStaffRepo) FindWithPerson(_ context.Context, _ int64) (*userModels.Staff, error) {
	return nil, nil
}

// --- Mock PersonService ---

type mockPersonService struct {
	findByAccountIDFn func(ctx context.Context, accountID int64) (*userModels.Person, error)
	staffRepo         *mockStaffRepo
}

func (m *mockPersonService) WithTx(_ bun.Tx) interface{} { return m }
func (m *mockPersonService) Get(_ context.Context, _ any) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) GetByIDs(_ context.Context, _ []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) Create(_ context.Context, _ *userModels.Person) error { return nil }
func (m *mockPersonService) Update(_ context.Context, _ *userModels.Person) error { return nil }
func (m *mockPersonService) Delete(_ context.Context, _ any) error                { return nil }
func (m *mockPersonService) List(_ context.Context, _ *base.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByTagID(_ context.Context, _ string) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	if m.findByAccountIDFn != nil {
		return m.findByAccountIDFn(ctx, accountID)
	}
	return &userModels.Person{}, nil
}
func (m *mockPersonService) FindByName(_ context.Context, _, _ string) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) LinkToAccount(_ context.Context, _ int64, _ int64) error   { return nil }
func (m *mockPersonService) UnlinkFromAccount(_ context.Context, _ int64) error        { return nil }
func (m *mockPersonService) LinkToRFIDCard(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockPersonService) UnlinkFromRFIDCard(_ context.Context, _ int64) error       { return nil }
func (m *mockPersonService) GetFullProfile(_ context.Context, _ int64) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByGuardianID(_ context.Context, _ int64) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonService) StaffRepository() userModels.StaffRepository     { return m.staffRepo }
func (m *mockPersonService) TeacherRepository() userModels.TeacherRepository { return nil }
func (m *mockPersonService) StudentRepository() userModels.StudentRepository { return nil }
func (m *mockPersonService) ListAvailableRFIDCards(_ context.Context) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (m *mockPersonService) ValidateStaffPIN(_ context.Context, _ string) (*userModels.Staff, error) {
	return nil, nil
}
func (m *mockPersonService) ValidateStaffPINForSpecificStaff(_ context.Context, _ int64, _ string) (*userModels.Staff, error) {
	return nil, nil
}
func (m *mockPersonService) GetStudentsByTeacher(_ context.Context, _ int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (m *mockPersonService) GetStudentsWithGroupsByTeacher(_ context.Context, _ int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}

// --- Mock WorkSessionService ---

type mockWorkSessionService struct {
	checkInFn           func(ctx context.Context, staffID int64, status string) (*activeModels.WorkSession, error)
	checkOutFn          func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	startBreakFn        func(ctx context.Context, staffID int64) (*activeModels.WorkSessionBreak, error)
	endBreakFn          func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getSessionBreaksFn  func(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	updateSessionFn     func(ctx context.Context, staffID int64, sessionID int64, updates activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error)
	getCurrentSessionFn func(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	getHistoryFn        func(ctx context.Context, staffID int64, from, to time.Time) ([]*activeSvc.SessionResponse, error)
	getSessionEditsFn   func(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error)
	getTodayPresenceFn  func(ctx context.Context) (map[int64]string, error)
	exportSessionsFn    func(ctx context.Context, staffID int64, from, to time.Time, format string) ([]byte, string, error)
}

func (m *mockWorkSessionService) CheckIn(ctx context.Context, staffID int64, status string) (*activeModels.WorkSession, error) {
	if m.checkInFn != nil {
		return m.checkInFn(ctx, staffID, status)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.checkOutFn != nil {
		return m.checkOutFn(ctx, staffID)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) StartBreak(ctx context.Context, staffID int64) (*activeModels.WorkSessionBreak, error) {
	if m.startBreakFn != nil {
		return m.startBreakFn(ctx, staffID)
	}
	return &activeModels.WorkSessionBreak{}, nil
}
func (m *mockWorkSessionService) EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	if m.endBreakFn != nil {
		return m.endBreakFn(ctx, staffID)
	}
	return &activeModels.WorkSession{}, nil
}
func (m *mockWorkSessionService) GetSessionBreaks(ctx context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	if m.getSessionBreaksFn != nil {
		return m.getSessionBreaksFn(ctx, sessionID)
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
func (m *mockWorkSessionService) GetHistory(ctx context.Context, staffID int64, from, to time.Time) ([]*activeSvc.SessionResponse, error) {
	if m.getHistoryFn != nil {
		return m.getHistoryFn(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockWorkSessionService) GetSessionEdits(ctx context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error) {
	if m.getSessionEditsFn != nil {
		return m.getSessionEditsFn(ctx, sessionID)
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
func (m *mockWorkSessionService) EnsureCheckedIn(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (m *mockWorkSessionService) ExportSessions(ctx context.Context, staffID int64, from, to time.Time, format string) ([]byte, string, error) {
	if m.exportSessionsFn != nil {
		return m.exportSessionsFn(ctx, staffID, from, to, format)
	}
	return []byte("data"), "export.csv", nil
}

// --- Mock StaffAbsenceService ---

type mockStaffAbsenceService struct {
	createAbsenceFn     func(ctx context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error)
	updateAbsenceFn     func(ctx context.Context, staffID int64, absenceID int64, req activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error)
	deleteAbsenceFn     func(ctx context.Context, staffID int64, absenceID int64) error
	getAbsencesForRange func(ctx context.Context, staffID int64, from, to time.Time) ([]*activeSvc.StaffAbsenceResponse, error)
	hasAbsenceOnDateFn  func(ctx context.Context, staffID int64, date time.Time) (bool, *activeModels.StaffAbsence, error)
}

func (m *mockStaffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
	if m.createAbsenceFn != nil {
		return m.createAbsenceFn(ctx, staffID, req)
	}
	return &activeSvc.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, absenceID int64, req activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
	if m.updateAbsenceFn != nil {
		return m.updateAbsenceFn(ctx, staffID, absenceID, req)
	}
	return &activeSvc.StaffAbsenceResponse{}, nil
}
func (m *mockStaffAbsenceService) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	if m.deleteAbsenceFn != nil {
		return m.deleteAbsenceFn(ctx, staffID, absenceID)
	}
	return nil
}
func (m *mockStaffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to time.Time) ([]*activeSvc.StaffAbsenceResponse, error) {
	if m.getAbsencesForRange != nil {
		return m.getAbsencesForRange(ctx, staffID, from, to)
	}
	return nil, nil
}
func (m *mockStaffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date time.Time) (bool, *activeModels.StaffAbsence, error) {
	if m.hasAbsenceOnDateFn != nil {
		return m.hasAbsenceOnDateFn(ctx, staffID, date)
	}
	return false, nil, nil
}

// --- Test helpers ---

func defaultPersonSvc() *mockPersonService {
	return &mockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			p := &userModels.Person{}
			p.ID = 10
			return p, nil
		},
		staffRepo: &mockStaffRepo{
			findByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				s := &userModels.Staff{}
				s.ID = 100
				return s, nil
			},
		},
	}
}

func testResource(wsSvc *mockWorkSessionService, absSvc *mockStaffAbsenceService, pSvc *mockPersonService) *Resource {
	return NewResource(wsSvc, absSvc, pSvc)
}

func withClaims(r *http.Request, claims jwt.AppClaims) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxClaims, claims)
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func validClaims() jwt.AppClaims {
	return jwt.AppClaims{ID: 1}
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
	_ usersSvc.PersonService        = (*mockPersonService)(nil)
	_ userModels.StaffRepository    = (*mockStaffRepo)(nil)
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
	rs := NewResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())
	assert.NotNil(t, rs)
	assert.NotNil(t, rs.WorkSessionService)
	assert.NotNil(t, rs.StaffAbsenceService)
	assert.NotNil(t, rs.PersonService)
}

// --- Router ---

func TestRouter(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())
	router := rs.Router()
	assert.NotNil(t, router)
}

// --- getStaffIDFromClaims ---

func TestGetStaffIDFromClaims_Success(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())
	claims := jwt.AppClaims{ID: 1}
	staffID, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.NoError(t, err)
	assert.Equal(t, int64(100), staffID)
}

func TestGetStaffIDFromClaims_ZeroID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())
	claims := jwt.AppClaims{ID: 0}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestGetStaffIDFromClaims_PersonNotFound(t *testing.T) {
	pSvc := &mockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
		staffRepo: &mockStaffRepo{},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc)
	claims := jwt.AppClaims{ID: 1}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "person not found")
}

func TestGetStaffIDFromClaims_StaffNotFound(t *testing.T) {
	pSvc := &mockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			p := &userModels.Person{}
			p.ID = 10
			return p, nil
		},
		staffRepo: &mockStaffRepo{
			findByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				return nil, errors.New("not found")
			},
		},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc)
	claims := jwt.AppClaims{ID: 1}
	_, err := rs.getStaffIDFromClaims(context.Background(), claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staff record not found")
}

// --- checkIn handler ---

func TestCheckIn_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, staffID int64, status string) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "present", status)
			ws := &activeModels.WorkSession{Status: "present"}
			ws.ID = 1
			return ws, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	body := bytes.NewBufferString(`{"status":"invalid"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckIn_InvalidClaims(t *testing.T) {
	pSvc := &mockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
		staffRepo: &mockStaffRepo{},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc)

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckIn_ServiceConflict(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		checkInFn: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			return nil, errors.New("already checked in")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	body := bytes.NewBufferString(`{"status":"present"}`)
	r := httptest.NewRequest(http.MethodPost, "/check-in", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkIn(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- checkOut handler ---

func TestCheckOut_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, staffID int64) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			ws := &activeModels.WorkSession{}
			ws.ID = 1
			return ws, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckOut_NoActiveSession(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		checkOutFn: func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
			return nil, errors.New("no active session found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/check-out", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.checkOut(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCheckOut_Unauthorized(t *testing.T) {
	pSvc := &mockPersonService{
		findByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			return nil, errors.New("not found")
		},
		staffRepo: &mockStaffRepo{},
	}
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, pSvc)

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
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/current", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getCurrent(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getHistory handler ---

func TestGetHistory_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, staffID int64, from, to time.Time) ([]*activeSvc.SessionResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "2026-01-01", from.Format("2006-01-02"))
			assert.Equal(t, "2026-01-31", to.Format("2006-01-02"))
			return []*activeSvc.SessionResponse{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetHistory_MissingDateParams(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/history", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidFromDate(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/history?from=bad&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_InvalidToDate(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=bad", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetHistory_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getHistoryFn: func(_ context.Context, _ int64, _, _ time.Time) ([]*activeSvc.SessionResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/history?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.getHistory(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- updateSession handler ---

func TestUpdateSession_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, staffID int64, sessionID int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(42), sessionID)
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	wsSvc := &mockWorkSessionService{
		updateSessionFn: func(_ context.Context, _ int64, _ int64, _ activeSvc.SessionUpdateRequest) (*activeModels.WorkSession, error) {
			return nil, errors.New("can only update own sessions")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	body := bytes.NewBufferString(`{"notes":"x"}`)
	r := httptest.NewRequest(http.MethodPut, "/42", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.updateSession(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- startBreak handler ---

func TestStartBreak_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, staffID int64) (*activeModels.WorkSessionBreak, error) {
			assert.Equal(t, int64(100), staffID)
			return &activeModels.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartBreak_AlreadyActive(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		startBreakFn: func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
			return nil, errors.New("break already active")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/break/start", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.startBreak(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- endBreak handler ---

func TestEndBreak_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, staffID int64) (*activeModels.WorkSession, error) {
			assert.Equal(t, int64(100), staffID)
			return &activeModels.WorkSession{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndBreak_NoActiveBreak(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		endBreakFn: func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
			return nil, errors.New("no active break found")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodPost, "/break/end", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.endBreak(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- getBreaks handler ---

func TestGetBreaks_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
			assert.Equal(t, int64(42), sessionID)
			return []*activeModels.WorkSessionBreak{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetBreaks_InvalidSessionID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/breaks/abc", nil)
	r = withChiParam(r, "sessionId", "abc")
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetBreaks_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionBreaksFn: func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/breaks/42", nil)
	r = withChiParam(r, "sessionId", "42")
	w := httptest.NewRecorder()

	rs.getBreaks(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getSessionEdits handler ---

func TestGetSessionEdits_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getSessionEditsFn: func(_ context.Context, sessionID int64) ([]*auditModels.WorkSessionEdit, error) {
			assert.Equal(t, int64(42), sessionID)
			return []*auditModels.WorkSessionEdit{}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/42/edits", nil)
	r = withChiParam(r, "id", "42")
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSessionEdits_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/abc/edits", nil)
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.getSessionEdits(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- exportSessions handler ---

func TestExportSessions_CSV(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, staffID int64, _, _ time.Time, format string) ([]byte, string, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "csv", format)
			return []byte("date,time\n"), "export.csv", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
		exportSessionsFn: func(_ context.Context, _ int64, _, _ time.Time, format string) ([]byte, string, error) {
			assert.Equal(t, "xlsx", format)
			return []byte{0x50, 0x4B}, "export.xlsx", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31&format=xlsx", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "spreadsheetml")
}

func TestExportSessions_DefaultCSV(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _ time.Time, format string) ([]byte, string, error) {
			assert.Equal(t, "csv", format)
			return []byte("data"), "export.csv", nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExportSessions_MissingDates(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/export", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportSessions_ServiceError(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		exportSessionsFn: func(_ context.Context, _ int64, _, _ time.Time, _ string) ([]byte, string, error) {
			return nil, "", errors.New("export failed")
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/export?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.exportSessions(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- listAbsences handler ---

func TestListAbsences_Success(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, staffID int64, from, to time.Time) ([]*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			return []*activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAbsences_MissingDates(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/absences", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAbsences_ServiceError(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		getAbsencesForRange: func(_ context.Context, _ int64, _, _ time.Time) ([]*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("database error")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodGet, "/absences?from=2026-01-01&to=2026-01-31", nil)
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.listAbsences(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createAbsence handler ---

func TestCreateAbsence_Success(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, staffID int64, req activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, "sick", req.AbsenceType)
			return &activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	body := bytes.NewBufferString(`{"absence_type":"sick","date_start":"2026-01-10","date_end":"2026-01-12"}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateAbsence_InvalidBody(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPost, "/absences", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	w := httptest.NewRecorder()

	rs.createAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAbsence_Conflict(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		createAbsenceFn: func(_ context.Context, _ int64, _ activeSvc.CreateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("absence overlaps with existing absence")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

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
	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, staffID int64, absenceID int64, _ activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(7), absenceID)
			return &activeSvc.StaffAbsenceResponse{}, nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	body := bytes.NewBufferString(`{"note":"updated note"}`)
	r := httptest.NewRequest(http.MethodPut, "/absences/7", body)
	r.Header.Set("Content-Type", "application/json")
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.updateAbsence(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateAbsence_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	absSvc := &mockStaffAbsenceService{
		updateAbsenceFn: func(_ context.Context, _ int64, _ int64, _ activeSvc.UpdateAbsenceRequest) (*activeSvc.StaffAbsenceResponse, error) {
			return nil, errors.New("can only update own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

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
	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, staffID int64, absenceID int64) error {
			assert.Equal(t, int64(100), staffID)
			assert.Equal(t, int64(7), absenceID)
			return nil
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteAbsence_InvalidID(t *testing.T) {
	rs := testResource(&mockWorkSessionService{}, &mockStaffAbsenceService{}, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodDelete, "/absences/abc", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "abc")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAbsence_NotFound(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("absence not found")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteAbsence_Forbidden(t *testing.T) {
	absSvc := &mockStaffAbsenceService{
		deleteAbsenceFn: func(_ context.Context, _ int64, _ int64) error {
			return errors.New("can only delete own absences")
		},
	}
	rs := testResource(&mockWorkSessionService{}, absSvc, defaultPersonSvc())

	r := httptest.NewRequest(http.MethodDelete, "/absences/7", nil)
	r = withClaims(r, validClaims())
	r = withChiParam(r, "id", "7")
	w := httptest.NewRecorder()

	rs.deleteAbsence(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- getPresenceMap handler ---

func TestGetPresenceMap_Success(t *testing.T) {
	wsSvc := &mockWorkSessionService{
		getTodayPresenceFn: func(_ context.Context) (map[int64]string, error) {
			return map[int64]string{1: "present", 2: "home_office"}, nil
		},
	}
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
	rs := testResource(wsSvc, &mockStaffAbsenceService{}, defaultPersonSvc())

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
		{"internal server - unknown", "some unknown error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := classifyServiceError(errors.New(tt.errMsg))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			err := renderer.Render(w, r)
			require.NoError(t, err)
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
		{"internal server - unknown", "some unknown error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := classifyAbsenceError(errors.New(tt.errMsg))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			err := renderer.Render(w, r)
			require.NoError(t, err)
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
