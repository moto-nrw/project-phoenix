package timetable

import (
	"bytes"
	"context"
	"database/sql"
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
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	deleteHook        func(context.Context, int64) error
	lastCreated       *schedule.CalendarPeriod
	lastUpdated       *schedule.CalendarPeriod
	lastDeletedID     int64
	shouldMaterialize bool
	overlaps          []*schedule.CalendarPeriod // returned by FindActiveOverlaps
	overlapsErr       error                      // if set, FindActiveOverlaps fails
	bootstrapPeriods  []*schedule.CalendarPeriod // returned by EnsureDefaultSchoolYear
	bootstrapCreated  bool
	bootstrapErr      error                                  // if set, EnsureDefaultSchoolYear fails
	usage             map[int64]schedule.CalendarPeriodUsage // returned by GetUsageCounts
	usageErr          error                                  // if set, GetUsageCounts fails
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

func (m *mockCalendarPeriodService) DeletePeriod(ctx context.Context, id int64) error {
	m.lastDeletedID = id
	if m.deleteHook != nil {
		return m.deleteHook(ctx, id)
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return m.err
}

func (m *mockCalendarPeriodService) ShouldMaterialize(_ int, _ timezone.Date, _ *schedule.CalendarPeriod) bool {
	return m.shouldMaterialize
}

func (m *mockCalendarPeriodService) EnsureDefaultSchoolYear(_ context.Context) ([]*schedule.CalendarPeriod, bool, error) {
	if m.bootstrapErr != nil {
		return nil, false, m.bootstrapErr
	}
	return m.bootstrapPeriods, m.bootstrapCreated, nil
}

func (m *mockCalendarPeriodService) FindActiveOverlaps(_ context.Context, _ *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error) {
	if m.overlapsErr != nil {
		return nil, m.overlapsErr
	}
	return m.overlaps, nil
}

func (m *mockCalendarPeriodService) GetUsageCounts(_ context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	if m.usageErr != nil {
		return nil, m.usageErr
	}
	return m.usage, nil
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
		StartDate:       timezone.NewDate(2025, 8, 1),
		EndDate:         timezone.NewDate(2026, 7, 31),
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
	t.Parallel()

	res := NewResource(Dependencies{})
	assert.NotNil(t, res)
}

func TestResource_Router(t *testing.T) {
	t.Parallel()

	mock := &mockCalendarPeriodService{}
	res := NewResource(Dependencies{CalendarPeriodService: mock})
	router := res.Router()
	assert.NotNil(t, router)
}

// =============================================================================
// Bind Validation Tests
// =============================================================================

func TestCalendarPeriodRequest_Bind(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	t.Run("maps period without anchor", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			Name:            "School Year 2025/2026",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
		anchor := timezone.NewDate(2025, 9, 1)
		p := &schedule.CalendarPeriod{
			Name:            "AB Week Period",
			PeriodType:      schedule.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
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
	t.Parallel()

	t.Run("returns periods list", func(t *testing.T) {
		p1 := &schedule.CalendarPeriod{
			Name:            "Period A",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		p1.ID = int64(10)
		p1.CreatedAt = time.Now()
		p1.UpdatedAt = time.Now()

		mock := &mockCalendarPeriodService{periods: []*schedule.CalendarPeriod{p1}}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Period A")
	})

	t.Run("returns empty list", func(t *testing.T) {
		mock := &mockCalendarPeriodService{periods: []*schedule.CalendarPeriod{}}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db error: password=calendar-secret")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodsLoadErrorMessage)
		assert.NotContains(t, w.Body.String(), "password=calendar-secret")
	})
}

func TestListPeriods_UsageCounts(t *testing.T) {
	t.Parallel()

	p1 := newTestPeriod()
	p1.ID = int64(10)

	t.Run("merges usage counts into responses", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			periods: []*schedule.CalendarPeriod{p1},
			usage: map[int64]schedule.CalendarPeriodUsage{
				p1.ID: {
					EnrollmentPhases:   2,
					ActivityGroups:     7,
					Schedules:          3,
					StudentEnrollments: 4,
					Supervisors:        5,
					ActivityInstances:  6,
				},
			},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"enrollment_phase_count":2`)
		assert.Contains(t, w.Body.String(), `"activity_group_count":7`)
		assert.Contains(t, w.Body.String(), `"schedule_count":3`)
		assert.Contains(t, w.Body.String(), `"student_enrollment_count":4`)
		assert.Contains(t, w.Body.String(), `"supervisor_count":5`)
		assert.Contains(t, w.Body.String(), `"activity_instance_count":6`)
	})

	t.Run("usage count failure fails the list without reporting false zeroes", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			periods:  []*schedule.CalendarPeriod{p1},
			usageErr: errors.New("count failed: relation internal_usage_table does not exist"),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.listPeriods, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodUsageErrorMessage)
		assert.NotContains(t, w.Body.String(), "internal_usage_table")
		assert.NotContains(t, w.Body.String(), `"enrollment_phase_count":0`)
	})
}

// =============================================================================
// getPeriod Handler Tests
// =============================================================================

func TestGetPeriod(t *testing.T) {
	t.Parallel()

	t.Run("returns period by ID", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			Name:            "Found Period",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		p.ID = int64(42)
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()

		mock := &mockCalendarPeriodService{period: p}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/42", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Found Period")
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 500 on internal error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db connection failed")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("usage count failure fails detail without reporting false zeroes", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:   newTestPeriod(),
			usageErr: errors.New("usage failed: private query details"),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.getPeriod, http.MethodGet, true)

		w := executeRequest(router, http.MethodGet, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodUsageErrorMessage)
		assert.NotContains(t, w.Body.String(), "private query details")
		assert.NotContains(t, w.Body.String(), `"enrollment_phase_count":0`)
	})
}

// =============================================================================
// createPeriod Handler Tests
// =============================================================================

func TestCreatePeriod(t *testing.T) {
	t.Parallel()

	t.Run("creates period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{Name: "Only Name"}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid period_type", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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
		res := NewResource(Dependencies{CalendarPeriodService: mock})
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

	t.Run("returns 409 with code for same-type overlap", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			err: &scheduleSvcErr{op: "check same-type period overlap", inner: schedule.ErrCalendarPeriodOverlapConflict},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Overlap",
			PeriodType: "semester",
			StartDate:  "2025-08-01",
			EndDate:    "2026-01-31",
			IsActive:   true,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodOverlapConflictCode)
		assert.Contains(t, w.Body.String(), "aktiver Zeitraum desselben Typs")
	})

	t.Run("names the conflicting period in the overlap 409", func(t *testing.T) {
		conflicting := newTestPeriod()
		conflicting.Name = "Schuljahr 2025/2026"
		mock := &mockCalendarPeriodService{
			err: &scheduleSvcErr{
				op:    "check same-type period overlap",
				inner: &schedule.CalendarPeriodOverlapError{Overlaps: []*schedule.CalendarPeriod{conflicting}},
			},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Overlap",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-01-31",
			IsActive:   true,
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodOverlapConflictCode)
		assert.Contains(t, w.Body.String(), "aktiver Zeitraum desselben Typs")
		assert.Contains(t, w.Body.String(), "Schuljahr 2025/2026")
		assert.Contains(t, w.Body.String(), "01.08.2025 – 31.07.2026")
		assert.Contains(t, w.Body.String(), `"overlapping_period_ids":[42]`)
		assert.Contains(t, w.Body.String(), `"overlapping_period_names":["Schuljahr 2025/2026"]`)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("create failed: duplicate detail from postgres")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		body := CalendarPeriodRequest{
			Name:       "Fail Create",
			PeriodType: "school_year",
			StartDate:  "2025-08-01",
			EndDate:    "2026-07-31",
		}

		w := executeRequest(router, http.MethodPost, "/", body)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodCreateErrorMessage)
		assert.NotContains(t, w.Body.String(), "duplicate detail from postgres")
	})
}

// =============================================================================
// Overlap Warning Tests (WP-B2) — advisory, never blocking
// =============================================================================

func TestCreatePeriodOverlapWarnings(t *testing.T) {
	t.Parallel()

	validBody := CalendarPeriodRequest{
		Name:       "Aktiv mit Kollision",
		PeriodType: "school_year",
		StartDate:  "2030-08-01",
		EndDate:    "2031-07-31",
		IsActive:   true,
	}

	t.Run("attaches one warning per overlapping active period", func(t *testing.T) {
		other := newTestPeriod()
		other.Name = "Schuljahr 2030/2031"
		other.StartDate = timezone.NewDate(2030, 8, 1)
		other.EndDate = timezone.NewDate(2031, 7, 31)

		mock := &mockCalendarPeriodService{overlaps: []*schedule.CalendarPeriod{other}}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", validBody)

		require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
		var env struct {
			Data CalendarPeriodResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		require.Len(t, env.Data.Warnings, 1)
		warning := env.Data.Warnings[0]
		assert.Equal(t, "overlapping_active_periods", warning.Code)
		assert.Equal(t, []int64{other.ID}, warning.OverlappingPeriodIDs)
		assert.Equal(t, []string{other.Name}, warning.OverlappingPeriodNames)
		assert.Contains(t, warning.Message, "Schuljahr 2030/2031")
		assert.Contains(t, warning.Message, "01.08.2030")
		assert.Contains(t, warning.Message, "31.07.2031")
		assert.Contains(t, warning.Message, "Überschneidet sich mit aktivem Zeitraum")
	})

	t.Run("omits warnings field when nothing overlaps", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", validBody)

		require.Equal(t, http.StatusCreated, w.Code)
		assert.NotContains(t, w.Body.String(), "warnings")
	})

	t.Run("overlap check failure never breaks the create", func(t *testing.T) {
		mock := &mockCalendarPeriodService{overlapsErr: errors.New("overlap lookup exploded")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.createPeriod, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", validBody)

		require.Equal(t, http.StatusCreated, w.Code, "advisory check must not block the save")
		assert.NotContains(t, w.Body.String(), "warnings")
	})
}

func TestUpdatePeriodOverlapWarnings(t *testing.T) {
	t.Parallel()

	t.Run("attaches warning on update of overlapping active period", func(t *testing.T) {
		other := newTestPeriod()
		other.Name = "Schuljahr 2030/2031"

		mock := &mockCalendarPeriodService{
			period:   newTestPeriod(),
			overlaps: []*schedule.CalendarPeriod{other},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Umbenannt",
			PeriodType: "school_year",
			StartDate:  "2030-08-01",
			EndDate:    "2031-07-31",
			IsActive:   true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var env struct {
			Data CalendarPeriodResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		require.Len(t, env.Data.Warnings, 1)
		assert.Equal(t, "overlapping_active_periods", env.Data.Warnings[0].Code)
		assert.Equal(t, []string{other.Name}, env.Data.Warnings[0].OverlappingPeriodNames)
	})
}

// =============================================================================
// bootstrapPeriods Handler Tests (WP-B1)
// =============================================================================

func TestBootstrapPeriods(t *testing.T) {
	t.Parallel()

	t.Run("returns 200 with created flag", func(t *testing.T) {
		p := newTestPeriod()
		mock := &mockCalendarPeriodService{
			bootstrapPeriods: []*schedule.CalendarPeriod{p},
			bootstrapCreated: true,
			usage: map[int64]schedule.CalendarPeriodUsage{
				p.ID: {ActivityGroups: 3},
			},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.bootstrapPeriods, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", nil)

		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var env struct {
			Data bootstrapPeriodsResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		assert.True(t, env.Data.Created)
		require.Len(t, env.Data.Periods, 1)
		assert.Equal(t, p.Name, env.Data.Periods[0].Name)
		assert.Equal(t, 3, env.Data.Periods[0].ActivityGroupCount)
	})

	t.Run("returns 200 with created false when periods already exist", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			bootstrapPeriods: []*schedule.CalendarPeriod{newTestPeriod()},
			bootstrapCreated: false,
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.bootstrapPeriods, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", nil)

		require.Equal(t, http.StatusOK, w.Code)
		var env struct {
			Data bootstrapPeriodsResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		assert.False(t, env.Data.Created)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{bootstrapErr: errors.New("bootstrap failed: db host 10.0.0.8")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.bootstrapPeriods, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodsBootstrapErrorMessage)
		assert.NotContains(t, w.Body.String(), "10.0.0.8")
	})

	t.Run("usage count failure fails bootstrap instead of returning unused periods", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			bootstrapPeriods: []*schedule.CalendarPeriod{newTestPeriod()},
			usageErr:         errors.New("usage failed: SELECT secret_usage"),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.bootstrapPeriods, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodUsageErrorMessage)
		assert.NotContains(t, w.Body.String(), "secret_usage")
		assert.NotContains(t, w.Body.String(), `"enrollment_phase_count":0`)
	})
}

// =============================================================================
// updatePeriod Handler Tests
// =============================================================================

func TestUpdatePeriod(t *testing.T) {
	t.Parallel()

	existingPeriod := newTestPeriod()

	t.Run("updates period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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

	t.Run("returns 409 with code for same-type overlap", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    existingPeriod,
			updateErr: &scheduleSvcErr{op: "check same-type period overlap", inner: schedule.ErrCalendarPeriodOverlapConflict},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Overlap Update",
			PeriodType: "semester",
			StartDate:  "2025-08-01",
			EndDate:    "2026-01-31",
			IsActive:   true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodOverlapConflictCode)
		assert.Contains(t, w.Body.String(), "aktiver Zeitraum desselben Typs")
	})

	t.Run("names the conflicting period in the overlap 409", func(t *testing.T) {
		conflicting := newTestPeriod()
		conflicting.Name = "Schuljahr 2025/2026"
		mock := &mockCalendarPeriodService{
			period: existingPeriod,
			updateErr: &scheduleSvcErr{
				op:    "check same-type period overlap",
				inner: &schedule.CalendarPeriodOverlapError{Overlaps: []*schedule.CalendarPeriod{conflicting}},
			},
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Overlap Update",
			PeriodType: "semester",
			StartDate:  "2025-08-01",
			EndDate:    "2026-01-31",
			IsActive:   true,
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), calendarPeriodOverlapConflictCode)
		assert.Contains(t, w.Body.String(), "Schuljahr 2025/2026")
		assert.Contains(t, w.Body.String(), `"overlapping_period_names":["Schuljahr 2025/2026"]`)
	})

	t.Run("updates period with anchor", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		anchoredPeriod.ID = int64(42)
		anchoredPeriod.CreatedAt = time.Now()
		anchoredPeriod.UpdatedAt = time.Now()
		anchorTime := timezone.NewDate(2025, 9, 1)
		anchoredPeriod.WeekCycleAnchor = &anchorTime

		mock := &mockCalendarPeriodService{period: anchoredPeriod}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when period not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 500 on get internal error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: errors.New("db connection failed")}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		w := executeRequest(r, http.MethodPut, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Kalenderzeitraum konnte nicht geladen werden")
		assert.NotContains(t, w.Body.String(), "db connection failed")
	})

	t.Run("returns 400 for invalid start_date in update", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: existingPeriod}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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

	t.Run("returns stable 409 when linked care offerings require the period", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    existingPeriod,
			updateErr: schedule.ErrCalendarPeriodCareOfferingConflict,
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

		r := chi.NewRouter()
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Put("/{id}", res.updatePeriod)

		body := CalendarPeriodRequest{
			Name:       "Incompatible Range",
			PeriodType: "school_year",
			StartDate:  "2025-09-01",
			EndDate:    "2026-06-30",
		}

		w := executeRequest(r, http.MethodPut, "/42", body)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"calendar_period_care_offering_conflict"`)
		assert.Contains(t, w.Body.String(), "verknüpften Betreuungsangebot")
	})

	t.Run("returns 500 on service update error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    existingPeriod,
			updateErr: errors.New("db write failed"),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})

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
		assert.Contains(t, w.Body.String(), "Kalenderzeitraum konnte nicht aktualisiert werden")
		assert.NotContains(t, w.Body.String(), "db write failed")
	})
}

