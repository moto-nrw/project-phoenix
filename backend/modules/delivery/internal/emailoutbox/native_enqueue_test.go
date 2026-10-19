package emailoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeEmailProbe func(context.Context, DurableEmail) (DurableEmailResult, error)

func (send nativeEmailProbe) EnqueueEmail(ctx context.Context, input DurableEmail) (DurableEmailResult, error) {
	return send(ctx, input)
}
func (nativeEmailProbe) CancelEmail(context.Context, int64, string, int64, string) (int64, error) {
	return 0, nil
}

func TestNativeEnqueueSharesTenantPayloadMetadataAndOriginalFailure(t *testing.T) {
	t.Parallel()
	ctx := tenantContext(t, int64(len("native-school")))
	storedID := int64(len("stored-intent"))
	relatedID := int64(len("related-announcement"))
	after := time.Date(2026, time.October, 17, 15, 42, 0, 0, time.UTC)
	original := map[string]any{"recipient_email": "parent@example.test", "large": json.Number("9007199254740993"), "nested": map[string]any{"empty": map[string]any{}, "nil": nil}, "flag": false}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &raw))
	var calls []DurableEmail
	failure := errors.New("store rejected intent")
	reject := false
	source := nativeEmailProbe(func(actual context.Context, input DurableEmail) (DurableEmailResult, error) {
		assert.Same(t, ctx, actual)
		calls = append(calls, input)
		if reject {
			return DurableEmailResult{ID: storedID}, failure
		}
		return DurableEmailResult{ID: storedID, Duplicate: true}, nil
	})
	service := NewService(source)
	legacy, err := service.Enqueue(ctx, EnqueueRequest{Kind: "kind", Payload: original, RelatedEntityType: "entity", RelatedEntityID: relatedID, IdempotencyKey: "key", DeliverAfter: after})
	require.NoError(t, err)
	id, err := service.EnqueueJSON(ctx, "kind", raw, "entity", relatedID, "key", after)
	require.NoError(t, err)
	assert.Equal(t, legacy.ID, id)
	require.Len(t, calls, 2)
	assert.Equal(t, calls[0], calls[1])
	assert.Equal(t, json.RawMessage(encoded), calls[1].Payload)
	assert.Contains(t, string(calls[1].Payload), "9007199254740993")
	assert.Equal(t, after, calls[1].DeliverAfter)
	reject = true
	id, err = service.EnqueueJSON(ctx, "kind", raw, "entity", relatedID, "key", after)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, failure)
	assert.Equal(t, "enqueue email outbox row: store rejected intent", err.Error())
}

func TestNativeEnqueuePreservesValidationBeforeTenantAndEncoding(t *testing.T) {
	t.Parallel()
	valid := json.RawMessage(`"parent@example.test"`)
	bad := json.RawMessage(`not-json`)
	for _, tc := range []struct {
		name, kind string
		payload    map[string]json.RawMessage
		want       string
		withTenant bool
	}{
		{name: "kind before payload and tenant", want: "outbox kind is required"},
		{name: "nil payload before tenant", kind: "kind", want: "outbox payload is required"},
		{name: "empty payload before tenant", kind: "kind", payload: map[string]json.RawMessage{}, want: "outbox recipient_email is required"},
		{name: "wrong recipient before bad JSON and tenant", kind: "kind", payload: map[string]json.RawMessage{"recipient_email": json.RawMessage(`42`), "bad": bad}, want: "outbox recipient_email is required"},
		{name: "tenant before encoding", kind: "kind", payload: map[string]json.RawMessage{"recipient_email": valid, "bad": bad}, want: "enqueue email outbox row: tenant is required:"},
		{name: "encoding before store", kind: "kind", payload: map[string]json.RawMessage{"recipient_email": valid, "bad": bad}, withTenant: true, want: "enqueue email outbox row: encode payload:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := nativeEmailProbe(func(context.Context, DurableEmail) (DurableEmailResult, error) {
				t.Fatal("invalid intent reached persistence")
				return DurableEmailResult{}, nil
			})
			ctx := context.Background()
			if tc.withTenant {
				ctx = tenantContext(t, int64(len("native-school")))
			}
			id, err := NewService(source).EnqueueJSON(ctx, tc.kind, tc.payload, "", 0, "", time.Time{})
			assert.Zero(t, id)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
	var absent *Service
	_, err := absent.EnqueueJSON(context.Background(), "", nil, "", 0, "", time.Time{})
	assert.EqualError(t, err, "outbox service not wired")
}
