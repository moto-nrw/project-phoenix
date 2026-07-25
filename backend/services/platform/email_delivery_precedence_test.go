package platform

import (
	"fmt"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
)

// The lattice is the whole out-of-order defence. If two statuses ever share a
// rank, a late event could overwrite a newer one purely on timestamp.
func TestDeliveryRank_IsStrictlyOrdered(t *testing.T) {
	ordered := []string{
		platformModels.EmailDeliveryStatusUnknown,
		platformModels.EmailDeliveryStatusQueued,
		platformModels.EmailDeliveryStatusDeferred,
		platformModels.EmailDeliveryStatusDelivered,
		platformModels.EmailDeliveryStatusSoftBounced,
		platformModels.EmailDeliveryStatusSuppressed,
		platformModels.EmailDeliveryStatusComplained,
		platformModels.EmailDeliveryStatusBounced,
	}

	for i := 1; i < len(ordered); i++ {
		assert.Greater(t, DeliveryRank(ordered[i]), DeliveryRank(ordered[i-1]),
			"%s must outrank %s", ordered[i], ordered[i-1])
	}
}

// An unrecognised status must rank lowest so it can never displace a real
// outcome that we did understand.
func TestDeliveryRank_UnknownValueRanksLowest(t *testing.T) {
	assert.Equal(t, 0, DeliveryRank("something-a-future-provider-invented"))
}

func TestStatusForEvent(t *testing.T) {
	hard := platformModels.BounceClassHard
	soft := platformModels.BounceClassSoft

	tests := []struct {
		name        string
		eventType   string
		bounceClass *string
		want        string
	}{
		{"queued", platformModels.DeliveryEventQueued, nil, platformModels.EmailDeliveryStatusQueued},
		{"deferred", platformModels.DeliveryEventDeferred, nil, platformModels.EmailDeliveryStatusDeferred},
		{"delivered", platformModels.DeliveryEventDelivered, nil, platformModels.EmailDeliveryStatusDelivered},
		{"complained", platformModels.DeliveryEventComplained, nil, platformModels.EmailDeliveryStatusComplained},
		{"suppressed", platformModels.DeliveryEventSuppressed, nil, platformModels.EmailDeliveryStatusSuppressed},
		{"hard bounce", platformModels.DeliveryEventBounced, &hard, platformModels.EmailDeliveryStatusBounced},
		{"soft bounce", platformModels.DeliveryEventBounced, &soft, platformModels.EmailDeliveryStatusSoftBounced},
		// Conservative default: an unclassified bounce is treated as hard, so
		// staff are told to fix the address rather than waiting forever.
		{"bounce without class", platformModels.DeliveryEventBounced, nil, platformModels.EmailDeliveryStatusBounced},
		{"unknown type", "teleported", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForEvent(tt.eventType, tt.bounceClass))
		})
	}
}

func TestIsTerminalFailure(t *testing.T) {
	terminal := []string{
		platformModels.EmailDeliveryStatusBounced,
		platformModels.EmailDeliveryStatusComplained,
		platformModels.EmailDeliveryStatusSuppressed,
	}
	for _, status := range terminal {
		assert.True(t, IsTerminalFailure(status), "%s should notify ops", status)
	}

	// A delay is explicitly NOT terminal: the mail server is still retrying
	// and alerting on it would train the office to ignore the alerts.
	notTerminal := []string{
		platformModels.EmailDeliveryStatusUnknown,
		platformModels.EmailDeliveryStatusQueued,
		platformModels.EmailDeliveryStatusDeferred,
		platformModels.EmailDeliveryStatusDelivered,
		platformModels.EmailDeliveryStatusSoftBounced,
	}
	for _, status := range notTerminal {
		assert.False(t, IsTerminalFailure(status), "%s should not notify ops", status)
	}
}

// parseOutboxRef is pure string parsing over a Message-ID we minted ourselves,
// so the ids here are inputs to the parser rather than database identities —
// they are round-tripped through fmt.Sprintf instead of being asserted as
// literals.
func TestParseOutboxRef(t *testing.T) {
	wantTenant := int64(time.Now().UnixNano() % 100000)
	wantOutbox := wantTenant + 1
	messageID := fmt.Sprintf("ob.%d.%d.9f3c1e2a@mail.example.de", wantTenant, wantOutbox)

	tenantID, outboxID, ok := parseOutboxRef(messageID)
	assert.True(t, ok)
	assert.Equal(t, wantTenant, tenantID)
	assert.Equal(t, wantOutbox, outboxID)

	for _, bad := range []string{
		"",
		"random@example.de",
		"ob.7@example.de",
		"ob.abc.4821.uuid@example.de",
		"ob.7.notanumber.uuid@example.de",
		"ob.0.4821.uuid@example.de",
		"ob.7.0.uuid@example.de",
	} {
		_, _, parsed := parseOutboxRef(bad)
		assert.False(t, parsed, "%q must not parse", bad)
	}
}

func TestNormalizeMessageID_StripsAngleBrackets(t *testing.T) {
	assert.Equal(t, "abc@example.de", normalizeMessageID("  <abc@example.de> "))
	assert.Equal(t, "abc@example.de", normalizeMessageID("abc@example.de"))
}
