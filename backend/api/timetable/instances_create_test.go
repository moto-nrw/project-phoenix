// Hermetic-ish integration test for POST /api/timetable/instances.
//
// Uses the same mockInstanceService pattern as instances_test.go for the
// service layer, but exercises the full handler stack (request decoding,
// validation, time parsing, enrichment) against a real DB-backed
// ActivityInstanceRepo / staff / student / room / activity-group repos.
// That way we cover the JSON contract end-to-end without standing up the
// full active.* + materialization service tree.
package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type createSetup struct {
	res       *Resource
	mock      *mockInstanceService
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	cleanupFn func()
}

func buildCreateSetup(t *testing.T) *createSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Create-Room-%d", suffix))

	cleanup := func() {
	}

	mock := &mockInstanceService{}
	res := NewResource(Dependencies{
		TimetableData:   testTimetableData(db),
		InstanceService: mock,
		DB:              db,
	})

	return &createSetup{res: res, mock: mock, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func createRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/instances", res.createInstance)
	return r
}

func doCreate(t *testing.T, router chi.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/instances", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func nextTimetableWorkday() timezone.Date {
	date := timezone.TodayDate().AddDays(1)
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDays(1)
	}
	return date
}

func decodeCreate(t *testing.T, w *httptest.ResponseRecorder) enrichedInstance {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out enrichedInstance
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

func TestCreateInstance_Spontaneous(t *testing.T) {
	t.Parallel()

	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, s.res)

	tomorrow := nextTimetableWorkday()

	// The mock returns a real-looking instance row; the handler still calls
	// enrichInstance against the real DB so room_name + counts are exercised.
	// We persist the row directly so the staff/student lookups have a real
	// row to query — sidesteps the InstanceService panic-on-nil-deps issue.
	persisted := testpkg.CreateTestActivityInstance(t, s.db, tomorrow, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM:     "14:00",
		EndHHMM:       "15:00",
		Title:         "Spontane Bastelstunde",
		IsSpontaneous: true,
	})
	s.mock.createRes = persisted

	body := map[string]any{
		"date":       tomorrow.String(),
		"start_time": "14:00",
		"end_time":   "15:00",
		"title":      "Spontane Bastelstunde",
		"room_id":    s.roomID,
	}

	w := doCreate(t, router, body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	got := decodeCreate(t, w)
	assert.Equal(t, persisted.ID, got.ID)
	assert.Equal(t, "Spontane Bastelstunde", got.Title)
	assert.Equal(t, "14:00", got.StartTime)
	assert.Equal(t, "15:00", got.EndTime)
	assert.Equal(t, scheduleModel.InstanceStatusPlanned, got.Status)
	assert.True(t, got.IsSpontaneous, "is_spontaneous is serialized from the service result")
	assert.False(t, got.IsLive)
	assert.Equal(t, s.roomID, got.RoomID)
	assert.NotEmpty(t, got.RoomName, "room name should resolve via RoomRepo")

	// Verify the service was called with the parsed inputs.
	require.NotNil(t, s.mock.lastCreate)
	assert.Equal(t, "Spontane Bastelstunde", s.mock.lastCreate.Title)
	assert.Equal(t, s.roomID, s.mock.lastCreate.RoomID)
	assert.Nil(t, s.mock.lastCreate.ActivityGroupID, "spontaneous ⇒ no template")
}

