package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestWorkerLeaseEvidence(t *testing.T) {
	t.Parallel()
	until := time.Date(2026, time.October, 1, 8, 0, 30, 0, time.UTC)
	acquiredBefore := testutil.ToFloat64(workerLeadershipChanges.WithLabelValues("acquired"))
	fencedBefore := testutil.ToFloat64(workerDuplicateSuppressions.WithLabelValues("fenced"))
	renewalsBefore := testutil.CollectAndCount(workerLeaseOperations)

	SetWorkerLeaseTerm(true, 7, until)
	SetWorkerReady(true)
	RecordWorkerLeadershipChange("acquired")
	RecordWorkerDuplicateSuppression("fenced")
	RecordWorkerLeaseOperation("renew", "error", 3*time.Millisecond)

	assert.Equal(t, float64(1), testutil.ToFloat64(workerLeaseHeld))
	assert.Equal(t, float64(7), testutil.ToFloat64(workerLeaseToken))
	assert.Equal(t, float64(until.Unix()), testutil.ToFloat64(workerLeaseExpiry))
	assert.Equal(t, float64(1), testutil.ToFloat64(workerReady))
	assert.Equal(t, acquiredBefore+1, testutil.ToFloat64(workerLeadershipChanges.WithLabelValues("acquired")))
	assert.Equal(t, fencedBefore+1, testutil.ToFloat64(workerDuplicateSuppressions.WithLabelValues("fenced")))
	assert.GreaterOrEqual(t, testutil.CollectAndCount(workerLeaseOperations), renewalsBefore)

	SetWorkerLeaseTerm(false, 7, time.Time{})
	SetWorkerReady(false)

	assert.Zero(t, testutil.ToFloat64(workerLeaseHeld))
	assert.Equal(t, float64(7), testutil.ToFloat64(workerLeaseToken), "standby keeps the last token for comparison")
	assert.Zero(t, testutil.ToFloat64(workerLeaseExpiry))
	assert.Zero(t, testutil.ToFloat64(workerReady))
}
