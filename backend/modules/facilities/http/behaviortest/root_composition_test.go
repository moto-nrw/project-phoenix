package httpadapter_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	httpadapter "github.com/moto-ogs/project-phoenix/modules/facilities/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// composeRooms composes the root's Rooms graph over the package's test
// database.
func composeRooms(t *testing.T) (*bun.DB, RoomsComposition, func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)), func(error) bool) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, listExport, err := api.NewRoomsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	svcFacilities := api.NewActiveTestFacilities(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcPresence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcPeople := api.NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcEducation := api.NewActiveTestEducation(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	prefetch, constraintViolation := api.RoomsHTTPBindingsForTest()
	return db, RoomsComposition{
		Dependencies: httpadapter.Dependencies{
			Facilities:          svcFacilities.Facilities,
			RoomHistorySettings: api.NewRoomHistorySettings(svcSettings.Settings, slog.Default(), prefetch),
			UserContext:         svcCaller, Active: svcPresence.Active, Users: api.NewRoomSnapshotPeople(svcPeople.Users), Education: svcEducation.Education,
			ListExport: listExport,
		},
		Settings: svcSettings.Settings,
		Usage:    svcFacilities.RoomUsage,
	}, api.ProtectedTenantRoutes, constraintViolation
}
