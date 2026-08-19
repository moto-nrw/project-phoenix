package timetable

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
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Mock MaterializationService — narrow test double for the HTTP handler.
// The real behavioural coverage lives in
// services/schedule/materialization_*.go tests; here we only verify the
// handler's contract: input parsing, validation, auth plumbing, response shape.
// -----------------------------------------------------------------------------

type mockMaterializer struct {
	result   *scheduleSvc.MaterializationResult
	err      error
	lastFrom timezone.Date
	lastTo   timezone.Date
	lastSrc  scheduleSvc.MaterializationSource
	called   int
}

func (m *mockMaterializer) MaterializeForTenant(_ context.Context, from, to timezone.Date, source scheduleSvc.MaterializationSource) (*scheduleSvc.MaterializationResult, error) {
	m.called++
	m.lastFrom = from
	m.lastTo = to
	m.lastSrc = source
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		m.result.From = from
		m.result.To = to
		return m.result, nil
	}
	return &scheduleSvc.MaterializationResult{From: from, To: to}, nil
}

func (m *mockMaterializer) ResolveWindow(baseDate timezone.Date, weeksAhead int) (timezone.Date, timezone.Date) {
	// Not used by the handler; provide a sensible default so any accidental
	// caller still gets a non-zero window.
	if weeksAhead < 1 {
		weeksAhead = 1
	}
	return baseDate, baseDate.AddDays(weeksAhead*7 - 1)
}

func (m *mockMaterializer) DetectEditedInWindow(_ context.Context, _ int64, _, _ timezone.Date, _ bool) ([]scheduleSvc.EditedOccurrence, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// resolveMaterializationWindow — pure logic, no HTTP plumbing.
// -----------------------------------------------------------------------------

func TestResolveMaterializationWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 22, 12, 0, 0, 0, time.UTC) // Wed
	strp := func(s string) *string { return &s }

	t.Run("no body → next Monday + 6 days (Sun)", func(t *testing.T) {
		from, to, err := resolveMaterializationWindow(&materializeRequest{}, now)
		require.NoError(t, err)
		assert.Equal(t, "2026-04-27", from.Format("2006-01-02"))
		assert.Equal(t, "2026-05-03", to.Format("2006-01-02"))
	})

	t.Run("only from_date given → error", func(t *testing.T) {
		_, _, err := resolveMaterializationWindow(&materializeRequest{FromDate: strp("2026-04-27")}, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both be present or both omitted")
	})

	t.Run("only to_date given → error", func(t *testing.T) {
		_, _, err := resolveMaterializationWindow(&materializeRequest{ToDate: strp("2026-05-03")}, now)
		require.Error(t, err)
	})

	t.Run("invalid date format → error", func(t *testing.T) {
		_, _, err := resolveMaterializationWindow(&materializeRequest{
			FromDate: strp("not-a-date"),
			ToDate:   strp("2026-05-03"),
		}, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_date")
	})

	t.Run("from > to → error", func(t *testing.T) {
		_, _, err := resolveMaterializationWindow(&materializeRequest{
			FromDate: strp("2026-05-03"),
			ToDate:   strp("2026-04-27"),
		}, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not be before")
	})

	t.Run("window > 56 days → error", func(t *testing.T) {
		_, _, err := resolveMaterializationWindow(&materializeRequest{
			FromDate: strp("2026-04-27"),
			ToDate:   strp("2026-06-30"), // > 8 weeks
		}, now)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "56 days")
	})

	t.Run("both dates present within limit → parsed", func(t *testing.T) {
		from, to, err := resolveMaterializationWindow(&materializeRequest{
			FromDate: strp("2026-04-27"),
			ToDate:   strp("2026-05-03"),
		}, now)
		require.NoError(t, err)
		assert.Equal(t, "2026-04-27", from.Format("2006-01-02"))
		assert.Equal(t, "2026-05-03", to.Format("2006-01-02"))
	})
}

// -----------------------------------------------------------------------------
// Handler smoke tests — wire through chi router with a direct Post("/", …),
// bypassing auth middleware (the existing api_test.go pattern).
// -----------------------------------------------------------------------------

func setupMaterializeRouter(rs *Resource) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/", rs.materialize)
	return r
}

func TestMaterialize_NoBody_DefaultsToNextWeek(t *testing.T) {
	t.Parallel()

	mock := &mockMaterializer{
		result: &scheduleSvc.MaterializationResult{
			InstancesCreated:        3,
			InstanceStudentsCreated: 9,
			InstanceStaffCreated:    2,
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mock})
	router := setupMaterializeRouter(rs)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mock.called)
	assert.Equal(t, scheduleSvc.MaterializationSourceManual, mock.lastSrc)
	assert.Equal(t, time.Monday, mock.lastFrom.Weekday(),
		"default window must start on a Monday")
	assert.Equal(t, time.Sunday, mock.lastTo.Weekday(),
		"default window must end on a Sunday")
}

func TestMaterialize_ValidBody_ParsesAndForwards(t *testing.T) {
	t.Parallel()

	mock := &mockMaterializer{
		result: &scheduleSvc.MaterializationResult{InstancesCreated: 1},
	}
	rs := NewResource(Dependencies{MaterializationService: mock})
	router := setupMaterializeRouter(rs)

	body, _ := json.Marshal(map[string]string{
		"from_date": "2026-04-27",
		"to_date":   "2026-05-03",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "2026-04-27", mock.lastFrom.Format("2006-01-02"))
	assert.Equal(t, "2026-05-03", mock.lastTo.Format("2006-01-02"))

	// Verify the response shape is wired through the struct fields.
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "response.data should be an object")
	assert.Equal(t, float64(1), data["instances_created"])
}

func TestMaterialize_InvalidWindow_Returns400(t *testing.T) {
	t.Parallel()

	mock := &mockMaterializer{}
	rs := NewResource(Dependencies{MaterializationService: mock})
	router := setupMaterializeRouter(rs)

	body, _ := json.Marshal(map[string]string{
		"from_date": "2026-04-27",
		"to_date":   "2026-04-26", // to < from
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, mock.called, "service must not be called on validation errors")
}

func TestMaterialize_NilService_Returns500(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{})
	router := setupMaterializeRouter(rs)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMaterialize_ServiceError_Returns500(t *testing.T) {
	t.Parallel()

	mock := &mockMaterializer{err: errors.New("boom")}
	rs := NewResource(Dependencies{MaterializationService: mock})
	router := setupMaterializeRouter(rs)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 1, mock.called)
}
