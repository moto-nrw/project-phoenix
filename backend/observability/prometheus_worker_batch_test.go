package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordWorkerRunTracksMaximumDuration(t *testing.T) {
	t.Parallel()
	const jobID = "test-worker-job-max-duration"

	RecordWorkerRunEvent(jobID, "success", 25*time.Millisecond)
	maxAfterFirstRun := testutil.ToFloat64(workerJobMaxDuration.WithLabelValues(jobID))

	RecordWorkerRunEvent(jobID, "success", time.Millisecond)

	assert.InDelta(t, 0.025, maxAfterFirstRun, 0.000001)
	assert.Equal(t, maxAfterFirstRun, testutil.ToFloat64(workerJobMaxDuration.WithLabelValues(jobID)))
}

func TestRecordWorkerTenantBatchEvidence(t *testing.T) {
	t.Parallel()
	const jobID = "test-worker-tenant-batch"
	processedBefore := testutil.ToFloat64(workerTenantBatchTenants.WithLabelValues(jobID, "success"))
	failedBefore := testutil.ToFloat64(workerTenantBatchTenants.WithLabelValues(jobID, "failure"))
	retriesBefore := testutil.ToFloat64(workerTenantBatchRetries.WithLabelValues(jobID))

	RecordWorkerTenantBatchEvent(jobID, 25*time.Millisecond, 5, 2, 3, 4, 2*time.Millisecond)

	assert.Equal(t, processedBefore+3, testutil.ToFloat64(workerTenantBatchTenants.WithLabelValues(jobID, "success")))
	assert.Equal(t, failedBefore+2, testutil.ToFloat64(workerTenantBatchTenants.WithLabelValues(jobID, "failure")))
	assert.Equal(t, retriesBefore+3, testutil.ToFloat64(workerTenantBatchRetries.WithLabelValues(jobID)))
	assert.Equal(t, float64(4), testutil.ToFloat64(workerTenantBatchBacklog.WithLabelValues(jobID)))
}
