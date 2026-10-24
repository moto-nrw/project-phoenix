package devices_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/http/devices"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the owners the internal route tests run over. It
// imports this package, so the binding lives in the external test package.
func init() {
	devices.ComposeDeviceFleet = func(t *testing.T) (*bun.DB, devicefleet.Capability) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		module, err := api.NewDeviceTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		return db, module.IoT.Fleet()
	}
}
