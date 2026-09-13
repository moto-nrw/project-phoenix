package compose

import (
	"errors"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/prometheus/client_golang/prometheus"
)

var offeringBookingMetricsOnce sync.Once

var offeringBookingOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "phoenix_care_offering_booking_operations_total",
	Help: "Effective-booking owner calls, including failed calls; success does not imply the enclosing workflow committed.",
}, []string{"operation", "kind", "outcome", "code"})

var offeringBookingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "phoenix_care_offering_booking_duration_seconds",
	Help:    "Effective-booking owner query and command latency.",
	Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"operation", "kind"})

var offeringBookingRows = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "phoenix_care_offering_booking_rows",
	Help:    "Input items and returned or reported affected rows per effective-booking owner call. Input items are not persisted-write counts.",
	Buckets: []float64{0, 1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
}, []string{"operation", "direction"})

func observeOfferingBookings() func(careplan.OfferingBookingObservation) {
	offeringBookingMetricsOnce.Do(func() {
		prometheus.MustRegister(offeringBookingOperations, offeringBookingDuration, offeringBookingRows)
	})
	return func(event careplan.OfferingBookingObservation) {
		outcome, code := "success", "none"
		if event.Err != nil {
			outcome, code = "error", careplan.ErrorCode(event.Err)
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
		offeringBookingOperations.WithLabelValues(event.Operation, event.Kind, outcome, code).Inc()
		offeringBookingDuration.WithLabelValues(event.Operation, event.Kind).Observe(event.Duration.Seconds())
		offeringBookingRows.WithLabelValues(event.Operation, "input").Observe(float64(event.InputRows))
		if event.OutputRows >= 0 {
			offeringBookingRows.WithLabelValues(event.Operation, "output").Observe(float64(event.OutputRows))
		}
	}
}
