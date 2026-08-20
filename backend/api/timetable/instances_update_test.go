package timetable

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"
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
	t.Parallel()

	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := updateRouter(s.ctx, s.res)

	targetDate := nextTimetableWorkday()
	persisted := testpkg.CreateTestActivityInstance(t, s.db, targetDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM:     "09:00",
		EndHHMM:       "10:00",
		Title:         "Updated Basteln",
		IsSpontaneous: true,
	})
	s.mock.updateRes = persisted

	body := map[string]any{
		"date":       targetDate.String(),
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
	t.Parallel()

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
		name      string
		path      string
		mutate    func(map[string]any)
		updateErr error
		want      int
	}{
		{name: "invalid id", path: "/instances/not-number", mutate: func(_ map[string]any) {}, want: http.StatusBadRequest},
		{name: "missing date", path: "/instances/50", mutate: func(b map[string]any) { b["date"] = "" }, want: http.StatusBadRequest},
		{name: "missing title", path: "/instances/50", mutate: func(b map[string]any) { b["title"] = "" }, want: http.StatusBadRequest},
		{name: "missing time", path: "/instances/50", mutate: func(b map[string]any) { b["start_time"] = "" }, want: http.StatusBadRequest},
		{name: "missing room", path: "/instances/50", mutate: func(b map[string]any) { b["room_id"] = 0 }, want: http.StatusBadRequest},
		{name: "invalid date", path: "/instances/50", mutate: func(b map[string]any) { b["date"] = "tomorrow" }, want: http.StatusBadRequest},
		{name: "weekend date on unknown instance", path: "/instances/50", mutate: func(b map[string]any) { b["date"] = "2026-05-09" }, updateErr: scheduleSvc.ErrInstanceNotFound, want: http.StatusNotFound},
		{name: "invalid start", path: "/instances/50", mutate: func(b map[string]any) { b["start_time"] = "soon" }, want: http.StatusBadRequest},
		{name: "invalid end", path: "/instances/50", mutate: func(b map[string]any) { b["end_time"] = "later" }, want: http.StatusBadRequest},
		{name: "end before start", path: "/instances/50", mutate: func(b map[string]any) { b["end_time"] = "10:30" }, want: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s.mock.updateErr = tc.updateErr
			body := make(map[string]any, len(valid))
			for key, value := range valid {
				body[key] = value
			}
			tc.mutate(body)
			w := doTemplateJSON(t, router, http.MethodPut, tc.path, body)
			assert.Equal(t, tc.want, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestUpdateInstance_ServiceErrors(t *testing.T) {
	t.Parallel()

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

	s.mock.updateErr = timetablePgErrorWithConstraint("23505", "idx_activity_instances_template_unique")
	duplicate := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusConflict, duplicate.Code)
	assert.Contains(t, duplicate.Body.String(), "duplicate_instance")

	s.mock.updateErr = fmt.Errorf("database down")
	internal := doTemplateJSON(t, router, http.MethodPut, "/instances/50", body)
	assert.Equal(t, http.StatusInternalServerError, internal.Code)
}

func timetablePgErrorWithConstraint(code, constraintName string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code, 'n': constraintName}
	return pgErr
}

func TestUpdateInstance_UnwiredAndEnrichmentFailure(t *testing.T) {
	t.Parallel()

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
		Date:          timezone.NewDate(2026, 5, 6),
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
