package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	"github.com/moto-ogs/project-phoenix/modules/devicefleet/deviceauth"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDeviceAuthenticationPersistsSchoolScopedHeartbeat(t *testing.T) {
	t.Parallel()
	// A private pool keeps database/sql wait channels from crossing the
	// synctest bubble; the shared SetupTestDB pool would hand them to parallel
	// tests. Fixtures warm this pool before the virtual clock starts.
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	database := testpkg.NewDatabase(t, db)
	fleet, err := owners.NewDeviceFleet(database)
	require.NoError(t, err)
	own := testpkg.CreateTestDevice(t, db, "heartbeat-own")
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	other := testpkg.CreateTestDeviceForTenant(t, db, otherSchool, "heartbeat-other")
	base := testpkg.ContextWithTenantRuntime(context.Background(), database.UnitOfWork())
	require.ErrorIs(t, fleet.UpdateDeviceLastSeen(base, own.ID, time.Now()), devicefleet.ErrTenantRequired)

	ownCtx := testpkg.ContextForTenant(base, own.TenantID)
	schools, err := NewDeviceSchools(database, database.UnitOfWork())
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, database.UnitOfWork())
	require.NoError(t, err)
	pin, err := settings.Settings.ResolveStringForTenant(base, own.TenantID, "security.ogs_device_pin")
	require.NoError(t, err)
	require.NotEmpty(t, pin)
	synctest.Test(t, func(t *testing.T) {
		var previous *time.Time
		for _, withPIN := range []bool{false, true} {
			// Separate observations on the virtual clock without real-time waiting.
			time.Sleep(time.Second)
			auth := testutil.NewDeviceAuthenticators(fleet, schools, settings.Settings)
			middleware := auth.DeviceOnly()
			if withPIN {
				middleware = auth.Device()
			}
			requestCtx, cancel := context.WithCancel(base)
			request := httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(requestCtx)
			request.Header.Set("Authorization", "Bearer "+*own.APIKey)
			request.Header.Set("X-Staff-PIN", pin)
			counter := testpkg.CaptureQueriesForContext(t, db)
			request = request.WithContext(counter.Context(request.Context()))
			response := httptest.NewRecorder()
			middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			wantQueries := 9
			if withPIN {
				wantQueries = 10
			}
			require.Len(t, counter.Queries(), wantQueries, "cold auth includes fresh school discovery and independent heartbeat")
			found, err := fleet.FindDevice(ownCtx, own.ID)
			require.NoError(t, err)
			require.NotNil(t, found.LastSeen, "tenantless authentication must write under the authenticated device's school")
			if previous != nil {
				require.True(t, found.LastSeen.After(*previous), "PIN authentication must also persist its heartbeat")
			}
			previous = found.LastSeen
			if withPIN {
				// Exercise the unchanged production debounce window on the virtual clock.
				time.Sleep(time.Second)
				observedAt := time.Now()
				response = httptest.NewRecorder()
				counter.Reset()
				middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
				require.Equal(t, http.StatusNoContent, response.Code)
				require.Len(t, counter.Queries(), 9, "warm auth still checks the school; only the heartbeat is debounced")
				cancel()
				// The second observation must remain unwritten until the full 60 s elapse.
				time.Sleep(59*time.Second - time.Nanosecond)
				synctest.Wait()
				current, err := fleet.FindDevice(ownCtx, own.ID)
				require.NoError(t, err)
				require.Equal(t, previous, current.LastSeen, "a heartbeat inside the debounce window must not write early")
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				// synctest.Wait returned, so the deferred flush has finished.
				current, err = fleet.FindDevice(ownCtx, own.ID)
				require.NoError(t, err)
				require.NotNil(t, current.LastSeen, "deferred heartbeat must retain its school/runtime after request cancellation")
				require.True(t, current.LastSeen.After(*previous), "deferred heartbeat must retain its school/runtime after request cancellation")
				require.True(t, current.LastSeen.Equal(observedAt), "deferred heartbeat must persist the second observation, not the flush time")
				_, err = db.ExecContext(base, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, own.TenantID)
				require.NoError(t, err)
				// Reuse the warmed authenticator and credential, but not the cancelled
				// request context. School deletion must beat a missing PIN immediately.
				request = httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(base)
				request.Header.Set("Authorization", "Bearer "+*own.APIKey)
				response = httptest.NewRecorder()
				middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("deleted school admitted") })).ServeHTTP(response, request)
				require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
				require.Contains(t, response.Body.String(), `"code":"iot.device_inactive"`)
			} else {
				cancel()
			}
		}
	})
	foreign, err := fleet.FindDevice(testpkg.ContextForTenant(base, otherSchool), other.ID)
	require.NoError(t, err)
	require.Nil(t, foreign.LastSeen)
	require.Error(t, fleet.UpdateDeviceLastSeen(ownCtx, other.ID, time.Now()))
	foreign, err = fleet.FindDevice(testpkg.ContextForTenant(base, otherSchool), other.ID)
	require.NoError(t, err)
	require.Nil(t, foreign.LastSeen, "a heartbeat must not update another school")
	require.NoError(t, testpkg.WithinTenantContext(t, base, db, otherSchool, func(ctx context.Context) error {
		counter := testpkg.CaptureQueriesForContext(t, db)
		auth := testutil.NewDeviceAuthenticators(fleet, schools, settings.Settings)
		request := httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(counter.Context(ctx))
		request.Header.Set("Authorization", "Bearer "+*own.APIKey)
		response := httptest.NewRecorder()
		auth.DeviceOnly()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("foreign credential admitted") })).ServeHTTP(response, request)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), `"code":"iot.device_api_key_invalid"`)
		require.Len(t, counter.Queries(), 1, "wrong-tenant credentials must fail before school discovery or a scope switch")
		_, err := fleet.FindDevice(ctx, other.ID)
		return err
	}))
}

