package owners

import (
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type workerAnalyticsProbe struct{}

func (workerAnalyticsProbe) Capture(string, string, map[string]any) {}

func TestWorkerAnalyticsNormalizesDemoEnvironment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	organizations, err := NewOrganizationTenancy(database)
	require.NoError(t, err)

	for _, tc := range []struct {
		appEnv string
		demo   bool
	}{
		{"development", false},
		{"production", false},
		{"demo", true},
		{" DeMo \t", true},
	} {
		t.Run(tc.appEnv, func(t *testing.T) {
			bound := false
			bind := func(tracker ProductEventTracker, reader settings.TenantReader, demo bool) ProductEventTracker {
				bound = true
				require.NotNil(t, reader)
				require.Equal(t, tc.demo, demo, "worker and serving roots must normalize the demo environment alike")
				return tracker
			}
			env := RootEnvironment{
				Logger: slog.Default(), Config: ServiceConfig{AppEnv: tc.appEnv},
				Observe: RootObservers{SettingsLookup: func(string, string, string, time.Duration) {}},
			}
			values, tracker, err := buildWorkerAnalyticsSettings(database, testpkg.TenantRuntime(t, db), env,
				organizations, nil, NewEnrollmentBookings(), workerAnalyticsProbe{}, bind)
			require.NoError(t, err)
			require.NotNil(t, values.Service)
			require.NotNil(t, tracker)
			require.True(t, bound)
		})
	}
}
