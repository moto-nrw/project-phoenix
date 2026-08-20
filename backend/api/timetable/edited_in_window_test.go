package timetable

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

func setupEditedInWindowRouter(rs *Resource) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/instances/edited-in-window", rs.editedInWindow)
	return r
}

func TestEditedInWindow_Success(t *testing.T) {
	t.Parallel()

	var gotGroup int64
	var gotFrom, gotTo timezone.Date
	var gotInclude bool
	mat := &mockMaterializationService{
		detectFn: func(activityGroupID int64, from, to timezone.Date, includeDeletions bool) ([]scheduleSvc.EditedOccurrence, error) {
			gotGroup, gotFrom, gotTo, gotInclude = activityGroupID, from, to, includeDeletions
			return []scheduleSvc.EditedOccurrence{
				{
					InstanceID: 101,
					Date:       timezone.NewDate(2026, 4, 20),
					StartTime:  "15:00:00",
					Title:      "Fußball AG",
					Changes:    []string{scheduleSvc.EditedChangeRoom, scheduleSvc.EditedChangeTitle},
				},
				{
					InstanceID: 102,
					Date:       timezone.NewDate(2026, 4, 27),
					StartTime:  "15:00:00",
					Title:      "Fußball AG",
					Changes:    []string{scheduleSvc.EditedChangeStaff},
				},
			}, nil
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mat})
	router := setupEditedInWindowRouter(rs)

	w := doGet(t, router, "/instances/edited-in-window?activity_group_id=77&from=2026-04-20&to=2026-05-03")

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, int64(77), gotGroup)
	assert.Equal(t, timezone.NewDate(2026, 4, 20), gotFrom)
	assert.Equal(t, timezone.NewDate(2026, 5, 3), gotTo)
	assert.False(t, gotInclude, "include_deletions defaults to false")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), data["count"])

	occ, ok := data["occurrences"].([]any)
	require.True(t, ok, "occurrences must be an array")
	require.Len(t, occ, 2)

	first, ok := occ[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(101), first["instance_id"])
	assert.Equal(t, "2026-04-20", first["date"])
	assert.Equal(t, "15:00:00", first["start_time"])
	assert.Equal(t, "Fußball AG", first["title"])
	changes, ok := first["changes"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"room", "title"}, changes)
}

func TestEditedInWindow_IncludeDeletionsForwarded(t *testing.T) {
	t.Parallel()

	var gotInclude bool
	mat := &mockMaterializationService{
		detectFn: func(_ int64, _, _ timezone.Date, includeDeletions bool) ([]scheduleSvc.EditedOccurrence, error) {
			gotInclude = includeDeletions
			return nil, nil
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mat})
	router := setupEditedInWindowRouter(rs)

	w := doGet(t, router, "/instances/edited-in-window?activity_group_id=7&from=2026-04-20&to=2026-04-26&include_deletions=true")
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, gotInclude, "include_deletions=true must reach the service")
}

func TestEditedInWindow_EmptyResultIsArray(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{
		detectFn: func(_ int64, _, _ timezone.Date, _ bool) ([]scheduleSvc.EditedOccurrence, error) {
			return nil, nil
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mat})
	router := setupEditedInWindowRouter(rs)

	w := doGet(t, router, "/instances/edited-in-window?activity_group_id=7&from=2026-04-20&to=2026-04-26")

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(0), data["count"])
	occ, ok := data["occurrences"].([]any)
	require.True(t, ok, "occurrences must be [] not null")
	assert.Empty(t, occ)
}

func TestEditedInWindow_MissingParams(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{MaterializationService: &mockMaterializationService{}})
	router := setupEditedInWindowRouter(rs)

	for _, path := range []string{
		"/instances/edited-in-window",
		"/instances/edited-in-window?activity_group_id=7",
		"/instances/edited-in-window?activity_group_id=7&from=2026-04-20",
		"/instances/edited-in-window?from=2026-04-20&to=2026-04-26",
	} {
		w := doGet(t, router, path)
		assert.Equal(t, http.StatusBadRequest, w.Code, "path %s should 400", path)
	}
}

func TestEditedInWindow_InvalidActivityGroupID(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{MaterializationService: &mockMaterializationService{}})
	router := setupEditedInWindowRouter(rs)

	for _, id := range []string{"abc", "0", "-3"} {
		w := doGet(t, router, "/instances/edited-in-window?activity_group_id="+id+"&from=2026-04-20&to=2026-04-26")
		assert.Equal(t, http.StatusBadRequest, w.Code, "activity_group_id=%s should 400", id)
	}
}

func TestEditedInWindow_InvalidDates(t *testing.T) {
	t.Parallel()

	rs := NewResource(Dependencies{MaterializationService: &mockMaterializationService{}})
	router := setupEditedInWindowRouter(rs)

	cases := []string{
		"activity_group_id=7&from=notadate&to=2026-04-26",   // bad from
		"activity_group_id=7&from=2026-04-20&to=notadate",   // bad to
		"activity_group_id=7&from=2026-04-26&to=2026-04-20", // to before from
	}
	for _, q := range cases {
		w := doGet(t, router, "/instances/edited-in-window?"+q)
		assert.Equal(t, http.StatusBadRequest, w.Code, "query %s should 400", q)
	}
}

// TestEditedInWindow_LongWindowNotCapped guards the #1907 critical: a series
// re-plan spans the whole calendar period (custom periods have no maximum), and
// the advisory frontend probe fail-opens on error. A wide window must therefore
// be accepted (200), never rejected, so it always covers the full re-plan range.
func TestEditedInWindow_LongWindowNotCapped(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{
		detectFn: func(_ int64, _, _ timezone.Date, _ bool) ([]scheduleSvc.EditedOccurrence, error) {
			return nil, nil
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mat})
	router := setupEditedInWindowRouter(rs)

	// ~3 years — well beyond any previous cap.
	w := doGet(t, router, "/instances/edited-in-window?activity_group_id=7&from=2026-01-01&to=2028-12-31")
	assert.Equal(t, http.StatusOK, w.Code, "a wide window must be accepted, body: %s", w.Body.String())
}

func TestEditedInWindow_ServiceError(t *testing.T) {
	t.Parallel()

	mat := &mockMaterializationService{
		detectFn: func(_ int64, _, _ timezone.Date, _ bool) ([]scheduleSvc.EditedOccurrence, error) {
			return nil, errors.New("boom")
		},
	}
	rs := NewResource(Dependencies{MaterializationService: mat})
	router := setupEditedInWindowRouter(rs)

	w := doGet(t, router, "/instances/edited-in-window?activity_group_id=7&from=2026-04-20&to=2026-04-26")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
