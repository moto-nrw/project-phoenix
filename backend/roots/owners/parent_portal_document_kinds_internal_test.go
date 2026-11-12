package owners

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

func kindIntent(t *testing.T, kind, relatedType string, organizationID, relatedID int64, payload any) deliveryContract.ClaimedIntent {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return deliveryContract.ClaimedIntent{
		OrganizationID: organizationID, Transport: deliveryContract.TransportPush, Template: kind,
		RelatedEntityType: &relatedType, RelatedEntityID: &relatedID, DeferredPayload: raw,
		PushRecipient: deliveryContract.PushRecipient{Deferred: true},
	}
}

// The worker routes the pushes of a contract, a release and a release's
// account to the parents portal (#5941). A payload whose Träger, related
// entity or related type differs from the outbox row is cancelled, never
// sent: a contract and a release with the same numeric ID never stand in for
// each other.
func TestDeferredPushesRouteContractAndReleaseNotices(t *testing.T) {
	t.Parallel()
	documents := &documentNoticesDouble{}
	route := deferredPushes(func(context.Context, deliveryContract.ClaimedIntent) error { return nil }, documents)
	contract := carecontract.ContractDocumentNotice{OrganizationID: 4, ContractID: 31, RecipientEmail: "eva@example.org"}
	release := organizationtenancy.TraegerDocumentReleaseNotice{OrganizationID: 4, ReleaseID: 31, DocumentID: 9}
	account := releaseAccountNotice{OrganizationID: 4, ReleaseID: 31, AccountID: 77}

	require.NoError(t, route(context.Background(), kindIntent(t, carecontract.PushKindContractDocument, carecontract.PushRelatedContractDocument, 4, 31, contract)))
	require.NoError(t, route(context.Background(), kindIntent(t, organizationtenancy.PushKindTraegerDocumentRelease, organizationtenancy.PushRelatedTraegerDocumentRelease, 4, 31, release)))
	require.NoError(t, route(context.Background(), kindIntent(t, pushKindTraegerDocumentReleaseAccount, organizationtenancy.PushRelatedTraegerDocumentRelease, 4, 31, account)))
	assert.Equal(t, []recordedDocumentNotice{{4, 31, "contract:eva@example.org"}, {4, 31, "release"}, {4, 31, "release-account:77"}}, documents.got)

	for name, intent := range map[string]deliveryContract.ClaimedIntent{
		"contract under a release's type": kindIntent(t, carecontract.PushKindContractDocument, organizationtenancy.PushRelatedTraegerDocumentRelease, 4, 31, contract),
		"release under a contract's type": kindIntent(t, organizationtenancy.PushKindTraegerDocumentRelease, carecontract.PushRelatedContractDocument, 4, 31, release),
		"contract of another Träger":      kindIntent(t, carecontract.PushKindContractDocument, carecontract.PushRelatedContractDocument, 5, 31, contract),
		"another release":                 kindIntent(t, organizationtenancy.PushKindTraegerDocumentRelease, organizationtenancy.PushRelatedTraegerDocumentRelease, 4, 32, release),
		"account entry without account": kindIntent(t, pushKindTraegerDocumentReleaseAccount, organizationtenancy.PushRelatedTraegerDocumentRelease, 4, 31,
			releaseAccountNotice{OrganizationID: 4, ReleaseID: 31}),
	} {
		assert.ErrorIs(t, route(context.Background(), intent), deliveryContract.ErrCancelled, name)
	}
	assert.Len(t, documents.got, 3, "a mismatched push reaches no parent")
}

// recordingOutbox records the deferred pushes a fan-out enqueues.
type recordingOutbox struct {
	intents []deliveryContract.DeferredPushIntent
}

func (o *recordingOutbox) EnqueueDeferredPush(_ context.Context, intent deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error) {
	o.intents = append(o.intents, intent)
	return deliveryContract.Enqueued{}, nil
}

// A release fans out to one keyed entry per account; the contract push is
// keyed by the contract and a hash of the address, never the address.
func TestDocumentPushKeysNameKindDocumentAndRecipient(t *testing.T) {
	t.Parallel()
	outbox := &recordingOutbox{}
	require.NoError(t, ParentReleaseFanOut{Delivery: outbox}.EnqueueReleaseAccounts(context.Background(), 4, 31, []int64{77, 78}))
	require.NoError(t, CareContractDocumentNotices{Delivery: outbox}.EnqueueContractDocumentNotice(context.Background(),
		carecontract.ContractDocumentNotice{OrganizationID: 4, ContractID: 31, RecipientEmail: " Eva@Example.org "}))
	require.Len(t, outbox.intents, 3)
	assert.Equal(t, "traeger-document-release:31:account:77", outbox.intents[0].IdempotencyKey)
	assert.Equal(t, "traeger-document-release:31:account:78", outbox.intents[1].IdempotencyKey)
	assert.Equal(t, contractDocumentNoticeKey(31, "eva@example.org"), outbox.intents[2].IdempotencyKey, "case and spaces do not make a new recipient")
	assert.NotContains(t, outbox.intents[2].IdempotencyKey, "example.org")
	assert.NotEqual(t, contractDocumentNoticeKey(31, "eva@example.org"), contractDocumentNoticeKey(31, "korrigiert@example.org"))
}
