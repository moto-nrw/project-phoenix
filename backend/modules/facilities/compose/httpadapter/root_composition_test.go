package httpadapter_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/facilities/compose/httpadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root composes the owners the internal route tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	httpadapter.ComposeRooms = func(t *testing.T) (*bun.DB, httpadapter.RoomsComposition, func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)), func(error) bool) {
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
		return db, httpadapter.RoomsComposition{
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
}
