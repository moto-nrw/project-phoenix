package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativePublicConversationProbe struct {
	communication.PortalConversationCore
	append func(context.Context, *communication.PortalParentMessage) error
}

func (p nativePublicConversationProbe) PortalAppendMessage(ctx context.Context, v *communication.PortalParentMessage) error {
	return p.append(ctx, v)
}
func TestParentConversationBindingCopiesMutationsAfterOriginalError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cause := errors.New("original owner refusal")
	created := time.Date(2026, 10, 17, 14, 20, 0, 0, time.UTC)
	source := nativePublicConversationProbe{append: func(got context.Context, v *communication.PortalParentMessage) error {
		assert.Same(t, ctx, got)
		assert.Equal(t, "original", v.Body)
		assert.Equal(t, json.RawMessage(`9007199254740993`), v.Payload["large"])
		v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt = 41, 43, created, created.Add(time.Minute)
		v.Body = "normalized before refusal"
		v.Payload["normalized"] = json.RawMessage(`true`)
		return cause
	}}
	port := owners.BindParentConversationCore(source)
	exerciseParentConversationBinding(t, ctx, port.AppendMessage, cause, created)
}
func exerciseParentConversationBinding[T any](t *testing.T, ctx context.Context, appendMessage func(context.Context, *T) error, cause error, created time.Time) {
	t.Helper()
	var value T
	require.NoError(t, json.Unmarshal([]byte(`{"thread_id":47,"student_id":53,"body":"original","payload":{"large":9007199254740993}}`), &value))
	require.ErrorIs(t, appendMessage(ctx, &value), cause)
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.Equal(t, json.RawMessage(`41`), fields["id"])
	assert.Equal(t, json.RawMessage(`43`), fields["tenant_id"])
	expected, err := json.Marshal(created)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(expected), fields["created_at"])
	expected, err = json.Marshal(created.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(expected), fields["updated_at"])
	assert.Equal(t, json.RawMessage(`"normalized before refusal"`), fields["body"])
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fields["payload"], &payload))
	assert.Equal(t, json.RawMessage(`9007199254740993`), payload["large"])
	assert.Equal(t, json.RawMessage(`true`), payload["normalized"])
}
