package devices_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// composeDeviceFleet composes the Device Fleet owner over the package's test
// database, as the root binds it.
func composeDeviceFleet(t *testing.T) (*bun.DB, devicefleet.Capability) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewDeviceTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module.IoT.Fleet()
}
