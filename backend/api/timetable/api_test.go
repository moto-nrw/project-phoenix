package timetable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock CalendarPeriodService
// =============================================================================

type mockCalendarPeriodService struct {
	periods           []*schedule.CalendarPeriod
	period            *schedule.CalendarPeriod
	err               error // default error for all methods
	updateErr         error // if set, UpdatePeriod returns this instead of err
	deleteErr         error // if set, DeletePeriod returns this instead of err
	lastCreated       *schedule.CalendarPeriod
	lastUpdated       *schedule.CalendarPeriod
	lastDeletedID     int64
	shouldMaterialize bool
}

func (m *mockCalendarPeriodService) GetAllPeriods(_ context.Context) ([]*schedule.CalendarPeriod, error) {
	return m.periods, m.err
}

func (m *mockCalendarPeriodService) GetActivePeriods(_ context.Context) ([]*schedule.CalendarPeriod, error) {
	return m.periods, m.err
}

func (m *mockCalendarPeriodService) GetPeriodByID(_ context.Context, _ int64) (*schedule.CalendarPeriod, error) {
	return m.period, m.err
}

func (m *mockCalendarPeriodService) CreatePeriod(_ context.Context, p *schedule.CalendarPeriod) error {
	m.lastCreated = p
	if m.err != nil {
		return m.err
	}
	p.ID = int64(100)
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return nil
}

func (m *mockCalendarPeriodService) UpdatePeriod(_ context.Context, p *schedule.CalendarPeriod) error {
	m.lastUpdated = p
	if m.updateErr != nil {
		return m.updateErr
	}
	return m.err
}

func (m *mockCalendarPeriodService) DeletePeriod(_ context.Context, id int64) error {
	m.lastDeletedID = id
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return m.err
}

func (m *mockCalendarPeriodService) GetOrCreateDefaultPeriod(_ context.Context) (*schedule.CalendarPeriod, error) {
	return m.period, m.err
}

func (m *mockCalendarPeriodService) ShouldMaterialize(_ int, _ time.Time, _ *schedule.CalendarPeriod) bool {
	return m.shouldMaterialize
}

// scheduleSvcErr is a minimal stand-in for services/schedule.ScheduleError.
// It lets the test verify errors.Is traverses a wrapping layer without pulling
// the services package into the handler test.
type scheduleSvcErr struct {
	op    string
	inner error
}

func (e *scheduleSvcErr) Error() string { return e.op + ": " + e.inner.Error() }
func (e *scheduleSvcErr) Unwrap() error { return e.inner }

// =============================================================================
// Helper functions
// =============================================================================

func setupTestRouter(handler http.HandlerFunc, method string, hasID bool) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	if hasID {
		switch method {
		case http.MethodGet:
			r.Get("/{id}", handler)
		case http.MethodPut:
			r.Put("/{id}", handler)
		case http.MethodDelete:
			r.Delete("/{id}", handler)
		}
	} else {
		switch method {
		case http.MethodGet:
			r.Get("/", handler)
		case http.MethodPost:
			r.Post("/", handler)
		}
	}
	return r
}

func executeRequest(router chi.Router, method, path string, body interface{}) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func newTestPeriod() *schedule.CalendarPeriod {
	p := &schedule.CalendarPeriod{
		Name:            "Test Period",
		PeriodType:      schedule.PeriodTypeSchoolYear,
		StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	p.ID = int64(42)
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return p
}

// =============================================================================
// NewResource / Router Tests
// =============================================================================

func TestNewResource(t *testing.T) {
	res := NewResource(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, res)
}

func TestResource_Router(t *testing.T) {
	mock := &mockCalendarPeriodService{}
	res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := res.Router()
	assert.NotNil(t, router)
}

// =============================================================================
// Bind Validation Tests
// =============================================================================

func TestCalendarPeriodRequest_Bind(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:       "Test Period",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}
		err := req.Bind(nil)
		assert.NoError(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("name too long", func(t *testing.T) {
		longName := ""
		for i := 0; i < 256; i++ {
			longName += "x"
		}
		req := &CalendarPeriodRequest{
			Name:       longName,
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot exceed 255 characters")
	})

	t.Run("missing period_type", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:      "Test",
			StartDate: "2025-08-01",
			EndDate:   "2026-07-31",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "period_type is required")
	})

	t.Run("invalid period_type", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:       "Test",
			PeriodType: "invalid_type",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid period_type")
	})

	t.Run("missing start_date", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:       "Test",
			PeriodType: "school_year",
			EndDate:    "2026-07-31",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start_date is required")
	})

	t.Run("missing end_date", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:       "Test",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
		}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_date is required")
	})

	t.Run("defaults week_cycle_length to 1 when zero", func(t *testing.T) {
		req := &CalendarPeriodRequest{
			Name:            "Test",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 0,
		}
		err := req.Bind(nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, req.WeekCycleLength)
	})
}

