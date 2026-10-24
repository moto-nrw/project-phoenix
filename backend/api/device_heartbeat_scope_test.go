package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeviceAuthenticationPersistsSchoolScopedHeartbeat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fleet, err := owners.NewDeviceFleet(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	own := testpkg.CreateTestDevice(t, db, "heartbeat-own")
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	other := testpkg.CreateTestDeviceForTenant(t, db, otherSchool, "heartbeat-other")
	base := testpkg.WithPackageTenantRuntime(context.Background())
	require.ErrorIs(t, fleet.UpdateDeviceLastSeen(base, own.ID, time.Now()), devicefleet.ErrTenantRequired)

	var previous *time.Time
	for _, withPIN := range []bool{false, true} {
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
		found, err := fleet.FindDevice(testpkg.Ctx(t), own.ID)
		require.NoError(t, err)
		require.NotNil(t, found.LastSeen, "tenantless authentication must write under the authenticated device's school")
		if previous != nil {
			require.True(t, found.LastSeen.After(*previous), "PIN authentication must also persist its heartbeat")
		}
		previous = found.LastSeen
		if withPIN {
			// A second observation is deferred for the real debounce window.
			response = httptest.NewRecorder()
			middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			cancel()
			require.Eventually(t, func() bool {
				current, readErr := fleet.FindDevice(testpkg.Ctx(t), own.ID)
				return readErr == nil && current.LastSeen != nil && current.LastSeen.After(*previous)
			}, 70*time.Second, 2*time.Second, "deferred heartbeat must retain its school/runtime after request cancellation")
		} else {
			cancel()
		}
	}
	foreign, err := fleet.FindDevice(testpkg.ContextForTenant(base, otherSchool), other.ID)
	require.NoError(t, err)
	require.Nil(t, foreign.LastSeen)
	require.Error(t, fleet.UpdateDeviceLastSeen(testpkg.Ctx(t), other.ID, time.Now()))
	foreign, err = fleet.FindDevice(testpkg.ContextForTenant(base, otherSchool), other.ID)
	require.NoError(t, err)
	require.Nil(t, foreign.LastSeen, "a heartbeat must not update another school")
}
