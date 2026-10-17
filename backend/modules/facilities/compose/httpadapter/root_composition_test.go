package httpadapter_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/facilities/compose/httpadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the owners the internal route tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	httpadapter.ComposeRooms = func(t *testing.T) (*bun.DB, httpadapter.RoomsComposition, func(chi.Router, func(chi.Router, func(http.Handler) http.Handler))) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		svc, err := api.NewRoomsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		return db, httpadapter.RoomsComposition{
			Dependencies: httpadapter.Dependencies{
				Facilities: svc.Facilities, Settings: svc.Settings, UserContext: svc.UserContext,
				Active: svc.Active, Users: api.NewRoomSnapshotPeople(svc.Users), Education: svc.Education, ListExport: svc.ListExport,
			},
			Usage: svc.RoomUsage,
		}, api.ProtectedTenantRoutes
	}
}
