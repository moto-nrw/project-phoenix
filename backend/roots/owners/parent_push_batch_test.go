package owners

import (
	"context"
	"testing"

	deliverycompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDocumentPushLookupFailureDoesNotConsumeUnrelatedRetries(t *testing.T) {
	t.Parallel()
	f, chain := newBillingPushFixture(t)
	gate, err := NewParentPushObjectAuthorization(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	// The isolated clone lets the real owner lookup fail without changing its
	// policy or affecting another test. Its authority reads still succeed.
	_, err = f.db.ExecContext(f.ctx, `ALTER TABLE parent_billing.documents RENAME TO documents_unavailable`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := f.db.ExecContext(f.ctx, `ALTER TABLE parent_billing.documents_unavailable RENAME TO documents`)
		require.NoError(t, err)
	})
	provider := &deliverycompose.MailPushProvider{
		Push:           f.transport,
		PushAuthorized: func(context.Context, delivery.ClaimedIntent) (bool, error) { return true, nil },
		PushObjects:    gate,
	}
	related := parentbilling.PushRelatedBillingDocument
	intents := []delivery.ClaimedIntent{
		{ID: 1, Template: "unrelated_notification"},
		{ID: 2, TenantID: chain.TenantID, Template: parentbilling.PushKindBillingDocument, RelatedEntityType: &related, RelatedEntityID: &f.document,
			PushRecipient: delivery.PushRecipient{AccountID: chain.AccountID, StudentIDs: []int64{chain.StudentID}}},
	}
	ctx := provider.PreparePushBatch(f.ctx, intents)
	_, err = provider.SendPush(ctx, intents[0])
	require.NoError(t, err, "an unrelated push needs no document lookup")
	require.Equal(t, 1, f.transport.calls)
	_, err = provider.SendPush(ctx, intents[1])
	require.Error(t, err)
	require.NotErrorIs(t, err, delivery.ErrCancelled, "document lookup errors stay retryable")
	require.NotContains(t, err.Error(), chain.Email)
	require.Equal(t, 1, f.transport.calls, "the document push never reaches transport")
}