// =============================================================================
// deletePeriod Handler Tests
// =============================================================================

func TestDeletePeriod(t *testing.T) {
	t.Parallel()

	t.Run("deletes period successfully", func(t *testing.T) {
		mock := &mockCalendarPeriodService{period: newTestPeriod()}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(42), mock.lastDeletedID)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		mock := &mockCalendarPeriodService{}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when period not found", func(t *testing.T) {
		mock := &mockCalendarPeriodService{err: sql.ErrNoRows}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 409 when roster links cannot be unscoped", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period: newTestPeriod(),
			deleteErr: errors.New(
				`delete calendar period: duplicate key value violates unique constraint "idx_student_enrollments_active"`,
			),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "doppelte aktive Kinder- oder Personalzuordnungen")
	})

	t.Run("returns stable 409 when linked care offerings require the period", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    newTestPeriod(),
			deleteErr: schedule.ErrCalendarPeriodCareOfferingConflict,
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"calendar_period_care_offering_conflict"`)
		assert.Contains(t, w.Body.String(), "verknüpften Betreuungsangebot")
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockCalendarPeriodService{
			period:    newTestPeriod(),
			deleteErr: errors.New("delete failed"),
		}
		res := NewResource(Dependencies{CalendarPeriodService: mock})
		router := setupTestRouter(res.deletePeriod, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/42", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Kalenderzeitraum konnte nicht gelöscht werden")
		assert.NotContains(t, w.Body.String(), "delete failed")
	})
}

func TestDeletePeriod_RosterConflictMarksTenantRollback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianProfile
	email := fmt.Sprintf("period-delete-rollback-%d@test.local", time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              "Period",
		LastName:               "Rollback",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}

	mock := &mockCalendarPeriodService{
		period: newTestPeriod(),
		deleteHook: func(ctx context.Context, id int64) error {
			require.Equal(t, int64(42), id)
			require.NoError(t, repo.Create(ctx, profile))
			require.Greater(t, profile.ID, int64(0), "probe insert must happen inside the tenant tx")
			return errors.New(
				`delete calendar period: duplicate key value violates unique constraint "idx_student_enrollments_active"`,
			)
		},
	}
	res := NewResource(Dependencies{CalendarPeriodService: mock})

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(req.Context(), testpkg.Tenant(t))))
		})
	})
	router.Use(tenant.TenantTxMiddleware(db))
	router.Delete("/{id}", res.deletePeriod)

	w := executeRequest(router, http.MethodDelete, "/42", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "doppelte aktive Kinder- oder Personalzuordnungen")
	_, err := repo.FindByID(testpkg.Ctx(t), profile.ID)
	assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound,
		"calendar period delete conflicts must roll back writes despite returning 409")
}

func TestDeletePeriod_CareOfferingConflictMarksTenantRollback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := db.NewDelete().
			TableExpr("users.guardian_profiles").
			Where("tenant_id = ?", tenantID).
			Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
		_, cleanupErr = db.NewDelete().TableExpr("platform.schools").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
		_, cleanupErr = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, cleanupErr)
	})

	repo := repositories.NewFactory(db).GuardianProfile
	email := fmt.Sprintf("period-care-conflict-rollback-%d@test.local", time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              "CarePeriod",
		LastName:               "Rollback",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}

	mock := &mockCalendarPeriodService{
		period: newTestPeriod(),
		deleteHook: func(ctx context.Context, id int64) error {
			require.Equal(t, int64(42), id)
			require.NoError(t, repo.Create(ctx, profile))
			require.Greater(t, profile.ID, int64(0), "probe insert must happen inside the tenant tx")
			return schedule.ErrCalendarPeriodCareOfferingConflict
		},
	}
	res := NewResource(Dependencies{CalendarPeriodService: mock})

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(req.Context(), tenantID)))
		})
	})
	router.Use(tenant.TenantTxMiddleware(db))
	router.Delete("/{id}", res.deletePeriod)

	w := executeRequest(router, http.MethodDelete, "/42", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"calendar_period_care_offering_conflict"`)
	_, err := repo.FindByID(testpkg.TenantContext(tenantID), profile.ID)
	assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound,
		"care-offering period conflicts must roll back writes despite returning 409")
}