func TestCreateInstance_Validation(t *testing.T) {
	t.Parallel()

	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, s.res)

	tomorrow := nextTimetableWorkday().String()
	weekend := timezone.NewDate(2026, 8, 24)
	for weekend.Weekday() != time.Saturday {
		weekend = weekend.AddDays(1)
	}
	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing date",
			body: map[string]any{"start_time": "14:00", "end_time": "15:00", "title": "X", "room_id": s.roomID},
		},
		{
			name: "missing title",
			body: map[string]any{"date": tomorrow, "start_time": "14:00", "end_time": "15:00", "room_id": s.roomID},
		},
		{
			name: "missing room_id",
			body: map[string]any{"date": tomorrow, "start_time": "14:00", "end_time": "15:00", "title": "X"},
		},
		{
			name: "invalid time format",
			body: map[string]any{"date": tomorrow, "start_time": "2:00 PM", "end_time": "15:00", "title": "X", "room_id": s.roomID},
		},
		{
			name: "invalid date format",
			body: map[string]any{"date": "tomorrow", "start_time": "14:00", "end_time": "15:00", "title": "X", "room_id": s.roomID},
		},
		{
			name: "invalid end time format",
			body: map[string]any{"date": tomorrow, "start_time": "14:00", "end_time": "later", "title": "X", "room_id": s.roomID},
		},
		{
			name: "title too long",
			body: map[string]any{"date": tomorrow, "start_time": "14:00", "end_time": "15:00", "title": strings.Repeat("x", 256), "room_id": s.roomID},
		},
		{
			name: "end before start",
			body: map[string]any{"date": tomorrow, "start_time": "15:00", "end_time": "14:00", "title": "X", "room_id": s.roomID},
		},
		{
			name: "weekend date",
			body: map[string]any{"date": weekend.String(), "start_time": "14:00", "end_time": "15:00", "title": "X", "room_id": s.roomID},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doCreate(t, router, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestCreateInstance_TemplateBoundAndErrorBranches(t *testing.T) {
	t.Parallel()

	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, s.res)

	template := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("Create-Bound-%d", time.Now().UnixNano()))
	templateID := template.ID
	tomorrow := nextTimetableWorkday()
	persisted := testpkg.CreateTestActivityInstance(t, s.db, tomorrow, s.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &templateID,
		StartHHMM:       "10:00",
		EndHHMM:         "11:00",
		Title:           "Template-bound extra slot",
	})
	s.mock.createRes = persisted

	body := map[string]any{
		"date":              tomorrow.String(),
		"start_time":        "10:00",
		"end_time":          "11:00",
		"title":             "Template-bound extra slot",
		"room_id":           s.roomID,
		"activity_group_id": templateID,
		"student_ids":       []int64{70, 80},
	}

	w := doCreate(t, router, body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	require.NotNil(t, s.mock.lastCreate)
	require.NotNil(t, s.mock.lastCreate.ActivityGroupID)
	assert.Equal(t, templateID, *s.mock.lastCreate.ActivityGroupID)
	assert.Equal(t, []int64{70, 80}, s.mock.lastCreate.StudentIDs)

	s.mock.createErr = errors.New("insert failed")
	errW := doCreate(t, router, body)
	assert.Equal(t, http.StatusInternalServerError, errW.Code)

	s.mock.createErr = fmt.Errorf("wrapped: %w", scheduleSvc.ErrInvalidInstanceReference)
	refW := doCreate(t, router, body)
	assert.Equal(t, http.StatusBadRequest, refW.Code)

	s.mock.createErr = fmt.Errorf("wrapped: %w", scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod)
	periodW := doCreate(t, router, body)
	assert.Equal(t, http.StatusBadRequest, periodW.Code)
}

func TestCreateInstance_DuplicateTemplateBoundReturnsConflict(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Create-Dupe-Room-%d", suffix))
	template := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Create-Dupe-Template-%d", suffix))
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("Create-Dupe-Period-%d", suffix),
		timezone.NewDate(2026, 8, 24).AddDays(-1), timezone.NewDate(2026, 8, 24).AddDays(7))
	testpkg.SetCalendarPeriodActive(t, db, period, true)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("schedule.activity_instances").
			Where("activity_group_id = ?", template.ID).
			Where("tenant_id = ?", tenant.FromContext(ctx)).
			Exec(ctx)
	})

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	res := NewResource(Dependencies{
		TimetableData:   testTimetableData(db),
		InstanceService: serviceFactory.Instance,
		DB:              db,
	})
	router := createRouter(ctx, res)

	body := map[string]any{
		"date":              nextTimetableWorkday().String(),
		"start_time":        "10:00",
		"end_time":          "11:00",
		"title":             "Duplicate slot",
		"room_id":           room.ID,
		"activity_group_id": template.ID,
	}

	first := doCreate(t, router, body)
	require.Equal(t, http.StatusCreated, first.Code, "body=%s", first.Body.String())

	second := doCreate(t, router, body)
	assert.Equal(t, http.StatusConflict, second.Code, "body=%s", second.Body.String())
	assert.Contains(t, second.Body.String(), "duplicate_instance")
}

func TestCreateInstance_UnwiredResource(t *testing.T) {
	t.Parallel()

	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, NewResource(Dependencies{InstanceService: s.mock}))

	body := map[string]any{
		"date":       nextTimetableWorkday().String(),
		"start_time": "10:00",
		"end_time":   "11:00",
		"title":      "Unwired",
		"room_id":    s.roomID,
	}
	w := doCreate(t, router, body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
