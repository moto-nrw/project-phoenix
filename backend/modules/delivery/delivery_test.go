package delivery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct{ pushCalled bool }

func (*recordingEngine) EnqueueEmail(context.Context, EmailIntent) (Enqueued, error) {
	return Enqueued{}, nil
}
func (e *recordingEngine) EnqueuePush(context.Context, PushIntent) (Enqueued, error) {
	e.pushCalled = true
	return Enqueued{}, nil
}
func (*recordingEngine) Cancel(context.Context, int64, Transport, RelatedEntity, string) (int64, error) {
	return 0, nil
}
func (*recordingEngine) Statuses(context.Context, int64, Transport, RelatedEntity) ([]Status, error) {
	return nil, nil
}
func (*recordingEngine) ReplaceEmailDeliveries(context.Context, int64, RelatedEntity, []EmailDelivery) error {
	return nil
}
func (*recordingEngine) DeleteEmailDeliveries(context.Context, int64, RelatedEntity) (int64, error) {
	return 0, nil
}
func (*recordingEngine) EmailDeliveryStatuses(context.Context, int64, RelatedEntity) ([]EmailDeliveryStatus, error) {
	return nil, nil
}
func (*recordingEngine) AttachEmailOutbox(context.Context, int64, int64, int64) error { return nil }
func (*recordingEngine) ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func TestEnqueuePushRejectsUntrustedEndpoint(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := NewModule(engine)
	tenantID := int64(42)
	_, err := module.EnqueuePush(context.Background(), PushIntent{
		TenantID: tenantID, Template: "appointment", IdempotencyKey: "appointment:1",
		Related:   RelatedEntity{Type: "appointment", ID: 1},
		Recipient: PushRecipient{Endpoint: "https://127.0.0.1/push", P256DH: "key", Auth: "auth"},
		Payload:   PushPayload{Title: "Termin"},
	})

	require.Error(t, err)
	assert.False(t, engine.pushCalled)
}
