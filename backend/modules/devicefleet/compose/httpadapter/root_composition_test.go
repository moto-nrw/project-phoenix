package httpadapter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/httpadapter"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The root imports this package, so the internal display suites reach its
// compositions through these bindings.
func init() {
	root := &httpadapter.RootCompositions
	root.Settings = func(t *testing.T, database testpkg.Database) httpadapter.DisplaySettings {
		t.Helper()
		service := api.NewSettingsTestService(testpkg.SettingsRuntime(t, database.Pool()))
		testpkg.SetTenantRuntime(t, service, database.Pool())
		return httpadapter.DisplaySettings{
			FeatureEnabled: api.DisplayFeatureEnabled(service),
			TenantFacts: func(schools devicefleetLegacy.SchoolDirectory) devicefleetCompose.TenantFacts {
				return devicefleetLegacy.NewTenantFacts(devicefleetLegacy.TenantFactDependencies{Schools: schools, Settings: service})
			},
			Enable: func(ctx context.Context) error {
				return service.SetValue(ctx, settings.KeyDisplayEnabled, true, nil, nil)
			},
			Reset: func(ctx context.Context) error {
				return service.ResetValue(ctx, settings.KeyDisplayEnabled, nil, nil)
			},
		}
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
			Fleet: devicefleetCompose.Dependencies{Rooms: rooms},
			Dashboard: devicefleetLegacy.DashboardDependencies{
				ActiveGroups: repos.ActiveGroup, Templates: repos.ActivityGroup, Instances: repos.ActivityInstance,
			},
			CarePlan: carePlan, ApprovedOfferings: approvedOfferings,
		}
	}
}
