package webpush

import (
	"context"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	provider "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendPushRejectsUntrustedEndpoint(t *testing.T) {
	t.Parallel()
	sender := New(deliveryContract.WebPushConfig{Subscriber: "mailto:push@example.test", PublicKey: "public", PrivateKey: "private"}, nil)

	_, err := sender.SendPush(context.Background(), deliveryContract.ClaimedIntent{
		PushRecipient: deliveryContract.PushRecipient{Endpoint: "https://127.0.0.1/internal"},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid push endpoint")
}

func TestPushOptionsFollowPersistedPriority(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		priority string
		ttl      int
		urgency  provider.Urgency
	}{
		{priority: "high", ttl: 3600, urgency: provider.UrgencyHigh},
		{priority: "normal", ttl: 86400, urgency: provider.UrgencyNormal},
		{priority: "low", ttl: 86400, urgency: provider.UrgencyLow},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.priority, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.ttl, pushTTL(testCase.priority))
			assert.Equal(t, testCase.urgency, pushUrgency(testCase.priority))
		})
	}
}
