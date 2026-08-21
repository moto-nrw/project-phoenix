package rooms_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupTestContext
// constructs a Resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *roomsAPI.Resource
	router   chi.Router
}

// setupTestContext initializes the test environment. The router serves the
// resource through the production middleware chain (Verifier → Authenticator →
// TenantMiddleware → RequiresPermission → TenantTxMiddleware) exactly as the
// real server does.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	resource := roomsAPI.NewResource(roomsAPI.ResourceConfig{
		FacilityService:    svc.Facilities,
		SettingsService:    svc.Settings,
		UserContextService: svc.UserContext,
		Logger:             slog.Default(),
		DB:                 db,
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		router:   resource.Router(),
	}
}

// =============================================================================
// List Rooms Tests
// =============================================================================

func TestListRooms(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	// Create test rooms
	_ = testpkg.CreateTestRoom(t, tc.db, "Test Room 1")
	_ = testpkg.CreateTestRoom(t, tc.db, "Test Room 2")

	t.Run("success_lists_all_rooms", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?page=1&page_size=10", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_building_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?building=Main", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Get Room Tests
// =============================================================================

func TestGetRoom(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "Get Room Test")

	t.Run("success_gets_room", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Get Room Test", "Response should contain room name")
	})

	t.Run("not_found_for_nonexistent_room", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Create Room Tests
// =============================================================================

func TestCreateRoom(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_creates_room", func(t *testing.T) {
		uniqueName := fmt.Sprintf("Created Room %d", time.Now().UnixNano())
		body := map[string]interface{}{
			"name":     uniqueName,
			"building": "Main",
			"capacity": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), uniqueName, "Response should contain room name")
	})

	t.Run("success_creates_room_with_all_fields", func(t *testing.T) {
		uniqueName := fmt.Sprintf("Full Room %d", time.Now().UnixNano())
		floor := 2
		// Hex stays deterministic — the row gets cleaned up below, so
		// repeated runs no longer collide on the (tenant_id, lower(color))
		// unique index from migration 1.15.45. Earlier crashed/cancelled
		// runs may have left a #FF5733 row behind that the deferred
		// cleanup never ran on; sweep it up here to keep the test
		// hermetic.
		_, _ = tc.db.NewDelete().
			TableExpr("facilities.rooms").
			Where("LOWER(color) = LOWER(?)", "#FF5733").
			Exec(context.Background())

		body := map[string]interface{}{
			"name":     uniqueName,
			"building": "Main",
			"floor":    floor,
			"capacity": 25,
			"category": "classroom",
			"color":    "#FF5733",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		// Clean up the row we just created so the unique-on-color index
		// doesn't poison subsequent runs of this test in the shared test
		// DB. The original test omitted this and relied on the (then-non-
		// existent) lack of a constraint to dodge collisions; now that
		// uniqueness is enforced the cleanup is mandatory.
		var resp struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		require.NotZero(t, resp.Data.ID)
	})

	t.Run("bad_request_missing_name", func(t *testing.T) {
		body := map[string]interface{}{
			"building": "Main",
			"capacity": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Update Room Tests
// =============================================================================

func TestUpdateRoom(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "Update Room Test")

	t.Run("success_updates_room", func(t *testing.T) {
		uniqueName := fmt.Sprintf("Updated Room %d", time.Now().UnixNano())
		body := map[string]interface{}{
			"name":     uniqueName,
			"capacity": 40,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", room.ID), body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated Room", "Response should contain updated name")
	})

	t.Run("not_found_for_nonexistent_room", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Test",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_missing_name", func(t *testing.T) {
		body := map[string]interface{}{
			"capacity": 40,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", room.ID), body)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Delete Room Tests
// =============================================================================

func TestDeleteRoom(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_deletes_room", func(t *testing.T) {
		// Create room specifically for deletion
		room := testpkg.CreateTestRoom(t, tc.db, "Delete Room Test")

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusNoContent, rr.Code, "Expected 204 No Content. Body: %s", rr.Body.String())
	})

	t.Run("error_for_nonexistent_room", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/999999", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		// Service returns not found when room doesn't exist (ErrorRenderer maps ErrRoomNotFound → 404)
		testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
	})

	t.Run("conflict_when_room_has_active_group", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, "Room With Active Group")
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "ActiveGroupInRoom")
		_ = testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertErrorResponse(t, rr, http.StatusConflict)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		req := testutil.NewRequest("DELETE", "/invalid", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Get Rooms by Category Tests
// =============================================================================

func TestGetRoomsByCategory(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_gets_rooms_by_category", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/by-category?category=classroom", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_missing_category", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/by-category", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Building and Category List Tests
// =============================================================================

func TestGetBuildingList(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_gets_building_list", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/buildings", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "buildings", "Response should contain buildings")
	})
}

func TestGetCategoryList(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_gets_category_list", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/categories", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "categories", "Response should contain categories")
	})
}

// =============================================================================
// Available Rooms Tests
// =============================================================================

