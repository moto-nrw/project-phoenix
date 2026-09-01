package observability

import (
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticSSEStatsProvider struct {
	stats SSEStats
}

func (p staticSSEStatsProvider) SnapshotStats() SSEStats {
	return p.stats
}

// Deliberately NOT parallel: RegisterSSEStatsProvider installs a
// process-global provider that MetricsHandler reads on every scrape, so two
// of these tests overwrite each other's provider.
func TestRefreshSSEGaugesResetsDisconnectedTenants(t *testing.T) {
	RegisterSSEStatsProvider(staticSSEStatsProvider{
		stats: SSEStats{ClientsByTenant: map[int64]int{101: 2, 202: 1}},
	})
	refreshSSEGauges()

	RegisterSSEStatsProvider(staticSSEStatsProvider{
		stats: SSEStats{ClientsByTenant: map[int64]int{202: 3}},
	})
	refreshSSEGauges()

	assert.Equal(t, float64(0), gaugeValue(t, "101"))
	assert.Equal(t, float64(3), gaugeValue(t, "202"))
}

func TestMetricsBearerTokenFromEnvRequiresConfiguredToken(t *testing.T) {
	t.Parallel()

	token, err := MetricsBearerTokenFromEnv(func(string) string {
		return "  "
	})

	require.Error(t, err)
	assert.Empty(t, token)
}

func TestMetricsBearerTokenFromEnvReturnsTrimmedToken(t *testing.T) {
	t.Parallel()

	token, err := MetricsBearerTokenFromEnv(func(string) string {
		return " correct-token "
	})

	require.NoError(t, err)
	assert.Equal(t, "correct-token", token)
}

func TestPrometheusRecordersUseStableLabels(t *testing.T) {
	t.Parallel()

	ObserveTenantRequest(404, "", "GET", "/api/students/{id}", 201, time.Millisecond, "commit")
	ObserveIoTRequest(0, "POST", "/api/iot/scan", 401, time.Millisecond, "")
	RecordSSEConnection(404, "connected")
	RecordSSEBroadcast(0, "student_location_changed", "all", 2)

	assert.Equal(t, "unknown", StatusClass(0))
	assert.Equal(t, "2xx", StatusClass(204))
	assert.Equal(t, "unknown", sanitizeLabel(" "))
	assert.Equal(t, "auth_error", outcomeForStatus(403))
	assert.Equal(t, "validation_error", outcomeForStatus(400))
	assert.Equal(t, "server_error", outcomeForStatus(500))
	assert.Equal(t, "success", outcomeForStatus(200))
	assert.Equal(t, "GET", normalizeHTTPMethod("get"))
	assert.Equal(t, "other", normalizeHTTPMethod("STUDENT-ERIKA"))
}

func TestRecordTenantRuntimeEventCountsByEntryPointAndOutcome(t *testing.T) {
	t.Parallel()
	before := testutil.ToFloat64(tenantRuntimeEvents.WithLabelValues("worker", "missing_tenant"))

	RecordTenantRuntimeEvent("worker", "missing_tenant")

	after := testutil.ToFloat64(tenantRuntimeEvents.WithLabelValues("worker", "missing_tenant"))
	assert.Equal(t, before+1, after)
}

func TestRecordAuthorizationEventUsesStableLabels(t *testing.T) {
	t.Parallel()
	denialsBefore := testutil.ToFloat64(authorizationDenials.WithLabelValues("permission_denied"))
	unknownBefore := testutil.ToFloat64(authorizationDenials.WithLabelValues("unknown"))
	durationsBefore := testutil.CollectAndCount(authMiddlewareDuration)

	RecordAuthorizationEvent("resolved", "permission_denied", time.Millisecond)
	RecordAuthorizationEvent("", "student-Erika", 0)

	assert.Equal(t, denialsBefore+1, testutil.ToFloat64(authorizationDenials.WithLabelValues("permission_denied")))
	assert.Equal(t, unknownBefore+1, testutil.ToFloat64(authorizationDenials.WithLabelValues("unknown")))
	assert.Equal(t, durationsBefore+1, testutil.CollectAndCount(authMiddlewareDuration))
}

func TestHTTPMethodLabelsCollapseUnknownValues(t *testing.T) {
	t.Parallel()

	const route = "/cardinality/method"
	before := testutil.ToFloat64(appHTTPRequests.WithLabelValues("other", route, "5xx"))

	ObserveHTTPRequest("student-Erika", route, 500, time.Millisecond)
	ObserveHTTPRequest("another-untrusted-method", route, 500, time.Millisecond)

	after := testutil.ToFloat64(appHTTPRequests.WithLabelValues("other", route, "5xx"))
	assert.Equal(t, before+2, after)
}

func TestRecordUnitOfWorkEvidence(t *testing.T) {
	t.Parallel()
	const entryPoint = "unit_test_evidence"
	rollbackBefore := testutil.ToFloat64(unitOfWorkRollbacks.WithLabelValues(entryPoint))
	retryBefore := testutil.ToFloat64(unitOfWorkRetries.WithLabelValues(entryPoint))
	durationBefore := testutil.CollectAndCount(unitOfWorkDuration)
	poolBefore := testutil.CollectAndCount(unitOfWorkPoolWait)
	lockBefore := testutil.CollectAndCount(unitOfWorkLockWait)

	RecordUnitOfWorkEvent(entryPoint, "transaction", "rollback", 25*time.Millisecond, 2)
	RecordUnitOfWorkEvent(entryPoint, "pool_wait", "", 3*time.Millisecond, 0)
	RecordUnitOfWorkEvent(entryPoint, "lock_wait", "", 4*time.Millisecond, 0)

	assert.Equal(t, rollbackBefore+1, testutil.ToFloat64(unitOfWorkRollbacks.WithLabelValues(entryPoint)))
	assert.Equal(t, retryBefore+2, testutil.ToFloat64(unitOfWorkRetries.WithLabelValues(entryPoint)))
	assert.Equal(t, durationBefore+1, testutil.CollectAndCount(unitOfWorkDuration))
	assert.Equal(t, poolBefore+1, testutil.CollectAndCount(unitOfWorkPoolWait))
	assert.Equal(t, lockBefore+1, testutil.CollectAndCount(unitOfWorkLockWait))
}

func TestFeedbackHTTPResponseUsesActualStatusClassAndStableCode(t *testing.T) {
	t.Parallel()
	badRequestBefore := testutil.ToFloat64(feedbackHTTPResponses.WithLabelValues("iot", "4xx", "invalid_parameters"))
	serverErrorBefore := testutil.ToFloat64(feedbackHTTPResponses.WithLabelValues("staff", "5xx", "internal_error"))

	ObserveFeedbackHTTPResponse("iot", 400, "invalid_parameters")
	ObserveFeedbackHTTPResponse("staff", 500, "internal_error")

	assert.Equal(t, badRequestBefore+1, testutil.ToFloat64(feedbackHTTPResponses.WithLabelValues("iot", "4xx", "invalid_parameters")))
	assert.Equal(t, serverErrorBefore+1, testutil.ToFloat64(feedbackHTTPResponses.WithLabelValues("staff", "5xx", "internal_error")))
}

func TestRecordWorkerRunEvidence(t *testing.T) {
	t.Parallel()
	const jobID = "test-worker-job"
	before := testutil.CollectAndCount(workerJobDuration)

	RecordWorkerRunEvent(jobID, "success", 25*time.Millisecond)

	assert.Equal(t, before+1, testutil.CollectAndCount(workerJobDuration))
}

func TestRecordSettingsEvidence(t *testing.T) {
	t.Parallel()
	const key = "test.settings_evidence"
	lookupBefore := testutil.ToFloat64(settingsLookups.WithLabelValues(key, "hit", "ok"))
	failureBefore := testutil.ToFloat64(settingsSideEffectFailures.WithLabelValues(key))
	durationBefore := testutil.CollectAndCount(settingsLookupDuration)

	ObserveSettingsLookup(key, "hit", "ok", 2*time.Millisecond)
	RecordSettingsSideEffectFailure(key)

	assert.Equal(t, lookupBefore+1, testutil.ToFloat64(settingsLookups.WithLabelValues(key, "hit", "ok")))
	assert.Equal(t, failureBefore+1, testutil.ToFloat64(settingsSideEffectFailures.WithLabelValues(key)))
	assert.Equal(t, durationBefore+1, testutil.CollectAndCount(settingsLookupDuration))
}

func TestObserveMealPlanOperationRecordsStatementDuration(t *testing.T) {
	t.Parallel()

	before := testutil.CollectAndCount(mealPlanStatementDuration)

	ObserveMealPlanOperation("replace_day", time.Millisecond, 2, 1, 3*time.Millisecond, nil)

	assert.Equal(t, before+1, testutil.CollectAndCount(mealPlanStatementDuration))
}

func TestObserveAuditAppendRecordsRuntimeEvidence(t *testing.T) {
	t.Parallel()

	const eventType = "*audit.AuthEvent"
	successBefore := testutil.ToFloat64(auditAppends.WithLabelValues(eventType, "success"))
	errorBefore := testutil.ToFloat64(auditAppends.WithLabelValues(eventType, "error"))
	rowsBefore := testutil.ToFloat64(auditRows.WithLabelValues(eventType))
	durationBefore := testutil.CollectAndCount(auditAppendDuration)

	ObserveAuditAppend(eventType, 2*time.Millisecond, 1, nil)
	ObserveAuditAppend(eventType, time.Millisecond, 0, assert.AnError)

	assert.Equal(t, successBefore+1, testutil.ToFloat64(auditAppends.WithLabelValues(eventType, "success")))
	assert.Equal(t, errorBefore+1, testutil.ToFloat64(auditAppends.WithLabelValues(eventType, "error")))
	assert.Equal(t, rowsBefore+1, testutil.ToFloat64(auditRows.WithLabelValues(eventType)))
	assert.Equal(t, durationBefore+1, testutil.CollectAndCount(auditAppendDuration))
}

func TestDBStatsCollectorEmitsProviderMetrics(t *testing.T) {
	t.Parallel()

	RegisterDBStatsProvider(func() DBStats {
		return DBStats{
			OpenConnections:   8,
			InUse:             3,
			Idle:              5,
			WaitCount:         13,
			WaitDuration:      2 * time.Second,
			MaxIdleClosed:     21,
			MaxLifetimeClosed: 34,
		}
	})
	ch := make(chan prometheus.Metric, 10)

	dbStatsCollector{}.Collect(ch)
	close(ch)

	var metricCount int
	for range ch {
		metricCount++
	}
	assert.Equal(t, 7, metricCount)
}

func TestDBStatsCollectorDescribesEveryEmittedMetric(t *testing.T) {
	t.Parallel()

	ch := make(chan *prometheus.Desc, 10)

	dbStatsCollector{}.Describe(ch)
	close(ch)

	var descCount int
	for range ch {
		descCount++
	}
	assert.Equal(t, 7, descCount)
}

func TestMain(m *testing.M) {
	code := m.Run()
	RegisterDBStatsProvider(nil)
	RegisterSSEStatsProvider(nil)
	os.Exit(code)
}

func gaugeValue(t *testing.T, tenant string) float64 {
	t.Helper()
	return testutil.ToFloat64(sseClients.WithLabelValues(tenant))
}
