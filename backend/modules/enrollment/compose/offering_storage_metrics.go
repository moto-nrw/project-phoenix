package compose

import (
	"errors"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/prometheus/client_golang/prometheus"
)

var offeringStorageMetricsOnce sync.Once

var offeringStorageOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "phoenix_enrollment_offering_storage_operations_total",
	Help: "Enrollment offering-storage owner calls, including failed calls; success does not imply the enclosing workflow committed.",
}, []string{"operation", "kind", "outcome", "code"})

var offeringStorageDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "phoenix_enrollment_offering_storage_duration_seconds",
	Help:    "Enrollment offering-storage owner query and command latency.",
	Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"operation", "kind"})

var offeringStorageRows = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "phoenix_enrollment_offering_storage_rows",
	Help:    "Input items and returned or reported affected rows per Enrollment offering-storage owner call. Input items are not persisted-write counts.",
	Buckets: []float64{0, 1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
}, []string{"operation", "direction"})

func observeOfferingStorage() func(enrollment.OfferingStorageObservation) {
	offeringStorageMetricsOnce.Do(func() {
		prometheus.MustRegister(offeringStorageOperations, offeringStorageDuration, offeringStorageRows)
	})
	return func(event enrollment.OfferingStorageObservation) {
		outcome, code := "success", "none"
		if event.Err != nil {
			outcome, code = "error", "failed"
			if errors.Is(event.Err, enrollment.ErrSubmittedOfferingChoiceConflict) {
				code = "immutable_conflict"
			}
			var databaseError interface{ Field(byte) string }
			if errors.As(event.Err, &databaseError) {
				switch databaseError.Field('C') {
				case "40P01":
					code = "deadlock"
				case "40001":
					code = "serialization_failure"
				case "55P03":
					code = "lock_timeout"
				}
			}
		}
		offeringStorageOperations.WithLabelValues(event.Operation, event.Kind, outcome, code).Inc()
		offeringStorageDuration.WithLabelValues(event.Operation, event.Kind).Observe(event.Duration.Seconds())
		offeringStorageRows.WithLabelValues(event.Operation, "input").Observe(float64(event.InputRows))
		if event.OutputRows >= 0 {
			offeringStorageRows.WithLabelValues(event.Operation, "output").Observe(float64(event.OutputRows))
		}
	}
}
