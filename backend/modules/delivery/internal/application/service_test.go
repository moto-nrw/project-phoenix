package application

import (
	"context"
	"errors"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type guardianDirectory struct {
	displays []deliveryContract.GuardianDisplay
	err      error
}

func (d guardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return d.displays, d.err
}

func TestEmailDeliveryStatusesFailsWhenGuardianDisplayLookupFails(t *testing.T) {
	t.Parallel()
	profileID := int64(51)
	store := &workerStore{statuses: []deliveryContract.EmailDeliveryStatus{{DeliveryID: 7, GuardianProfileID: &profileID}}}
	lookupErr := errors.New("people directory unavailable")
	service := NewService(store, guardianDirectory{err: lookupErr}, func(domain.Observation) {})

	_, err := service.EmailDeliveryStatuses(context.Background(), 3, deliveryContract.RelatedEntity{Type: "announcement", ID: 9})

	require.Error(t, err)
	assert.ErrorIs(t, err, lookupErr)
}

func TestEmailDeliveryStatusesAddsNamesAndSorts(t *testing.T) {
	t.Parallel()
	firstID, secondID := int64(51), int64(52)
	store := &workerStore{statuses: []deliveryContract.EmailDeliveryStatus{
		{DeliveryID: 8, GuardianProfileID: &secondID},
		{DeliveryID: 7, GuardianProfileID: &firstID},
	}}
	service := NewService(store, guardianDirectory{displays: []deliveryContract.GuardianDisplay{
		{GuardianProfileID: firstID, FirstName: "Ada", LastName: "A"},
		{GuardianProfileID: secondID, FirstName: "Berta", LastName: "B"},
	}}, func(domain.Observation) {})

	rows, err := service.EmailDeliveryStatuses(context.Background(), 3, deliveryContract.RelatedEntity{Type: "announcement", ID: 9})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Ada", rows[0].FirstName)
	assert.Equal(t, "Berta", rows[1].FirstName)
}

// enqueueRecorder records the intents the service hands to the store.
type enqueueRecorder struct {
	*workerStore
	intents []domain.Intent
}

func (r *enqueueRecorder) Enqueue(_ context.Context, intent domain.Intent) (domain.Enqueued, error) {
	r.intents = append(r.intents, intent)
	return domain.Enqueued{ID: int64(len(r.intents))}, nil
}

// A deferred push names a school or a Träger (ADR 0126), never both and
// never neither; a push of a Träger keeps its Träger for the store.
func TestEnqueueDeferredPushNamesExactlyOneScope(t *testing.T) {
	t.Parallel()
	store := &enqueueRecorder{workerStore: &workerStore{}}
	service := NewService(store, guardianDirectory{}, func(domain.Observation) {})
	intent := deliveryContract.DeferredPushIntent{
		Template: "parent_billing_document", IdempotencyKey: "parent-billing-document:31", Payload: []byte(`{"document_id":31}`),
		Related: deliveryContract.RelatedEntity{Type: "parent_billing_document", ID: 31},
	}

	for name, scope := range map[string][2]int64{"neither": {0, 0}, "both": {21, 4}, "negative Träger": {0, -4}} {
		refused := intent
		refused.TenantID, refused.OrganizationID = scope[0], scope[1]
		_, err := service.EnqueueDeferredPush(context.Background(), refused)
		assert.Error(t, err, name)
	}
	assert.Empty(t, store.intents, "a refused push reaches no store")

	traeger := intent
	traeger.OrganizationID = 4
	queued, err := service.EnqueueDeferredPush(context.Background(), traeger)
	require.NoError(t, err)
	assert.Equal(t, deliveryContract.TransportPush, queued.Transport)
	require.Len(t, store.intents, 1)
	stored := store.intents[0]
	assert.Equal(t, int64(4), stored.OrganizationID)
	assert.Zero(t, stored.TenantID)
	assert.Equal(t, domain.TransportPush, stored.Transport)
	assert.JSONEq(t, `{"deferred":true}`, string(stored.Recipient))
}
