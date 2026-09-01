package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestObserveSynchronousDeliveryClassifiesOutcomesWithStableLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		caller  string
		err     error
		outcome string
	}{
		{name: "success", caller: "delivery_metric_success", outcome: "success"},
		{name: "transport failure", caller: "delivery_metric_failure", err: errors.New("smtp unavailable"), outcome: "failure"},
		{name: "timeout", caller: "delivery_metric_timeout", err: context.DeadlineExceeded, outcome: "timeout"},
		{name: "cancellation", caller: "delivery_metric_canceled", err: context.Canceled, outcome: "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := testutil.ToFloat64(synchronousDeliveries.WithLabelValues(
				"email", "mfa_email_code", tt.caller, tt.outcome,
			))

			ObserveSynchronousDelivery("email", "mfa_email_code", tt.caller, 25*time.Millisecond, tt.err)

			after := testutil.ToFloat64(synchronousDeliveries.WithLabelValues(
				"email", "mfa_email_code", tt.caller, tt.outcome,
			))
			assert.Equal(t, before+1, after)
		})
	}
}

func TestObserveSynchronousDeliverySanitizesEmptyLabels(t *testing.T) {
	t.Parallel()

	before := testutil.ToFloat64(synchronousDeliveries.WithLabelValues(
		"unknown", "unknown", "unknown", "success",
	))

	ObserveSynchronousDelivery(" ", "", "\t", time.Millisecond, nil)

	after := testutil.ToFloat64(synchronousDeliveries.WithLabelValues(
		"unknown", "unknown", "unknown", "success",
	))
	assert.Equal(t, before+1, after)
}

func TestObserveDurableDeliveryRecordsCountsAndOldestPendingAge(t *testing.T) {
	t.Parallel()
	before := testutil.ToFloat64(durableDeliveryOperations.WithLabelValues(
		"email", "welcome", "claim", "success",
	))

	ObserveDurableDelivery("email", "welcome", "claim", 10*time.Millisecond, 3, nil)
	ObserveDurableDelivery("", "", "oldest_pending_age", 90*time.Second, 1, nil)

	after := testutil.ToFloat64(durableDeliveryOperations.WithLabelValues(
		"email", "welcome", "claim", "success",
	))
	assert.Equal(t, before+3, after)
	assert.Equal(t, float64(90), testutil.ToFloat64(durableDeliveryOldestPendingAge))
}
