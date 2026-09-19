package emailoutbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type recordingDurableEmail struct {
	enqueued []DurableEmail
	result   DurableEmailResult
}

func (r *recordingDurableEmail) EnqueueEmail(_ context.Context, input DurableEmail) (DurableEmailResult, error) {
	r.enqueued = append(r.enqueued, input)
	return r.result, nil
}

func (r *recordingDurableEmail) CancelEmail(context.Context, int64, string, int64, string) (int64, error) {
	return 0, nil
}

func tenantContext(t *testing.T, id int64) context.Context {
	t.Helper()
	tenantID, err := tenant.NewTenantID(id)
	require.NoError(t, err)
	return tenant.WithTenant(context.Background(), tenantID)
}

// Enqueue hands Delivery one intent keyed on the caller's tenant and reports
// the stored intent's identity.
func TestServiceEnqueueHandsTheTenantIntentToDelivery(t *testing.T) {
	t.Parallel()

	const schoolID, invitationID int64 = 7, 3
	delivery := &recordingDurableEmail{result: DurableEmailResult{ID: 91}}
	service := NewService(delivery)

	enqueued, err := service.Enqueue(tenantContext(t, schoolID), EnqueueRequest{
		Kind:              "guardian_invitation",
		Payload:           map[string]any{"recipient_email": "eltern@example.test", "first_name": "Ada"},
		RelatedEntityType: "guardian_invitation",
		RelatedEntityID:   invitationID,
		IdempotencyKey:    "invite-3",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(91), enqueued.ID)
	require.Len(t, delivery.enqueued, 1)
	intent := delivery.enqueued[0]
	assert.Equal(t, schoolID, intent.TenantID)
	assert.Equal(t, "guardian_invitation", intent.Template)
	assert.Equal(t, "eltern@example.test", intent.Recipient)
	assert.Equal(t, "guardian_invitation", intent.RelatedType)
	assert.Equal(t, invitationID, intent.RelatedID)
	assert.Equal(t, "invite-3", intent.IdempotencyKey)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(intent.Payload, &payload))
	assert.Equal(t, "Ada", payload["first_name"])
}

// An intent without an addressee or outside a tenant never reaches Delivery.
func TestServiceEnqueueRejectsIncompleteRequests(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		ctx     context.Context
		request EnqueueRequest
	}{
		"missing kind":      {tenantContext(t, 7), EnqueueRequest{Payload: map[string]any{"recipient_email": "a@example.test"}}},
		"missing payload":   {tenantContext(t, 7), EnqueueRequest{Kind: "k"}},
		"missing recipient": {tenantContext(t, 7), EnqueueRequest{Kind: "k", Payload: map[string]any{}}},
		"missing tenant":    {context.Background(), EnqueueRequest{Kind: "k", Payload: map[string]any{"recipient_email": "a@example.test"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			delivery := &recordingDurableEmail{}
			_, err := NewService(delivery).Enqueue(tc.ctx, tc.request)
			require.Error(t, err)
			assert.Empty(t, delivery.enqueued)
		})
	}
}

// The registry answers the renderers it was built with and nothing else;
// changing the source map afterwards does not reach it.
func TestTemplateRegistryIsFixedAtConstruction(t *testing.T) {
	t.Parallel()
	var renderer RendererFunc
	source := map[string]Renderer{"known": renderer}
	registry := NewTemplateRegistry(source)
	source["later"] = renderer

	_, err := registry.Lookup("known")
	require.NoError(t, err)

	_, err = registry.Lookup("later")
	require.Error(t, err)
	assert.Equal(t, []string{"known"}, registry.Kinds())
}
