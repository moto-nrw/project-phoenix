package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validEvent() *EmailDeliveryEvent {
	return &EmailDeliveryEvent{
		OutboxID:        42,
		Provider:        "generic",
		ProviderEventID: "evt_1",
		EventType:       DeliveryEventDelivered,
		OccurredAt:      time.Now(),
	}
}

func TestEmailDeliveryEvent_ValidateFillsDefaults(t *testing.T) {
	event := validEvent()
	require.NoError(t, event.Validate())

	assert.False(t, event.ReceivedAt.IsZero(), "received_at defaults to now")
	assert.NotNil(t, event.Raw, "raw must never be nil — the column is NOT NULL")
}

// The CHECK constraints exist in the DB; validating in app code fails fast
// before the round-trip and keeps the error message useful.
func TestEmailDeliveryEvent_ValidateRejectsBadInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*EmailDeliveryEvent)
	}{
		{"missing provider", func(e *EmailDeliveryEvent) { e.Provider = "  " }},
		{"missing provider event id", func(e *EmailDeliveryEvent) { e.ProviderEventID = "" }},
		{"missing outbox id", func(e *EmailDeliveryEvent) { e.OutboxID = 0 }},
		{"unknown event type", func(e *EmailDeliveryEvent) { e.EventType = "teleported" }},
		{"missing occurred_at", func(e *EmailDeliveryEvent) { e.OccurredAt = time.Time{} }},
		{"bad bounce class", func(e *EmailDeliveryEvent) {
			bad := "sideways"
			e.BounceClass = &bad
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := validEvent()
			tt.mutate(event)
			require.Error(t, event.Validate())
		})
	}
}

func TestEmailDeliveryEvent_ValidateAcceptsKnownBounceClasses(t *testing.T) {
	for _, class := range []string{BounceClassHard, BounceClassSoft} {
		event := validEvent()
		event.EventType = DeliveryEventBounced
		c := class
		event.BounceClass = &c
		require.NoError(t, event.Validate(), "bounce class %q must be accepted", class)
	}
}

func TestEmailOutbox_ValidateDeliveryStatus(t *testing.T) {
	// Empty defaults to "unknown": a freshly queued row has no provider
	// feedback, which is NOT the same as "delivered".
	row := &EmailOutbox{Kind: "test"}
	require.NoError(t, row.Validate())
	assert.Equal(t, EmailDeliveryStatusUnknown, row.DeliveryStatus)

	for _, status := range []string{
		EmailDeliveryStatusUnknown, EmailDeliveryStatusQueued, EmailDeliveryStatusDeferred,
		EmailDeliveryStatusDelivered, EmailDeliveryStatusSoftBounced, EmailDeliveryStatusSuppressed,
		EmailDeliveryStatusComplained, EmailDeliveryStatusBounced,
	} {
		row := &EmailOutbox{Kind: "test", DeliveryStatus: status}
		require.NoError(t, row.Validate(), "status %q must be accepted", status)
	}

	row = &EmailOutbox{Kind: "test", DeliveryStatus: "half-delivered"}
	require.Error(t, row.Validate())
}