// =============================================================================
// mapPeriodToResponse Tests
// =============================================================================

func TestMapPeriodToResponse(t *testing.T) {
	t.Run("maps period without anchor", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			Name:            "School Year 2025/2026",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		p.ID = int64(42)
		p.CreatedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		p.UpdatedAt = time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

		resp := mapPeriodToResponse(p)

		assert.Equal(t, int64(42), resp.ID)
		assert.Equal(t, "School Year 2025/2026", resp.Name)
		assert.Equal(t, "school_year", resp.PeriodType)
		assert.Equal(t, "2025-08-01", resp.StartDate)
		assert.Equal(t, "2026-07-31", resp.EndDate)
		assert.Equal(t, 1, resp.WeekCycleLength)
		assert.Nil(t, resp.WeekCycleAnchor)
		assert.True(t, resp.IsActive)
		assert.NotEmpty(t, resp.CreatedAt)
		assert.NotEmpty(t, resp.UpdatedAt)
	})

	t.Run("maps period with anchor", func(t *testing.T) {
		anchor := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		p := &schedule.CalendarPeriod{
			Name:            "AB Week Period",
			PeriodType:      schedule.PeriodTypeSemester,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 2,
			WeekCycleAnchor: &anchor,
			IsActive:        false,
		}
		p.ID = int64(43)
		p.CreatedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		p.UpdatedAt = time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

		resp := mapPeriodToResponse(p)

		assert.Equal(t, int64(43), resp.ID)
		assert.Equal(t, "semester", resp.PeriodType)
		assert.Equal(t, 2, resp.WeekCycleLength)
		require.NotNil(t, resp.WeekCycleAnchor)
		assert.Equal(t, "2025-09-01", *resp.WeekCycleAnchor)
		assert.False(t, resp.IsActive)
	})
}

// =============================================================================
// listPeriods Handler Tests
// =============================================================================