// The injected school port fails a real SQL statement, not merely a Go call.
// The transient wrapper exercises the existing classifier after PostgreSQL has
// already marked the discovery transaction as aborted.
type failedDeviceSchoolRead struct{ transient bool }

func (s failedDeviceSchoolRead) FindSchool(ctx context.Context, _ int64) (*deviceauth.School, error) {
	raw, _ := tenant.TransactionFromContext(ctx)
	tx := raw.(bun.Tx)
	var value int
	err := tx.NewRaw("SELECT 1 / 0").Scan(ctx, &value)
	if s.transient {
		err = errors.Join(context.DeadlineExceeded, err)
	}
	return nil, err
}

func TestDeviceAuthenticationSchoolReadFailureIsIsolated(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		participating bool
		transient     bool
	}{
		{"standalone transient", false, true},
		{"standalone permanent", false, false},
		{"participating transient", true, true},
		{"participating permanent", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, settings := setupIoTSettingsModule(t)
			database := testpkg.NewDatabase(t, db)
			fleet, err := owners.NewDeviceFleet(database)
			require.NoError(t, err)
			own := testpkg.CreateTestDevice(t, db, "failed-school-read")
			auth := testutil.NewDeviceAuthenticators(fleet, failedDeviceSchoolRead{transient: tc.transient}, settings)
			base := testpkg.ContextWithTenantRuntime(context.Background(), database.UnitOfWork())
			counter := testpkg.CaptureQueriesForContext(t, db)
			handlerFailure := errors.New("handler rolled back")
			handlerReached := false
			serve := func(ctx context.Context) error {
				request := httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(counter.Context(ctx))
				request.Header.Set("Authorization", "Bearer "+*own.APIKey)
				request.Header.Set("X-Staff-PIN", "1234")
				response := httptest.NewRecorder()
				auth.Device()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					handlerReached = true
					// This owner read must work after the failed optional school SQL.
					_, readErr := fleet.FindDevice(r.Context(), own.ID)
					require.NoError(t, readErr)
					w.WriteHeader(http.StatusUnprocessableEntity)
				})).ServeHTTP(response, request)
				if tc.transient {
					require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
				} else {
					require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
					require.Contains(t, response.Body.String(), `"code":"iot.device_inactive"`)
				}
				return handlerFailure
			}
			if tc.participating {
				err = testpkg.WithinTenantContext(t, base, db, own.TenantID, serve)
			} else {
				err = serve(base)
			}
			require.ErrorIs(t, err, handlerFailure)
			require.Equal(t, tc.transient, handlerReached)
			found, err := fleet.FindDevice(testpkg.ContextForTenant(base, own.TenantID), own.ID)
			require.NoError(t, err)
			if tc.transient {
				require.NotNil(t, found.LastSeen, "heartbeat survives a later handler rollback")
				require.Len(t, counter.Operation("UPDATE"), 1)
			} else {
				require.Nil(t, found.LastSeen)
				require.Empty(t, counter.Operation("UPDATE"))
			}
			t.Logf("school failure SQL: %q", counter.Queries())
		})
	}
}

// A valid PIN heartbeat must already be visible before a handler opens its own
// write unit. Rolling that unit back must undo its writes, not the heartbeat.
func TestDevicePINHeartbeatSurvivesHandlerRollback(t *testing.T) {
	t.Parallel()
	db, settings := setupIoTSettingsModule(t)
	database := testpkg.NewDatabase(t, db)
	fleet, err := owners.NewDeviceFleet(database)
	require.NoError(t, err)
	schools, err := NewDeviceSchools(database, database.UnitOfWork())
	require.NoError(t, err)
	own := testpkg.CreateTestDevice(t, db, "pin-heartbeat-rollback")
	room := testpkg.CreateTestRoom(t, db, "rolled-back-device-room")
	base := testpkg.ContextWithTenantRuntime(context.Background(), database.UnitOfWork())
	ownCtx := testpkg.ContextForTenant(base, own.TenantID)
	auth := testutil.NewDeviceAuthenticators(fleet, schools, settings)
	request := func(pin string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(base)
		r.Header.Set("Authorization", "Bearer "+*own.APIKey)
		r.Header.Set("X-Staff-PIN", pin)
		return r
	}
	response := httptest.NewRecorder()
	auth.Device()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("wrong PIN admitted")
	})).ServeHTTP(response, request("wrong"))
	require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"code":"iot.device_pin_invalid"`)
	found, err := fleet.FindDevice(ownCtx, own.ID)
	require.NoError(t, err)
	require.Nil(t, found.LastSeen, "a rejected PIN must not persist a heartbeat")
	var committed *time.Time
	handlerFailure := errors.New("handler rolled back")
	response = httptest.NewRecorder()
	auth.Device()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		found, readErr := fleet.FindDevice(ownCtx, own.ID)
		require.NoError(t, readErr)
		require.NotNil(t, found.LastSeen, "heartbeat commits before entering the handler")
		committed = found.LastSeen
		writeErr := testpkg.WithinTenantContext(t, r.Context(), db, own.TenantID, func(ctx context.Context) error {
			require.NoError(t, fleet.UpdateRoomID(ctx, own.ID, room.ID))
			return handlerFailure
		})
		require.ErrorIs(t, writeErr, handlerFailure)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})).ServeHTTP(response, request("1234"))
	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
	found, err = fleet.FindDevice(ownCtx, own.ID)
	require.NoError(t, err)
	require.NotNil(t, committed, "the valid request must reach the handler after the rejected attempt")
	require.Equal(t, committed, found.LastSeen)
	require.Nil(t, found.RoomID, "the handler's mutation must roll back independently")
}
