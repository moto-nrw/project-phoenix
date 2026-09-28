package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/stretchr/testify/assert"
)

func newTestClosingDay() schoolcalendar.ClosingDay {
	return schoolcalendar.ClosingDay{
		ID:        int64(7),
		StartDate: "2026-12-24",
		EndDate:   "2026-12-31",
		Reason:    "Weihnachtswoche",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestListClosingDays(t *testing.T) {
	t.Parallel()

	t.Run("returns all closing days", func(t *testing.T) {
		mock := &ClosingDaysMock{
			ListFn: func(_ context.Context, _ schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
				return []schoolcalendar.ClosingDay{newTestClosingDay()}, nil
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.listClosingDays, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Weihnachtswoche")
		assert.Contains(t, w.Body.String(), "2026-12-24")
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mock := &ClosingDaysMock{
			ListFn: func(_ context.Context, _ schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
				return nil, errors.New("db error: password=secret")
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.listClosingDays, http.MethodGet, false)

		w := executeRequest(router, http.MethodGet, "/", nil)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.NotContains(t, w.Body.String(), "password=secret")
	})
}

func TestCreateClosingDay(t *testing.T) {
	t.Parallel()

	t.Run("creates a closing day range", func(t *testing.T) {
		var created *schoolcalendar.CreateClosingDay
		mock := &ClosingDaysMock{
			CreateFn: func(_ context.Context, input schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error) {
				created = &input
				return schoolcalendar.ClosingDay{ID: 1, StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason}, nil
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
			Reason:    "Sommerschließung",
		})

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.NotNil(t, created)
		assert.Equal(t, "2026-07-20", created.StartDate)
		assert.Equal(t, "Sommerschließung", created.Reason)
	})

	t.Run("accepts single-day range (start = end)", func(t *testing.T) {
		mock := &ClosingDaysMock{}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2027-02-08",
			EndDate:   "2027-02-08",
			Reason:    "Rosenmontag",
		})

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("rejects missing reason", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects whitespace-only reason", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
			Reason:    " \t\n ",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("counts reason length in characters and trims it", func(t *testing.T) {
		var created *schoolcalendar.CreateClosingDay
		mock := &ClosingDaysMock{
			CreateFn: func(_ context.Context, input schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error) {
				created = &input
				return schoolcalendar.ClosingDay{ID: 1, StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason}, nil
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
			Reason:    "  " + strings.Repeat("ä", schoolcalendar.ClosingDayReasonMaxLength) + "  ",
		})

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.NotNil(t, created)
		assert.Equal(t, strings.Repeat("ä", schoolcalendar.ClosingDayReasonMaxLength), created.Reason)

		w = executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-07-20",
			EndDate:   "2026-08-07",
			Reason:    strings.Repeat("ä", schoolcalendar.ClosingDayReasonMaxLength+1),
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects end before start", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
		router := setupTestRouter(res.createClosingDay, http.MethodPost, false)

		w := executeRequest(router, http.MethodPost, "/", ClosingDayRequest{
			StartDate: "2026-08-07",
			EndDate:   "2026-07-20",
			Reason:    "Verkehrt",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects invalid date format", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
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
	t.Parallel()

	t.Run("updates a closing day", func(t *testing.T) {
		var updated *schoolcalendar.UpdateClosingDay
		mock := &ClosingDaysMock{
			FindFn: func(_ context.Context, _ int64) (schoolcalendar.ClosingDay, error) {
				return newTestClosingDay(), nil
			},
			UpdateFn: func(_ context.Context, input schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error) {
				updated = &input
				return schoolcalendar.ClosingDay{ID: input.ID, StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason}, nil
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.updateClosingDay, http.MethodPut, true)

		w := executeRequest(router, http.MethodPut, "/7", ClosingDayRequest{
			StartDate: "2026-12-23",
			EndDate:   "2027-01-02",
			Reason:    "Weihnachtsferien",
		})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotNil(t, updated)
		assert.Equal(t, int64(7), updated.ID)
		assert.Equal(t, "2027-01-02", updated.EndDate)
		assert.Equal(t, "Weihnachtsferien", updated.Reason)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
		router := setupTestRouter(res.updateClosingDay, http.MethodPut, true)

		w := executeRequest(router, http.MethodPut, "/abc", ClosingDayRequest{
			StartDate: "2026-12-23",
			EndDate:   "2027-01-02",
			Reason:    "Weihnachtsferien",
		})

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &ClosingDaysMock{
			FindFn: func(_ context.Context, _ int64) (schoolcalendar.ClosingDay, error) {
				return schoolcalendar.ClosingDay{}, schoolcalendar.ErrClosingDayNotFound
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
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
	t.Parallel()

	t.Run("deletes a closing day", func(t *testing.T) {
		var deleted int64
		mock := &ClosingDaysMock{
			FindFn: func(_ context.Context, _ int64) (schoolcalendar.ClosingDay, error) {
				return newTestClosingDay(), nil
			},
			DeleteFn: func(_ context.Context, id int64) error {
				deleted = id
				return nil
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/7", nil)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, int64(7), deleted)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		mock := &ClosingDaysMock{
			FindFn: func(_ context.Context, _ int64) (schoolcalendar.ClosingDay, error) {
				return schoolcalendar.ClosingDay{}, schoolcalendar.ErrClosingDayNotFound
			},
		}
		res := NewResource(Dependencies{ClosingDays: mock})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/999", nil)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for invalid ID", func(t *testing.T) {
		res := NewResource(Dependencies{ClosingDays: &ClosingDaysMock{}})
		router := setupTestRouter(res.deleteClosingDay, http.MethodDelete, true)

		w := executeRequest(router, http.MethodDelete, "/abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
