package timetable

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Mock ClosingDayService
// =============================================================================

type mockClosingDayService struct {
	days        []*schedule.ClosingDay
	day         *schedule.ClosingDay
	err         error
	lastCreated *schedule.ClosingDay
	lastUpdated *schedule.ClosingDay
	lastDeleted int64
}

func (m *mockClosingDayService) GetAll(_ context.Context) ([]*schedule.ClosingDay, error) {
	return m.days, m.err
}

func (m *mockClosingDayService) GetByID(_ context.Context, _ int64) (*schedule.ClosingDay, error) {
	return m.day, m.err
}

func (m *mockClosingDayService) Create(_ context.Context, d *schedule.ClosingDay) error {
	m.lastCreated = d
	return m.err
}

func (m *mockClosingDayService) Update(_ context.Context, d *schedule.ClosingDay) error {
	m.lastUpdated = d
	return m.err
}

func (m *mockClosingDayService) Delete(_ context.Context, id int64) error {
	m.lastDeleted = id
	return m.err
}

func (m *mockClosingDayService) ClosingDaysInRange(_ context.Context, _, _ timezone.Date) ([]*schedule.ClosingDay, error) {
	return m.days, m.err
}

func (m *mockClosingDayService) ClosingDayDates(_ context.Context, _, _ timezone.Date) (map[timezone.Date]bool, error) {
	return nil, m.err
}

func newTestClosingDay() *schedule.ClosingDay {
	d := &schedule.ClosingDay{
		StartDate: timezone.NewDate(2026, 12, 24),
		EndDate:   timezone.NewDate(2026, 12, 31),
		Reason:    "Weihnachtswoche",
	}
	d.ID = int64(7)
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	return d
}

func TestListClosingDays(t *testing.T) {
	t.Run("returns all closing days", func(t *testing.T) {
		mock := &mockClosingDayService{days: []*schedule.ClosingDay{newTestClosingDay()}}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.listClosingDays, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Weihnachtswoche")
		assert.Contains(t, w.Body.String(), "2026-12-24")
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &mockClosingDayService{err: errors.New("db error: password=secret")}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.listClosingDays, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.NotContains(t, w.Body.String(), "password=secret")
	})
}

func TestCreateClosingDay(t *testing.T) {
	t.Run("creates a closing day range", func(t *testing.T) {
		mock := &mockClosingDayService{}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
			Reason:    "Sommerschließung",
		})

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.NotNil(t, mock.lastCreated)
		assert.Equal(t, timezone.NewDate(2026, 7, 20), mock.lastCreated.StartDate)
		assert.Equal(t, "Sommerschließung", mock.lastCreated.Reason)
	})

	t.Run("accepts single-day range (start = end)", func(t *testing.T) {
		mock := &mockClosingDayService{}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2027-02-08",
			EndDate:   "2027-02-08",
			Reason:    "Rosenmontag",
		})

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("rejects missing reason", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDayService: &mockClosingDayService{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects end before start", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDayService: &mockClosingDayService{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-08-07",
			EndDate:   "2026-07-20",
			Reason:    "Verkehrt",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects invalid date format", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDayService: &mockClosingDayService{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "20.07.2026",
			EndDate:   "2026-08-07",
			Reason:    "Falsches Format",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestUpdateClosingDay(t *testing.T) {
	t.Run("updates a closing day", func(t *testing.T) {
		mock := &mockClosingDayService{day: newTestClosingDay()}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.updateClosingDay, http.MethodPut, true)

		w := executeRequest(router, http.MethodPut, "/7", ClosingDayRequest{
			StartDate: "2026-12-23",
			EndDate:   "2027-01-02",
			Reason:    "Weihnachtsferien",
		})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotNil(t, mock.lastUpdated)
		assert.Equal(t, timezone.NewDate(2027, 1, 2), mock.lastUpdated.EndDate)
		assert.Equal(t, "Weihnachtsferien", mock.lastUpdated.Reason)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDayService: &mockClosingDayService{}})
		router := setupTestRouter(res.updateClosingDay, http.MethodPut, true)

		w := executeRequest(router, http.MethodPut, "/abc", ClosingDayRequest{
			StartDate: "2026-12-23",
			EndDate:   "2027-01-02",
			Reason:    "Weihnachtsferien",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &mockClosingDayService{err: sql.ErrNoRows}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.updateClosingDay, http.MethodPut, true)

		w := executeRequest(router, http.MethodPut, "/999", ClosingDayRequest{
			StartDate: "2026-12-23",
			EndDate:   "2027-01-02",
			Reason:    "Weihnachtsferien",
		})

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestDeleteClosingDay(t *testing.T) {
	t.Run("deletes a closing day", func(t *testing.T) {
		mock := &mockClosingDayService{day: newTestClosingDay()}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/7", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(7), mock.lastDeleted)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &mockClosingDayService{err: sql.ErrNoRows}
		res := NewResource(Dependencies{ClosingDayService: mock})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDayService: &mockClosingDayService{}})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
