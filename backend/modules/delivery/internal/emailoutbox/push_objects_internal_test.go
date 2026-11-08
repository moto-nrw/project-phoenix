package emailoutbox

import (
	"context"
	"errors"
	"testing"

	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type objectPushTransport struct{ calls int }

func (p *objectPushTransport) SendPush(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	p.calls++
	return delivery.ProviderResult{}, nil
}

func TestPushObjectBatchIsRunLocalAndFailsClosed(t *testing.T) {
	t.Parallel()
	sender := &objectPushTransport{}
	reads := 0
	allowed := true
	var readErr error
	provider := &MailPushProvider{Push: sender, PushAuthorized: func(context.Context, delivery.ClaimedIntent) (bool, error) { return true, nil },
		PushObjects: func(_ context.Context, intents []delivery.ClaimedIntent) (map[int64]bool, error) {
			reads++
			require.Len(t, intents, 2)
			return map[int64]bool{1: allowed}, readErr
		},
	}
	intents := []delivery.ClaimedIntent{{ID: 1}, {ID: 2}}
	ctx := provider.PreparePushBatch(context.Background(), intents)
	_, err := provider.SendPush(ctx, intents[0])
	require.NoError(t, err)
	_, err = provider.SendPush(ctx, intents[1])
	require.ErrorIs(t, err, delivery.ErrCancelled, "missing decision denies")
	assert.Equal(t, 1, reads, "one lookup for both devices")
	assert.Equal(t, 1, sender.calls)
	allowed = false
	ctx = provider.PreparePushBatch(context.Background(), intents)
	_, err = provider.SendPush(ctx, intents[0])
	require.ErrorIs(t, err, delivery.ErrCancelled, "retry reads current authority")
	readErr = errors.New("private-recipient@example.org")
	ctx = provider.PreparePushBatch(context.Background(), intents)
	_, err = provider.SendPush(ctx, intents[0])
	require.Error(t, err)
	assert.NotErrorIs(t, err, delivery.ErrCancelled, "lookup failures retry")
	assert.NotContains(t, err.Error(), "private-recipient")
	assert.Equal(t, 1, sender.calls)
}

func TestTargetedPushChecksObjectWithoutBatch(t *testing.T) {
	t.Parallel()
	sender := &objectPushTransport{}
	reads := 0
	provider := &MailPushProvider{Push: sender, PushAuthorized: func(context.Context, delivery.ClaimedIntent) (bool, error) { return true, nil },
		PushObjects: func(_ context.Context, intents []delivery.ClaimedIntent) (map[int64]bool, error) {
			reads++
			require.Len(t, intents, 1)
			return map[int64]bool{}, nil
		},
	}
	_, err := provider.SendPush(context.Background(), delivery.ClaimedIntent{ID: 3})
	require.ErrorIs(t, err, delivery.ErrCancelled)
	assert.Equal(t, 1, reads)
	assert.Zero(t, sender.calls)
}
