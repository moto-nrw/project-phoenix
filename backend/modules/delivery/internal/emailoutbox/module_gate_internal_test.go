package emailoutbox

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

// A claimed intent of a cancelled module is cancelled before anything is
// rendered or sent (#4895).
func TestProviderCancelsIntentsOfACancelledModule(t *testing.T) {
	t.Parallel()
	provider := &MailPushProvider{Deliverable: func(context.Context, deliveryContract.ClaimedIntent) (bool, error) { return false, nil }}

	_, err := provider.SendEmail(context.Background(), deliveryContract.ClaimedIntent{Transport: deliveryContract.TransportEmail})
	require.ErrorIs(t, err, deliveryContract.ErrCancelled)
	_, err = provider.SendPush(context.Background(), deliveryContract.ClaimedIntent{Transport: deliveryContract.TransportPush})
	require.ErrorIs(t, err, deliveryContract.ErrCancelled)
}

func TestProviderRetriesWhenTheBookingCannotBeRead(t *testing.T) {
	t.Parallel()
	readErr := errors.New("settings unavailable")
	provider := &MailPushProvider{Deliverable: func(context.Context, deliveryContract.ClaimedIntent) (bool, error) { return false, readErr }}

	_, err := provider.SendEmail(context.Background(), deliveryContract.ClaimedIntent{Transport: deliveryContract.TransportEmail})
	require.ErrorIs(t, err, readErr)
	assert.NotErrorIs(t, err, deliveryContract.ErrCancelled)
}
