package display_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	displayAPI "github.com/moto-nrw/project-phoenix/api/display"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	displayService "github.com/moto-nrw/project-phoenix/services/display"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test constructs a Resource via
// jwt.MustNewTokenAuth (CI runs without a .env, so AUTH_JWT_SECRET is unset).
func init() {
	testutil.SeedTestJWTConfig()
}

// displayTenantCounter seeds unique tenant IDs for this package. Deliberately
// SMALL numbers (not UnixNano): the tenant ID travels through JWT claims,
// which JSON-decode as float64 — IDs above 2^53 silently lose precision and
// the FK to platform.schools no longer matches. Uniqueness only needs to hold
// within this package's cloned test database.
var displayTenantCounter int64 = 780_000 + time.Now().UnixNano()%100_000

func newDisplayTestTenant(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	tenantID := atomic.AddInt64(&displayTenantCounter, 1)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return tenantID
}

func newDisplayRouter(t *testing.T, db *bun.DB) http.Handler {
	t.Helper()
	repos := repositories.NewFactory(db)
	svc := displayService.NewService(displayService.Dependencies{
		DisplayRepo:       repos.Display,
		SchoolRepo:        repos.School,
		Facilities:        facilities.NewService(repos.Room, repos.ActiveGroup),
		ActiveGroupRepo:   repos.ActiveGroup,
		VisitRepo:         repos.ActiveVisit,
		ActivityGroupRepo: repos.ActivityGroup,
		InstanceRepo:      repos.ActivityInstance,
		AttendanceRepo:    repos.Attendance,
		PickupSchedule:    schedule.NewPickupScheduleService(repos.StudentPickupSchedule, repos.StudentPickupException, repos.StudentPickupNote),
		DB:                db,
	})
	return displayAPI.NewResource(svc, db).Router()
}

func displayTestJWT(t *testing.T, accountID, tenantID int64, permissions []string) string {
	t.Helper()
	return testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          int(accountID),
		Sub:         fmt.Sprintf("%d", accountID),
		Roles:       []string{"user"},
		Permissions: permissions,
		TenantID:    tenantID,
	})
}

