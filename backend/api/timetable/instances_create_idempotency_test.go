package timetable_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apiTest "github.com/moto-nrw/project-phoenix/api/testutil"
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type idempotentCreateSetup struct {
	db       *bun.DB
	router   chi.Router
	claimsID int
	tenantID int64
	roomID   int64
}

func buildIdempotentCreateSetup(t *testing.T) *idempotentCreateSetup {
	t.Helper()
	db, serviceFactory := apiTest.SetupAPITest(t)
	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Create-Idempotent-Room-%d", suffix))
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("create-idempotent-%d", suffix))
	resource := timetableAPI.NewResource(timetableAPI.Dependencies{
		TimetableData:   serviceFactory.TimetableData,
		InstanceService: serviceFactory.Instance,
		DB:              db,
	})
	router := chi.NewRouter()
	router.Mount("/timetable", resource.Router())
	return &idempotentCreateSetup{
		db: db, router: router, claimsID: int(account.ID),
		tenantID: tenant.FromContext(ctx), roomID: room.ID,
	}
}

func nextCreateWorkday() timezone.Date {
	date := timezone.TodayDate().AddDays(1)
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDays(1)
	}
	return date
}

func postIdempotentCreate(
	t *testing.T, setup *idempotentCreateSetup, body any, key string,
) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/timetable/instances/", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	return apiTest.ExecuteWithAuthPermissions(
		t, setup.router, req, apiTest.AdminTestClaims(setup.claimsID),
		[]string{permissions.SchedulesManage},
	)
}

func createdInstanceID(t *testing.T, response *httptest.ResponseRecorder) int64 {
	t.Helper()
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope), "body=%s", response.Body.String())
	return envelope.Data.ID
}

func TestCreateInstance_IdempotencyKeyDeduplicatesOnlyOneCreateOperation(t *testing.T) {
	t.Parallel()
	setup := buildIdempotentCreateSetup(t)
	suffix := time.Now().UnixNano()
	title := fmt.Sprintf("Idempotent manual instance %d", suffix)
	body := map[string]any{
		"date": nextCreateWorkday().String(), "start_time": "10:00", "end_time": "11:00",
		"title": title, "room_id": setup.roomID,
	}

	first := postIdempotentCreate(t, setup, body, fmt.Sprintf("form-%d", suffix))
	require.Equal(t, http.StatusCreated, first.Code, "body=%s", first.Body.String())
	second := postIdempotentCreate(t, setup, body, fmt.Sprintf("form-%d", suffix))
	require.Equal(t, http.StatusCreated, second.Code, "body=%s", second.Body.String())
	assert.Equal(t, createdInstanceID(t, first), createdInstanceID(t, second))

	third := postIdempotentCreate(t, setup, body, fmt.Sprintf("separate-form-%d", suffix))
	require.Equal(t, http.StatusCreated, third.Code, "body=%s", third.Body.String())
	assert.NotEqual(t, createdInstanceID(t, first), createdInstanceID(t, third))

	count, err := setup.db.NewSelect().TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", setup.tenantID).Where("title = ?", title).Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCreateInstance_IdempotencyKeyRejectsDifferentCreateOperation(t *testing.T) {
	t.Parallel()
	setup := buildIdempotentCreateSetup(t)
	suffix := time.Now().UnixNano()
	body := map[string]any{
		"date": nextCreateWorkday().String(), "start_time": "10:00", "end_time": "11:00",
		"title": fmt.Sprintf("Idempotent manual instance %d", suffix), "room_id": setup.roomID,
	}
	key := fmt.Sprintf("form-%d", suffix)
	first := postIdempotentCreate(t, setup, body, key)
	require.Equal(t, http.StatusCreated, first.Code, "body=%s", first.Body.String())

	body["title"] = fmt.Sprintf("Changed manual instance %d", suffix)
	second := postIdempotentCreate(t, setup, body, key)
	assert.Equal(t, http.StatusConflict, second.Code, "body=%s", second.Body.String())
	assert.Contains(t, second.Body.String(), "idempotency_key_reused")
}
