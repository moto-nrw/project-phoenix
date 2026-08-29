package api

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordHTTPRuntimeEventCountsMissingTenant(t *testing.T) {
	t.Parallel()
	before := httpMissingTenantRuntimeEvents(t)
	tracer := newRuntimeTracer(slog.New(slog.DiscardHandler))

	recordHTTPRuntimeEvent(context.Background(), tracer, tenant.RuntimeEvent{Kind: tenant.RuntimeMissingTenant, Err: tenant.ErrInvalidTenantID})

	assert.GreaterOrEqual(t, httpMissingTenantRuntimeEvents(t), before+1)
}

func TestRuntimeFailureMetricCardinalityIgnoresCorrelationAndErrorText(t *testing.T) {
	t.Parallel()
	tracer := newRuntimeTracer(slog.New(slog.DiscardHandler))

	for _, entryPoint := range []string{"http", "worker"} {
		ctx, _, err := tracer.StartRequest(context.Background(), entryPoint+"-correlation-one")
		require.NoError(t, err)
		tracer.Failure(ctx, entryPoint, "operation-one", "transaction_failure", errors.New("student one"))
		seriesAfterFirst := runtimeFailureSeriesCount(t, entryPoint, "transaction_failure")

		ctx, _, err = tracer.StartRequest(context.Background(), entryPoint+"-correlation-two")
		require.NoError(t, err)
		tracer.Failure(ctx, entryPoint, "operation-two", "transaction_failure", errors.New("student two"))

		assert.Equal(t, seriesAfterFirst, runtimeFailureSeriesCount(t, entryPoint, "transaction_failure"),
			"correlation, operation, and error text must not create metric series")
	}
}

func runtimeFailureSeriesCount(t *testing.T, entryPoint, outcome string) int {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	count := 0
	for _, family := range families {
		if family.GetName() != "phoenix_tenant_runtime_events_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["entry_point"] == entryPoint && labels["outcome"] == outcome {
				count++
			}
		}
	}
	return count
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
