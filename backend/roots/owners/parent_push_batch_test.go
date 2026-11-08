package owners

import (
	"context"
	"errors"
	"testing"

	deliverycompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
)

type unavailableDocumentPushAuthority struct{}

func (unavailableDocumentPushAuthority) AuthorizeDocumentPushes(context.Context, []parentportal.DocumentPush) (map[int64]bool, error) {
	return nil, errors.New("private-recipient@example.org")
}

func TestDocumentPushLookupFailureDoesNotConsumeUnrelatedRetries(t *testing.T) {
	t.Parallel()
	transport := &documentPushTransport{}
	provider := &deliverycompose.MailPushProvider{
		Push:           transport,
		PushAuthorized: func(context.Context, delivery.ClaimedIntent) (bool, error) { return true, nil },
		PushObjects:    BindParentPushObjectAuthorization(unavailableDocumentPushAuthority{}),
	}
	related, documentID := parentbilling.PushRelatedBillingDocument, int64(123)
	intents := []delivery.ClaimedIntent{
		{ID: 1, Template: "unrelated_notification"},
		{ID: 2, Template: parentbilling.PushKindBillingDocument, RelatedEntityType: &related, RelatedEntityID: &documentID},
	}
	ctx := provider.PreparePushBatch(context.Background(), intents)
	_, err := provider.SendPush(ctx, intents[0])
	require.NoError(t, err, "an unrelated push needs no document lookup")
	require.Equal(t, 1, transport.calls)
	_, err = provider.SendPush(ctx, intents[1])
	require.Error(t, err)
	require.NotErrorIs(t, err, delivery.ErrCancelled, "document lookup errors stay retryable")
	require.NotContains(t, err.Error(), "private-recipient")
	require.Equal(t, 1, transport.calls, "the document push never reaches transport")
}
