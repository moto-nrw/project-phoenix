package platform

import "github.com/moto-nrw/project-phoenix/observability"

// prometheusDeliveryMetrics adapts the package-level Prometheus counters to
// the DeliveryMetrics port. It exists so EmailDeliveryService can be tested
// against a recording double instead of the global registry.
type prometheusDeliveryMetrics struct{}

// NewPrometheusDeliveryMetrics returns the production metrics sink.
func NewPrometheusDeliveryMetrics() DeliveryMetrics { return prometheusDeliveryMetrics{} }

func (prometheusDeliveryMetrics) RecordDeliveryEvent(provider, eventType, result string) {
	observability.RecordEmailDeliveryEvent(provider, eventType, result)
}

func (prometheusDeliveryMetrics) RecordDeliveryStatus(status string) {
	observability.RecordEmailDeliveryStatus(status)
}