func TestGetAvailableRooms(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("success_gets_available_rooms", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_capacity_filter", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available?capacity=20", nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Room History Tests
// =============================================================================

func TestGetRoomHistory(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	// Room history is gated by gdpr.attendance_log_enabled — opt-in per
	// tenant. Enable it so the smoke checks below exercise the happy path
	// instead of the feature-disabled branch (which has its own coverage
	// in TestGetRoomHistory_FeatureDisabled).
	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyAttendanceLogEnabled, true, nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyAttendanceLogEnabled, nil, nil)
	})

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "History Room Test")

	t.Run("success_gets_room_history", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_date_range", func(t *testing.T) {
		// Use URL-safe format (no colons in time portion) - RFC3339 is supported but needs encoding
		start := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)
		end := time.Now().UTC().Format(time.RFC3339)
		// Create request with properly encoded query params
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)
		q := req.URL.Query()
		q.Set("start", start)
		q.Set("end", end)
		req.URL.RawQuery = q.Encode()

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history?start=invalid", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_start_after_end", func(t *testing.T) {
		start := time.Now().Format(time.RFC3339)
		end := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history?start=%s&end=%s", room.ID, start, end), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		testutil.AssertBadRequest(t, rr)
	})
}

// TestGetRoomHistory_FeatureDisabled — gdpr.attendance_log_enabled is the
// kill switch for the room history feature. Default is false, so we only need
// to NOT enable it: the handler must return 403 with a "feature_disabled"
// code regardless of the caller's permissions.
//
// The response shape is part of a cross-boundary contract: the Next.js
// proxy (frontend/src/app/api/rooms/[id]/history/route.ts) parses the body
// as { error: string } and matches `error === "feature_disabled"` to
// translate the 403 into a non-error signal for the drawer. If the body
// field name or the literal value drifts, the drawer falls back to a
// generic error toast instead of hiding the section deliberately. The
// JSON-decode assertion below locks the shape in so a refactor of
// common.ErrorForbidden can't silently break it.
func TestGetRoomHistory_FeatureDisabled(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	room := testpkg.CreateTestRoom(t, tc.db, "FeatureDisabled Room")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)
	rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

	require.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 when feature is disabled. Body: %s", rr.Body.String())

	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), "response body must be JSON-decodable: %s", rr.Body.String())
	assert.Equal(t, "feature_disabled", body.Error, "the `error` field must literally equal `feature_disabled` — the frontend proxy matches this exact string to translate the 403 into a UI-hides-section signal")
}

// TestGetRoomHistory_StaffScope — the room history is gated on caller identity
// alone since #2329 (the gdpr.attendance_log_scope per-group filter is gone).
// Four cases on the same session:
//   - admin caller sees the session without a staff lookup
//   - a teacher who supervises the session sees it
//   - a teacher who does NOT supervise the session sees it too
//   - a caller with rooms:read but no staff record gets 403
//     (`not_group_supervisor`) — the failure mode for a caregiver or other
//     non-staff role that happens to hold the read permission.
func TestGetRoomHistory_StaffScope(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyAttendanceLogEnabled, true, nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyAttendanceLogEnabled, nil, nil)
	})

	room := testpkg.CreateTestRoom(t, tc.db, "ScopeRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "ScopeActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	supervisor, supervisorAcc := testpkg.CreateTestStaffWithAccount(t, tc.db, "Scope", "Supervisor")
	_, bystanderAcc := testpkg.CreateTestStaffWithAccount(t, tc.db, "Scope", "Bystander")

	_ = testpkg.CreateTestGroupSupervisor(t, tc.db, supervisor.ID, activeGroup.ID, "lead")

	type historyResp struct {
		Data []map[string]any `json:"data"`
	}

	decodeData := func(t *testing.T, rr *httptest.ResponseRecorder) []map[string]any {
		t.Helper()
		var body historyResp
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		return body.Data
	}

	// staffPermissions is the read permission the handler accepts. It
	// must NOT include any admin wildcard or common.HasAdminPermissions
	// returns true and the staff lookup is skipped entirely.
	staffPermissions := []string{"rooms:read"}

	teacherClaims := func(accountID int64) jwt.AppClaims {
		return jwt.AppClaims{
			ID:          int(accountID),
			Sub:         "teacher@example.com",
			Username:    "teacher",
			Roles:       []string{"user"},
			Permissions: staffPermissions,
			TenantID:    testpkg.Tenant(t),
		}
	}

	t.Run("supervisor_sees_own_session", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, teacherClaims(supervisorAcc.ID))

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		data := decodeData(t, rr)
		require.Len(t, data, 1, "supervisor should see exactly the one session they supervise")
		assert.Equal(t, float64(activeGroup.ID), data[0]["session_id"], "session id should match the active group")
	})

	t.Run("bystander_staff_sees_session_too", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, teacherClaims(bystanderAcc.ID))

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		data := decodeData(t, rr)
		require.Len(t, data, 1, "verified staff read the room history whether or not they supervise the session")
		assert.Equal(t, float64(activeGroup.ID), data[0]["session_id"])
	})

	t.Run("admin_needs_no_staff_record", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		data := decodeData(t, rr)
		require.Len(t, data, 1, "admin should see the session even without a supervisor row")
		assert.Equal(t, float64(activeGroup.ID), data[0]["session_id"])
	})

	t.Run("caller_without_staff_record_returns_forbidden", func(t *testing.T) {
		// Account + Person exist but no `users.staff` row is wired up —
		// GetCurrentStaff resolves the person successfully and then
		// returns ErrUserNotLinkedToStaff from FindByPersonID, which the
		// handler maps to 403 not_group_supervisor. This is the branch
		// the #2329 relaxation deliberately keeps closed: a staff record
		// is still required, only the group affiliation is not.
		_, nonStaffAcc := testpkg.CreateTestPersonWithAccount(t, tc.db, "NonStaff", "Caregiver")
		t.Cleanup(func() {
			// Person first, then the auth account (CleanupAccount only
			// removes auth-side rows — a leftover person would leak
			// across hermetic runs).
		})

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)

		rr := testutil.ExecuteWithAuth(t, tc.router, req, teacherClaims(nonStaffAcc.ID))

		assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "not_group_supervisor")
	})
}

