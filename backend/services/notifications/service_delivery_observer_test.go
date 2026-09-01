package notifications_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/notifications"
)

func TestNotifySynchronouslyObservesTransportOutcomeWithoutRecipientLabels(t *testing.T) {
	t.Parallel()

	deliveryErr := errors.New("push transport unavailable")
	channel := &syncChannel{
		recordingChannel: recordingChannel{name: "web_push"},
		syncErr:          deliveryErr,
	}
	type observation struct {
		transport string
		template  string
		caller    string
		duration  time.Duration
		err       error
	}
	observed := make(chan observation, 1)
	svc := notifications.NewServiceWithDeliveryObserver(
		openWindowSettings(),
		nil,
		func(transport, template, caller string, duration time.Duration, err error) {
			observed <- observation{transport, template, caller, duration, err}
		},
		channel,
	)

	err := svc.NotifySynchronously(context.Background(), syncEvent())
	require.ErrorIs(t, err, deliveryErr)

	got := <-observed
	assert.Equal(t, "web_push", got.transport)
	assert.Equal(t, notifications.TypeParentAppointmentReminder, got.template)
	assert.Equal(t, notifications.TypeParentAppointmentReminder, got.caller)
	assert.GreaterOrEqual(t, got.duration, time.Duration(0))
	assert.ErrorIs(t, got.err, deliveryErr)
	assert.NotContains(t, got.transport+got.template+got.caller, "77")
}