func doDisplayRequest(t *testing.T, router http.Handler, method, path, jwtToken string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// doDashboardRequest fetches the public dashboard with the display token in
// the X-Display-Token header — the token never travels in the URL.
func doDashboardRequest(t *testing.T, router http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	if token != "" {
		req.Header.Set("X-Display-Token", token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// createDisplayViaAPI creates a display through the admin endpoint and
// returns its ID and one-time raw token.
func createDisplayViaAPI(t *testing.T, router http.Handler, adminJWT, name string) (string, string) {
	t.Helper()
	rec := doDisplayRequest(t, router, http.MethodPost, "/", adminJWT, map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, rec.Code, "create display failed: %s", rec.Body.String())

	var created struct {
		Display struct {
			ID int64 `json:"id"`
		} `json:"display"`
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Token, "create must return the raw token once")
	return fmt.Sprintf("%d", created.Display.ID), created.Token
}

func cleanupDisplays(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("display.displays").
		Where("tenant_id = ?", tenantID).
		Exec(ctx)
}

func TestDisplayAdminCRUD(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantID := newDisplayTestTenant(t, db)
	defer cleanupDisplays(t, db, tenantID)
	router := newDisplayRouter(t, db)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("display-admin-%d@test.local", tenantID))
	adminJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:read", "display:manage"})

	t.Run("unauthenticated list is rejected", func(t *testing.T) {
		rec := doDisplayRequest(t, router, http.MethodGet, "/", "", nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing permission is rejected", func(t *testing.T) {
		readOnlyJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:read"})
		rec := doDisplayRequest(t, router, http.MethodPost, "/", readOnlyJWT, map[string]any{"name": "Nope"})
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("list accepts display:read OR display:manage", func(t *testing.T) {
		readOnlyJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:read"})
		rec := doDisplayRequest(t, router, http.MethodGet, "/", readOnlyJWT, nil)
		assert.Equal(t, http.StatusOK, rec.Code)

		manageOnlyJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:manage"})
		rec = doDisplayRequest(t, router, http.MethodGet, "/", manageOnlyJWT, nil)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("create returns raw token exactly once and never the hash", func(t *testing.T) {
		rec := doDisplayRequest(t, router, http.MethodPost, "/", adminJWT, map[string]any{"name": "Eingangsbereich"})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "token_hash")

		listRec := doDisplayRequest(t, router, http.MethodGet, "/", adminJWT, nil)
		require.Equal(t, http.StatusOK, listRec.Code)
		assert.NotContains(t, listRec.Body.String(), "token_hash")
		assert.Contains(t, listRec.Body.String(), "Eingangsbereich")
	})

	t.Run("create requires a name", func(t *testing.T) {
		rec := doDisplayRequest(t, router, http.MethodPost, "/", adminJWT, map[string]any{"name": "  "})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("update renames and deactivates", func(t *testing.T) {
		id, rawToken := createDisplayViaAPI(t, router, adminJWT, "Flur EG")

		rec := doDisplayRequest(t, router, http.MethodPatch, "/"+id, adminJWT, map[string]any{"name": "Flur OG", "is_active": false})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Flur OG")

		dashRec := doDashboardRequest(t, router, rawToken)
		require.Equal(t, http.StatusOK, dashRec.Code)
		assert.Contains(t, dashRec.Body.String(), `"status":"inactive"`)
	})

	t.Run("regenerate invalidates the old token", func(t *testing.T) {
		id, oldToken := createDisplayViaAPI(t, router, adminJWT, "Mensa")

		rec := doDisplayRequest(t, router, http.MethodPost, "/"+id+"/regenerate", adminJWT, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var regen struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &regen))
		require.NotEmpty(t, regen.Token)
		require.NotEqual(t, oldToken, regen.Token)

		oldRec := doDashboardRequest(t, router, oldToken)
		assert.Equal(t, http.StatusNotFound, oldRec.Code)
		newRec := doDashboardRequest(t, router, regen.Token)
		assert.Equal(t, http.StatusOK, newRec.Code)
	})

	t.Run("delete kills the dashboard", func(t *testing.T) {
		id, rawToken := createDisplayViaAPI(t, router, adminJWT, "Turnhalle")

		rec := doDisplayRequest(t, router, http.MethodDelete, "/"+id, adminJWT, nil)
		require.Equal(t, http.StatusNoContent, rec.Code)

		dashRec := doDashboardRequest(t, router, rawToken)
		assert.Equal(t, http.StatusNotFound, dashRec.Code)
	})
}

func TestDisplayDashboardPublic(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantID := newDisplayTestTenant(t, db)
	defer cleanupDisplays(t, db, tenantID)
	router := newDisplayRouter(t, db)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("display-dash-%d@test.local", tenantID))
	adminJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:manage"})

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, "Bauraum")
	activityGroup := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Fußball AG")
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, tenantID, activityGroup.ID, room.ID)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Emma", "Testkind", "1a")
	testpkg.CreateTestVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, timezone.Now().Add(-30*time.Minute), nil)
	defer testpkg.CleanupActivityFixturesForTenant(t, db, tenantID, room.ID, activityGroup.ID, activeGroup.ID, student.ID)

	_, rawToken := createDisplayViaAPI(t, router, adminJWT, "Eingang")

	t.Run("unknown token yields 404", func(t *testing.T) {
		rec := doDashboardRequest(t, router, "definitely-not-a-token")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("missing token header yields 400", func(t *testing.T) {
		rec := doDashboardRequest(t, router, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("dashboard aggregates without exposing student data", func(t *testing.T) {
		rec := doDashboardRequest(t, router, rawToken)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var payload displayService.DashboardPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

		assert.Equal(t, "active", payload.Status)
		assert.Equal(t, fmt.Sprintf("Test School %d", tenantID), payload.SchoolName)
		assert.Equal(t, "Eingang", payload.DisplayName)
		assert.Equal(t, timezone.TodayDate().String(), payload.Date)

		var bauraum *displayService.RoomOccupancy
		for i := range payload.RoomOccupancy {
			if payload.RoomOccupancy[i].Name == room.Name {
				bauraum = &payload.RoomOccupancy[i]
			}
		}
		require.NotNil(t, bauraum, "room must appear in occupancy list")
		assert.True(t, bauraum.IsOccupied)
		assert.Equal(t, 1, bauraum.StudentCount)

		require.Len(t, payload.RunningActivities, 1)
		assert.Equal(t, activityGroup.Name, payload.RunningActivities[0].Name)
		assert.Equal(t, room.Name, payload.RunningActivities[0].RoomName)
		assert.Equal(t, 1, payload.RunningActivities[0].Participants)
		assert.Equal(t, 1, payload.Totals.ActivitiesRunning)
		assert.Equal(t, 1, payload.Totals.RoomsOccupied)

		// GDPR contract: no student identity leaves this endpoint.
		body := rec.Body.String()
		assert.NotContains(t, body, "Emma")
		assert.NotContains(t, body, "Testkind")
		assert.NotContains(t, body, "first_name")
		assert.NotContains(t, body, "student_id")
	})

	t.Run("upcoming activities list planned instances after now", func(t *testing.T) {
		// The upcoming filter compares wall clocks only, so offsets that
		// cross midnight wrap to the other end of the day and flip the
		// expectation. Small offsets + a narrow guard keep this hermetic.
		if clock := timezone.Now().In(timezone.Berlin).Format("15:04"); clock < "00:05" || clock > "23:54" {
			t.Skip("too close to midnight for stable wall-clock offsets")
		}
		ctx := testpkg.TenantContext(tenantID)
		future := timezone.Now().Add(3 * time.Minute)
		past := timezone.Now().Add(-3 * time.Minute)
		instances := []*scheduleModels.ActivityInstance{
			buildInstance(tenantID, "Hausaufgaben", room.ID, future),
			buildInstance(tenantID, "Frühsport", room.ID, past),
		}
		for _, inst := range instances {
			_, err := db.NewInsert().Model(inst).ModelTableExpr("schedule.activity_instances").Exec(ctx)
			require.NoError(t, err)
		}
		defer func() {
			_, _ = db.NewDelete().TableExpr("schedule.activity_instances").Where("tenant_id = ?", tenantID).Exec(context.Background())
		}()

		rec := doDashboardRequest(t, router, rawToken)
		require.Equal(t, http.StatusOK, rec.Code)

		var payload displayService.DashboardPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Len(t, payload.UpcomingActivities, 1, "past instance must be filtered: %s", rec.Body.String())
		assert.Equal(t, "Hausaufgaben", payload.UpcomingActivities[0].Name)
		assert.Equal(t, future.In(timezone.Berlin).Format("15:04"), payload.UpcomingActivities[0].StartTime)
	})
}

func buildInstance(tenantID int64, title string, roomID int64, start time.Time) *scheduleModels.ActivityInstance {
	inst := &scheduleModels.ActivityInstance{
		Date:      timezone.TodayDate(),
		Title:     title,
		StartTime: timezone.WallClock(start.In(timezone.Berlin)),
		EndTime:   timezone.WallClock(start.In(timezone.Berlin).Add(45 * time.Minute)),
		RoomID:    roomID,
		Status:    scheduleModels.InstanceStatusPlanned,
	}
	inst.SetTenantID(tenantID)
	return inst
}

func TestDisplayDashboardCrossTenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantA := newDisplayTestTenant(t, db)
	tenantB := newDisplayTestTenant(t, db)
	defer cleanupDisplays(t, db, tenantA)
	defer cleanupDisplays(t, db, tenantB)
	router := newDisplayRouter(t, db)

	accountA := testpkg.CreateTestAccount(t, db, fmt.Sprintf("display-a-%d@test.local", tenantA))
	accountB := testpkg.CreateTestAccount(t, db, fmt.Sprintf("display-b-%d@test.local", tenantB))
	jwtA := displayTestJWT(t, accountA.ID, tenantA, []string{"display:read", "display:manage"})
	jwtB := displayTestJWT(t, accountB.ID, tenantB, []string{"display:read", "display:manage"})

	roomB := testpkg.CreateTestRoomForTenant(t, db, tenantB, "Geheimraum B")
	defer testpkg.CleanupActivityFixturesForTenant(t, db, tenantB, roomB.ID)

	_, tokenA := createDisplayViaAPI(t, router, jwtA, "Display A")
	createDisplayViaAPI(t, router, jwtB, "Display B")

	t.Run("dashboard of tenant A never shows tenant B rooms", func(t *testing.T) {
		rec := doDashboardRequest(t, router, tokenA)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.NotContains(t, rec.Body.String(), "Geheimraum B")
		assert.Equal(t, fmt.Sprintf("Test School %d", tenantA), gjsonSchoolName(t, rec.Body.Bytes()))
	})

	t.Run("admin list of tenant A never shows tenant B displays", func(t *testing.T) {
		rec := doDisplayRequest(t, router, http.MethodGet, "/", jwtA, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Display A")
		assert.NotContains(t, rec.Body.String(), "Display B")
	})
}

func gjsonSchoolName(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		SchoolName string `json:"school_name"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload.SchoolName
}

func TestDisplayDashboardPickupBuckets(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	today := timezone.TodayDate()
	weekday := int(today.Weekday())
	if weekday < 1 || weekday > 5 {
		t.Skip("pickup schedules only resolve Mon-Fri; skipping on weekends")
	}

	tenantID := newDisplayTestTenant(t, db)
	defer cleanupDisplays(t, db, tenantID)
	router := newDisplayRouter(t, db)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("display-pickup-%d@test.local", tenantID))
	adminJWT := displayTestJWT(t, account.ID, tenantID, []string{"display:manage"})

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Betreuer", "Test")
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, fmt.Sprintf("DISP-TEST-%d", tenantID))
	present1 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Anna", "Anwesend", "2b")
	present2 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Ben", "Anwesend", "2b")
	checkedOut := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Carla", "Weg", "2b")
	defer testpkg.CleanupActivityFixturesForTenant(t, db, tenantID, staff.ID, device.ID, present1.ID, present2.ID, checkedOut.ID)

	now := timezone.Now()
	checkoutTime := now.Add(-10 * time.Minute)
	createAttendanceForTenant(t, db, tenantID, present1.ID, staff.ID, device.ID, nil)
	createAttendanceForTenant(t, db, tenantID, present2.ID, staff.ID, device.ID, nil)
	createAttendanceForTenant(t, db, tenantID, checkedOut.ID, staff.ID, device.ID, &checkoutTime)

	// Both present students share the 23:45 slot; the checked-out student's
	// schedule must not count.
	createPickupScheduleForTenant(t, db, tenantID, present1.ID, weekday, staff.ID, "23:45")
	createPickupScheduleForTenant(t, db, tenantID, present2.ID, weekday, staff.ID, "23:45")
	createPickupScheduleForTenant(t, db, tenantID, checkedOut.ID, weekday, staff.ID, "23:45")
	defer func() {
		_, _ = db.NewDelete().TableExpr("schedule.student_pickup_schedules").Where("tenant_id = ?", tenantID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("active.attendance").Where("tenant_id = ?", tenantID).Exec(context.Background())
	}()

	if timezone.Now().In(timezone.Berlin).Format("15:04") >= "23:45" {
		t.Skip("too close to midnight for a stable future pickup slot")
	}

	_, rawToken := createDisplayViaAPI(t, router, adminJWT, "Pickup Display")
	rec := doDashboardRequest(t, router, rawToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload displayService.DashboardPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	assert.Equal(t, 2, payload.Totals.StudentsPresent)
	require.Len(t, payload.PickupTimes, 1, "expected one bucket: %s", rec.Body.String())
	assert.Equal(t, "23:45", payload.PickupTimes[0].Time)
	assert.Equal(t, 2, payload.PickupTimes[0].Count)

	// Counts only — never names.
	assert.NotContains(t, rec.Body.String(), "Anna")
	assert.NotContains(t, rec.Body.String(), "Carla")
}

func createAttendanceForTenant(t *testing.T, db *bun.DB, tenantID, studentID, staffID, deviceID int64, checkOut *time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attendance := &activeModels.Attendance{
		StudentID:    studentID,
		Date:         timezone.TodayDate(),
		CheckInTime:  timezone.Now().Add(-4 * time.Hour),
		CheckOutTime: checkOut,
		CheckedInBy:  staffID,
		DeviceID:     deviceID,
	}
	attendance.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(attendance).ModelTableExpr("active.attendance").Exec(ctx)
	require.NoError(t, err, "failed to create attendance")
}

func createPickupScheduleForTenant(t *testing.T, db *bun.DB, tenantID, studentID int64, weekday int, staffID int64, pickupHHMM string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+pickupHHMM)
	require.NoError(t, err)

	row := &scheduleModels.StudentPickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: parsed,
		CreatedBy:  staffID,
	}
	row.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(row).ModelTableExpr("schedule.student_pickup_schedules").Exec(ctx)
	require.NoError(t, err, "failed to create pickup schedule")
}

// Guard against accidental reintroduction of identity fields in the payload
// structs themselves (compile-time-ish check via JSON round trip).
func TestDashboardPayloadHasNoIdentityFields(t *testing.T) {
	payload := displayService.DashboardPayload{}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"first_name", "last_name", "student_id", "photo"} {
		assert.NotContains(t, lower, forbidden)
	}
}
