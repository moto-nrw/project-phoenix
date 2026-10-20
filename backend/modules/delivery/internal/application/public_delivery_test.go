package application

import (
	"context"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enqueueRecordingStore records whether validation let an intent reach the store.
type enqueueRecordingStore struct {
	*workerStore
	enqueued bool
}

func (s *enqueueRecordingStore) Enqueue(context.Context, domain.Intent) (domain.Enqueued, error) {
	s.enqueued = true
	return domain.Enqueued{}, nil
}

func TestEnqueuePushRejectsUntrustedEndpoint(t *testing.T) {
	t.Parallel()

	store := &enqueueRecordingStore{workerStore: &workerStore{}}
	service := NewService(store, guardianDirectory{}, func(domain.Observation) {})
	tenantID := int64(42)
	_, err := service.EnqueuePush(context.Background(), deliveryContract.PushIntent{
		TenantID: tenantID, Template: "appointment", IdempotencyKey: "appointment:1",
		Related:   deliveryContract.RelatedEntity{Type: "appointment", ID: 1},
		Recipient: deliveryContract.PushRecipient{Endpoint: "https://127.0.0.1/push", P256DH: "key", Auth: "auth"},
		Payload:   deliveryContract.PushPayload{Title: "Termin"},
	})

	require.Error(t, err)
	assert.False(t, store.enqueued)
}
