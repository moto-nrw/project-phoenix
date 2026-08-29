package api

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordHTTPRuntimeEventCountsMissingTenant(t *testing.T) {
	t.Parallel()
	before := httpMissingTenantRuntimeEvents(t)

	recordHTTPRuntimeEvent(tenant.RuntimeEvent{Outcome: tenant.RuntimeMissingTenant, Err: tenant.ErrInvalidTenantID})

	assert.GreaterOrEqual(t, httpMissingTenantRuntimeEvents(t), before+1)
}

func httpMissingTenantRuntimeEvents(t *testing.T) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "phoenix_tenant_runtime_events_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["entry_point"] == "http" && labels["outcome"] == "missing_tenant" {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}