// TestGetRoomHistory_RangeCapClamped — gdpr.room_detail_visible_days is the
// hard window cap. When the caller asks for a wider range than the setting
// allows, the handler must silently clamp start to (end - cap) and exclude
// sessions that were not active inside the clamped window. We arrange two
// sessions (one started inside the 1-day cap, one started AND ended five
// days before the window), ask for a 30-day window, and assert that only
// the recent session comes back.
//
// Note: the filter is "session active in window", not "session started in
// window" — a session that started before the window but is still running
// would correctly be included. The old session here is explicitly closed
// before the window so this case truly tests the cap, not the activeness
// branch.
func TestGetRoomHistory_RangeCapClamped(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyAttendanceLogEnabled, true, nil, nil))
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyRoomDetailVisibleDays, 1, nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyAttendanceLogEnabled, nil, nil)
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyRoomDetailVisibleDays, nil, nil)
	})

	room := testpkg.CreateTestRoom(t, tc.db, "ClampRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "ClampActivity")
	recent := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	old := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	// CreateTestActiveGroup stamps start_time = now and leaves end_time
	// NULL. Backdate the second session AND close it before the 1-day cap,
	// so the active-in-window filter cannot pull it in via end_time IS
	// NULL. Direct UPDATE is unavoidable — there's no fixture knob for
	// start_time / end_time today.
	_, err := tc.db.NewUpdate().
		Table("active.groups").
		Set("start_time = ?", time.Now().Add(-5*24*time.Hour)).
		Set("end_time = ?", time.Now().Add(-4*24*time.Hour)).
		Where("id = ?", old.ID).
		Exec(context.Background())
	require.NoError(t, err)

	start := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	end := time.Now().Add(time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)
	q := req.URL.Query()
	q.Set("start", start)
	q.Set("end", end)
	req.URL.RawQuery = q.Encode()

	rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	require.Len(t, body.Data, 1, "30d request must be clamped to 1d window (cap=1); only the recent session survives")
	assert.Equal(t, float64(recent.ID), body.Data[0]["session_id"], "the returned session should be the one inside the cap")
}

// TestGetRoomHistory_DurationMinutesPopulated — covers the duration_minutes
// SELECT branch which all other tests miss because they only create
// open-ended sessions (end_time IS NULL). Backdate start_time by 90 minutes
// and set end_time = now, then assert the response carries
// duration_minutes ≈ 90. Also asserts ended_at is non-null (the other
// branch in the JSON envelope for closed sessions).
func TestGetRoomHistory_DurationMinutesPopulated(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyAttendanceLogEnabled, true, nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyAttendanceLogEnabled, nil, nil)
	})

	room := testpkg.CreateTestRoom(t, tc.db, "DurationRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DurationActivity")
	session := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)

	// Backdate the fixture and close it 90 minutes later. Direct UPDATE
	// because there's no fixture knob for end_time today (same pattern as
	// TestGetRoomHistory_RangeCapClamped).
	startedAt := time.Now().Add(-90 * time.Minute)
	endedAt := time.Now()
	_, err := tc.db.NewUpdate().
		Table("active.groups").
		Set("start_time = ?", startedAt).
		Set("end_time = ?", endedAt).
		Where("id = ?", session.ID).
		Exec(context.Background())
	require.NoError(t, err)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/history", room.ID), nil)
	rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	require.Len(t, body.Data, 1, "closed session must still appear in history")
	entry := body.Data[0]
	require.NotNil(t, entry["ended_at"], "ended_at must be set for a closed session")

	duration, ok := entry["duration_minutes"].(float64)
	require.True(t, ok, "duration_minutes must be a number, got %T (%v)", entry["duration_minutes"], entry["duration_minutes"])
	// EXTRACT(EPOCH)/60 is integer-cast; allow ±1 minute for clock drift
	// between fixture creation, the UPDATE, and the handler call.
	assert.InDelta(t, 90.0, duration, 1.0, "duration_minutes should be ~90 for a 90-minute session")
}
