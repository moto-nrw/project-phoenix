package httpadapter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/httpadapter"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The root imports this package, so the internal display suites reach its
// compositions through these bindings.
func init() {
	root := &httpadapter.RootCompositions
	root.SettingsRows = func(t *testing.T, database testpkg.Database) httpadapter.SettingsRows {
		t.Helper()
		repos := api.NewSettingsTestRepositories(testpkg.SettingsRuntime(t, database.Pool()))
		return httpadapter.SettingsRows{Values: repos.Values, Audit: repos.Audit}
	}
	root.Display = func(t *testing.T, database testpkg.Database) httpadapter.DisplayComposition {
		t.Helper()
		repos, err := api.NewDisplayTestRepositories(database, testpkg.SettingsRuntime(t, database.Pool()))
		require.NoError(t, err)
		approvedOfferings, err := api.NewApprovedOfferingTestProjection(database, repos.Enrollment())
		require.NoError(t, err)
		rooms, err := api.NewFacilities(database)
		require.NoError(t, err)
		carePlan, err := api.NewCarePlan(database, api.MustNewPeopleDirectory(database), repos.InstanceStudent)
		require.NoError(t, err)
		return httpadapter.DisplayComposition{
			Settings: httpadapter.SettingsRows{Values: repos.Values, Audit: repos.Audit},
			Fleet:    devicefleetCompose.Dependencies{Rooms: rooms},
			Dashboard: devicefleetLegacy.DashboardDependencies{
				ActiveGroups: repos.ActiveGroup, Templates: repos.ActivityGroup, Instances: repos.ActivityInstance,
			},
			CarePlan: carePlan, ApprovedOfferings: approvedOfferings,
		}
	}
}
