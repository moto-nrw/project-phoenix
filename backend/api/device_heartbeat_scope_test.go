package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeviceAuthenticationPersistsSchoolScopedHeartbeat(t *testing.T) {
	t.Parallel()
	// Keep database/sql pool wait channels from crossing the synctest bubble.
	// Fixtures warm this private pool before the virtual clock starts.
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
	synctest.Test(t, func(t *testing.T) {
		var previous *time.Time
		for _, withPIN := range []bool{false, true} {
			// Separate observations on the virtual clock without real-time waiting.
			time.Sleep(time.Second)
			auth := deviceauth.New(deviceauth.Dependencies{Devices: fleet, Settings: &configurationResolverStub{
				ResolveStringForTenantFn: func(_ context.Context, tenantID int64, _ string) (string, error) {
					require.Equal(t, own.TenantID, tenantID)
					return "1234", nil
				},
			}})
			middleware := auth.DeviceOnly()
			if withPIN {
				middleware = auth.Device()
			}
			requestCtx, cancel := context.WithCancel(base)
			request := httptest.NewRequest(http.MethodGet, "/heartbeat", nil).WithContext(requestCtx)
			request.Header.Set("Authorization", "Bearer "+*own.APIKey)
			request.Header.Set("X-Staff-PIN", "1234")
			response := httptest.NewRecorder()
			middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
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
				middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
				require.Equal(t, http.StatusNoContent, response.Code)
				cancel()
				// The second observation must remain unwritten until the full 60 s elapse.
				time.Sleep(59*time.Second - time.Nanosecond)
				synctest.Wait()
				current, err := fleet.FindDevice(ownCtx, own.ID)
				require.NoError(t, err)
				require.Equal(t, previous, current.LastSeen, "a heartbeat inside the debounce window must not write early")
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				require.Eventually(t, func() bool {
					current, readErr := fleet.FindDevice(ownCtx, own.ID)
					return readErr == nil && current.LastSeen != nil && current.LastSeen.After(*previous)
				}, 70*time.Second, 2*time.Second, "deferred heartbeat must retain its school/runtime after request cancellation")
				current, err = fleet.FindDevice(ownCtx, own.ID)
				require.NoError(t, err)
				require.NotNil(t, current.LastSeen)
				require.True(t, current.LastSeen.Equal(observedAt), "deferred heartbeat must persist the second observation, not the flush time")
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
}
