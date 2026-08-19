package observability

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func pwaGaugeValues(t *testing.T, tenant, portal string) (standalone, eligible float64) {
	t.Helper()
	return testutil.ToFloat64(pwaStandaloneUsers.WithLabelValues(tenant, portal)),
		testutil.ToFloat64(pwaEligibleUsers.WithLabelValues(tenant, portal))
}

func TestRefreshPWAGaugesResetsVanishedLabels(t *testing.T) {
	RegisterPWAUsageStatsProvider(PWAUsageStatsProviderFunc(func() ([]PWAUsageStat, error) {
		return []PWAUsageStat{
			{TenantID: 301, Portal: "staff", StandaloneUsers: 3, EligibleUsers: 12},
			{TenantID: 301, Portal: "parent", StandaloneUsers: 40, EligibleUsers: 200},
		}, nil
	}))
	refreshPWAGauges()

	standalone, eligible := pwaGaugeValues(t, "301", "staff")
	assert.Equal(t, float64(3), standalone)
	assert.Equal(t, float64(12), eligible)

	RegisterPWAUsageStatsProvider(PWAUsageStatsProviderFunc(func() ([]PWAUsageStat, error) {
		return []PWAUsageStat{{TenantID: 301, Portal: "parent", StandaloneUsers: 41, EligibleUsers: 200}}, nil
	}))
	refreshPWAGauges()

	standalone, eligible = pwaGaugeValues(t, "301", "staff")
	assert.Equal(t, float64(0), standalone, "vanished label pair must reset to zero")
	assert.Equal(t, float64(0), eligible)
	standalone, _ = pwaGaugeValues(t, "301", "parent")
	assert.Equal(t, float64(41), standalone)
}

func TestRefreshPWAGaugesKeepsValuesOnProviderError(t *testing.T) {
	RegisterPWAUsageStatsProvider(PWAUsageStatsProviderFunc(func() ([]PWAUsageStat, error) {
		return []PWAUsageStat{{TenantID: 302, Portal: "staff", StandaloneUsers: 5, EligibleUsers: 9}}, nil
	}))
	refreshPWAGauges()

	RegisterPWAUsageStatsProvider(PWAUsageStatsProviderFunc(func() ([]PWAUsageStat, error) {
		return nil, errors.New("db down")
	}))
	refreshPWAGauges()

	standalone, eligible := pwaGaugeValues(t, "302", "staff")
	assert.Equal(t, float64(5), standalone, "a failed refresh must not fake a zero")
	assert.Equal(t, float64(9), eligible)
}
