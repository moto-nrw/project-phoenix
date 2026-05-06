package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func updateRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Put("/instances/{id}", res.updateInstance)
	return r
}

func TestUpdateInstance_Success(t *testing.T) {
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := updateRouter(s.ctx, s.res)

	targetDate := time.Now().AddDate(0, 0, 2)
	persisted := testpkg.CreateTestActivityInstance(t, s.db, targetDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM:     "09:00",
		EndHHMM:       "10:00",
		Title:         "Updated Basteln",
		IsSpontaneous: true,
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", persisted.ID) })
	s.mock.updateRes = persisted

	body := map[string]any{
		"date":       targetDate.Format("2006-01-02"),
		"start_time": "09:00",
		"end_time":   "10:00",
		"title":      "Updated Basteln",
		"room_id":    s.roomID,
		"staff_ids":  []int64{50, 60},
	}

	w := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/instances/%d", persisted.ID), body)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeCreate(t, w)
	assert.Equal(t, persisted.ID, got.ID)
	assert.Equal(t, "Updated Basteln", got.Title)
	assert.Equal(t, "09:00", got.StartTime)
	assert.Equal(t, scheduleModel.InstanceStatusPlanned, got.Status)

	require.NotNil(t, s.mock.lastUpdate)
	assert.Equal(t, "Updated Basteln", s.mock.lastUpdate.Title)
	assert.Equal(t, s.roomID, s.mock.lastUpdate.RoomID)
	assert.Equal(t, "09:00", s.mock.lastUpdate.StartTime.Format("15:04"))
}

func TestUpdateInstance_Validation(t *testing.T) {
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := updateRouter(s.ctx, s.res)

	valid := map[string]any{
		"date":       "2026-05-06",
		"start_time": "11:00",
		"end_time":   "12:00",
		"title":      "Valid",
		"room_id":    s.roomID,
	}

	cases := []struct {
		name   string
		path   string
		mutate func(map[string]any)
	}{
		{name: "invalid id", path: "/instances/not-number", mutate: func(_ map[string]any) {}},
		{name: "missing date", path: "/instances/50", mutate: func(b map[string]any) { b["date"] = "" }},
		{name: "missing title", path: "/instances/50", mutate: func(b map[string]any) { b["title"] = "" }},
		{name: "missing time", path: "/instances/50", mutate: func(b map[string]any) { b["start_time"] = "" }},
		{name: "missing room", path: "/instances/50", mutate: func(b map[string]any) { b["room_id"] = 0 }},
		{name: "invalid date", path: "/instances/50", mutate: func(b map[string]any) { b["date"] = "tomorrow" }},
		{name: "invalid start", path: "/instances/50", mutate: func(b map[string]any) { b["start_time"] = "soon" }},
		{name: "invalid end", path: "/instances/50", mutate: func(b map[string]any) { b["end_time"] = "later" }},
		{name: "end before start", path: "/instances/50", mutate: func(b map[string]any) { b["end_time"] = "10:30" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := make(map[string]any, len(valid))
			for key, value := range valid {
				body[key] = value
			}
			tc.mutate(body)
			w := doTemplateJSON(t, router, http.MethodPut, tc.path, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestUpdateInstance_ServiceErrors(t *testing.T) {
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := updateRouter(s.ctx, s.res)

	body := map[string]any{
		"date":       "2026-05-06",
		"start_time": "11:00",
		"end_time":   "12:00",
		"title":      "Valid",
		"room_id":    s.roomID,
	}

	s.mock.updateErr = scheduleSvc.ErrInstanceNotFound
	notFound := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusNotFound, notFound.Code)

	s.mock.updateErr = &wrappedErr{inner: scheduleSvc.ErrInvalidInstanceTransition}
	conflict := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusConflict, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "invalid_transition")

	s.mock.updateErr = fmt.Errorf("wrapped: %w", scheduleSvc.ErrInvalidInstanceReference)
	badReference := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusBadRequest, badReference.Code)

	s.mock.updateErr = fmt.Errorf("database down")
	internal := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusInternalServerError, internal.Code)
}

func TestUpdateInstance_UnwiredAndEnrichmentFailure(t *testing.T) {
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := updateRouter(s.ctx, NewResource(Dependencies{InstanceService: s.mock}))

	body := map[string]any{
		"date":       "2026-05-06",
		"start_time": "11:00",
		"end_time":   "12:00",
		"title":      "Valid",
		"room_id":    s.roomID,
	}
	s.mock.updateRes = &scheduleModel.ActivityInstance{
		Date:          time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC),
		StartTime:     time.Date(2000, 1, 1, 11, 0, 0, 0, time.UTC),
		EndTime:       time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
		Title:         "Valid",
		RoomID:        s.roomID,
		Status:        scheduleModel.InstanceStatusPlanned,
		IsSpontaneous: true,
	}
	s.mock.updateRes.ID = 777

	w := doTemplateJSON(t, router, http.MethodPut, "/instances/777", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