func TestListPeriods(t *testing.T) {
	t.Run("returns periods list", func(t *testing.T) {
		p1 := &schedule.CalendarPeriod{
			Name:            "Period A",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		p1.ID = int64(10)
		p1.CreatedAt = time.Now()
		p1.UpdatedAt = time.Now()

		mock := &mockCalendarPeriodService{periods: []*schedule.CalendarPeriod{p1}}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Period A")
	})

	t.Run("returns empty list", func(t *testing.T) {
		mock := &mockCalendarPeriodService{periods: []*schedule.CalendarPeriod{}}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db error")}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// getPeriod Handler Tests
// =============================================================================

func TestGetPeriod(t *testing.T) {
	t.Run("returns period by ID", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			Name:            "Found Period",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		p.ID = int64(42)
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()

		mock := &mockCalendarPeriodService{period: p}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/42", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Found Period")
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 500 on internal error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db connection failed")}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// createPeriod Handler Tests
// =============================================================================

func TestCreatePeriod(t *testing.T) {
	t.Run("creates period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:            "New Period",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 1,
			IsActive:        true,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "New Period")
		assert.NotNil(t, mock.lastCreated)
		assert.Equal(t, "New Period", mock.lastCreated.Name)
	})

	t.Run("creates period with week cycle anchor", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		anchor := "2025-09-01"
		body := CalendarPeriodRequest{
			Name:            "AB Period",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 2,
			WeekCycleAnchor: &anchor,
			IsActive:        true,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusCreated, w.Code)
		require.NotNil(t, mock.lastCreated)
		assert.NotNil(t, mock.lastCreated.WeekCycleAnchor)
	})

	t.Run("defaults week cycle length to 1", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:            "Default Cycle",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 0,
			IsActive:        true,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusCreated, w.Code)
		require.NotNil(t, mock.lastCreated)
		assert.Equal(t, 1, mock.lastCreated.WeekCycleLength)
	})

	t.Run("returns 400 for missing required fields", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{Name: "Only Name"}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid period_type", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Bad Type",
			PeriodType: "invalid_type",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid period_type")
	})

	t.Run("returns 400 for invalid start_date format", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Bad Start",
			PeriodType: "school_year",
			StartDate:  "not-a-date",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "start_date")
	})

	t.Run("returns 400 for invalid end_date format", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Bad End",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "not-a-date",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "end_date")
	})

	t.Run("returns 400 for invalid week_cycle_anchor format", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		badAnchor := "not-a-date"
		body := CalendarPeriodRequest{
			Name:            "Bad Anchor",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 2,
			WeekCycleAnchor: &badAnchor,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "week_cycle_anchor")
	})

	t.Run("returns 400 for end_date before start_date", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Bad Dates",
			PeriodType: "school_year",
			StartDate:  "2026-08-01",
			EndDate:    "2025-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "end_date must be after start_date")
	})

	t.Run("returns 400 for cycle > 1 without anchor", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:            "No Anchor",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 2,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "week_cycle_anchor is required")
	})

	t.Run("returns 409 for duplicate name", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: schedule.ErrCalendarPeriodNameConflict}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Duplicate",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already exists")
	})

	t.Run("returns 409 when service wraps the sentinel", func(t *testing.T) {
		// Services wrap the sentinel in ScheduleError — errors.Is must still match.
		mock := &mockCalendarPeriodService{
			err: &scheduleSvcErr{op: "create calendar period", inner: schedule.ErrCalendarPeriodNameConflict},
		}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Duplicate",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("create failed")}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Fail Create",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// updatePeriod Handler Tests
// =============================================================================

func TestUpdatePeriod(t *testing.T) {
	existingPeriod := newTestPeriod()

	t.Run("updates period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Updated Period",
			PeriodType: "semester",
			StartDate:  "2025-08-01",
			EndDate:    "2026-01-31",
			IsActive:   false,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Updated Period")
	})

	t.Run("updates period with anchor", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		anchor := "2025-09-01"
		body := CalendarPeriodRequest{
			Name:            "With Anchor",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 2,
			WeekCycleAnchor: &anchor,
			IsActive:        true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("clears anchor when not provided", func(t *testing.T) {
		anchoredPeriod := &schedule.CalendarPeriod{
			Name:            "Anchored",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		anchoredPeriod.ID = int64(42)
		anchoredPeriod.CreatedAt = time.Now()
		anchoredPeriod.UpdatedAt = time.Now()
		anchorTime := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		anchoredPeriod.WeekCycleAnchor = &anchorTime

		mock := &mockCalendarPeriodService{period: anchoredPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "No Anchor",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
			IsActive:   true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusOK, w.Code)
		require.NotNil(t, mock.lastUpdated)
		assert.Nil(t, mock.lastUpdated.WeekCycleAnchor)
	})

	t.Run("defaults week cycle length to 1", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:            "Zero Cycle",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 0,
			IsActive:        true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusOK, w.Code)
		require.NotNil(t, mock.lastUpdated)
		assert.Equal(t, 1, mock.lastUpdated.WeekCycleLength)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when period not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 500 on get internal error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db connection failed")}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("returns 400 for invalid start_date in update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Bad Start",
			PeriodType: "school_year",
			StartDate:  "bad-date",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid end_date in update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Bad End",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "bad-date",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid anchor in update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		badAnchor := "bad-date"
		body := CalendarPeriodRequest{
			Name:            "Bad Anchor",
			PeriodType:      "school_year",
			StartDate:       "2025-08-01",
			EndDate:         "2026-07-31",
			WeekCycleLength: 2,
			WeekCycleAnchor: &badAnchor,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for end_date before start_date in update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Bad Dates",
			PeriodType: "school_year",
			StartDate:  "2026-08-01",
			EndDate:    "2025-07-31",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "end_date must be after start_date")
	})

	t.Run("returns 409 for duplicate name on update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    existingPeriod,
			updateErr: schedule.ErrCalendarPeriodNameConflict,
		}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Duplicate Name",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already exists")
	})

	t.Run("returns 500 on service update error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    existingPeriod,
			updateErr: errors.New("db write failed"),
		}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Will Fail",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// =============================================================================
// deletePeriod Handler Tests
// =============================================================================

func TestDeletePeriod(t *testing.T) {
	t.Run("deletes period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: newTestPeriod()}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(42), mock.lastDeletedID)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when period not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    newTestPeriod(),
			deleteErr: errors.New("delete failed"),
		}
		res := NewResource(mock, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
