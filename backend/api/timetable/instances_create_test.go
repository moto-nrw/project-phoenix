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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	facilitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
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
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Create-Room-%d", suffix))

	cleanup := func() {
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)
	}

	mock := &mockInstanceService{}
	res := NewResource(Dependencies{
		ActivityInstanceRepo: scheduleRepo.NewActivityInstanceRepository(db),
		InstanceStaffRepo:    scheduleRepo.NewInstanceStaffRepository(db),
		InstanceStudentRepo:  scheduleRepo.NewInstanceStudentRepository(db),
		RoomRepo:             facilitiesRepo.NewRoomRepository(db),
		ActivityGroupRepo:    activitiesRepo.NewGroupRepository(db),
		InstanceService:      mock,
		DB:                   db,
	})

	return &createSetup{res: res, mock: mock, db: db, ctx: ctx, roomID: room.ID, cleanupFn: cleanup}
}

func createRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
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
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, s.res)

	tomorrow := time.Now().AddDate(0, 0, 1)

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
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", persisted.ID) })
	s.mock.createRes = persisted

	body := map[string]any{
		"date":       tomorrow.Format("2006-01-02"),
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
	assert.True(t, got.IsSpontaneous, "no activity_group_id ⇒ spontaneous")
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
	s := buildCreateSetup(t)
	defer s.cleanupFn()
	router := createRouter(s.ctx, s.res)

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
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
			name: "end before start",
			body: map[string]any{"date": tomorrow, "start_time": "15:00", "end_time": "14:00", "title": "X", "room_id": s.roomID},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doCreate(t, router, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}
