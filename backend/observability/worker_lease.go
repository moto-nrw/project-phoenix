package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Runtime evidence of the Worker lease (#2726). Every Worker process reports
// its own view; the owner is the process whose held gauge is 1. Labels carry
// only fixed operations, outcomes, changes and reasons.
var (
	workerLeaseHeld = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoenix_worker_lease_held",
		Help: "1 while this process holds the Worker lease, 0 in standby.",
	})
	workerLeaseToken = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoenix_worker_lease_fencing_token",
		Help: "Fencing token of the last term this process held.",
	})
	workerLeaseExpiry = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoenix_worker_lease_expiry_timestamp_seconds",
		Help: "Database-time expiry of the held term; 0 in standby.",
	})
	workerLeaseOperations = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "phoenix_worker_lease_operation_seconds",
		Help:    "Worker lease acquire, renew and release latency by outcome.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"operation", "outcome"})
	workerLeadershipChanges = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "phoenix_worker_leadership_changes_total",
		Help: "Worker leadership changes of this process: acquired, lost, expired, fenced, released.",
	}, []string{"change"})
	workerStandbyDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "phoenix_worker_standby_duration_seconds",
		Help:    "Time this process waited in standby before it led.",
		Buckets: []float64{0.01, 0.1, 0.5, 1, 5, 10, 30, 60, 300, 1800, 3600},
	})
	workerDuplicateSuppressions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "phoenix_worker_duplicate_suppressions_total",
		Help: "Job runs skipped in standby and job commits fenced after the lease passed on.",
	}, []string{"reason"})
	workerReady = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "phoenix_worker_ready",
		Help: "1 while this Worker holds the lease and runs jobs.",
	})
	workerDrainDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "phoenix_worker_drain_seconds",
		Help:    "Time a graceful Worker stop waited for running jobs before releasing the lease.",
		Buckets: []float64{0.01, 0.1, 0.5, 1, 2.5, 5, 10, 30},
	})
)

func init() {
	prometheus.MustRegister(
		workerLeaseHeld,
		workerLeaseToken,
		workerLeaseExpiry,
		workerLeaseOperations,
		workerLeadershipChanges,
		workerStandbyDuration,
		workerDuplicateSuppressions,
		workerReady,
		workerDrainDuration,
	)
}

// SetWorkerLeaseTerm records the term this process holds, or standby.
func SetWorkerLeaseTerm(held bool, token int64, until time.Time) {
	workerLeaseHeld.Set(boolGauge(held))
	workerLeaseToken.Set(float64(token))
	if held {
		workerLeaseExpiry.Set(float64(until.UnixMilli()) / 1000)
		return
	}
	workerLeaseExpiry.Set(0)
}

// RecordWorkerLeaseOperation records one acquire, renew or release.
func RecordWorkerLeaseOperation(operation, outcome string, latency time.Duration) {
	workerLeaseOperations.WithLabelValues(sanitizeLabel(operation), sanitizeLabel(outcome)).Observe(latency.Seconds())
}

// RecordWorkerLeadershipChange counts one leadership change of this process.
func RecordWorkerLeadershipChange(change string) {
	workerLeadershipChanges.WithLabelValues(sanitizeLabel(change)).Inc()
}

// ObserveWorkerStandby records how long this process waited before it led.
func ObserveWorkerStandby(duration time.Duration) {
	workerStandbyDuration.Observe(duration.Seconds())
}

// RecordWorkerDuplicateSuppression counts a job run or commit the lease
// prevented.
func RecordWorkerDuplicateSuppression(reason string) {
	workerDuplicateSuppressions.WithLabelValues(sanitizeLabel(reason)).Inc()
}

// SetWorkerReady records the Worker's readiness.
func SetWorkerReady(ready bool) {
	workerReady.Set(boolGauge(ready))
}

// ObserveWorkerDrain records the drain time of a graceful Worker stop.
func ObserveWorkerDrain(duration time.Duration) {
	workerDrainDuration.Observe(duration.Seconds())
}

func boolGauge(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
