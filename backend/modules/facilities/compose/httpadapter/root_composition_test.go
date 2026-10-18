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
		svc, listExport, err := api.NewRoomsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		prefetch, constraintViolation := api.RoomsHTTPBindingsForTest()
		return db, httpadapter.RoomsComposition{
			Dependencies: httpadapter.Dependencies{
				Facilities: svc.Facilities, RoomHistorySettings: api.NewRoomHistorySettings(svc.Settings, slog.Default(), prefetch),
				UserContext: svc.UserContext, Active: svc.Active, Users: api.NewRoomSnapshotPeople(svc.Users), Education: svc.Education,
				ListExport: listExport,
			},
			Settings: svc.Settings,
			Usage:    svc.RoomUsage,
		}, api.ProtectedTenantRoutes, constraintViolation
	}
}
