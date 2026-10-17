package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeConversationProbe struct {
	appendMessage func(context.Context, *ParentMessage) error
	markRead      func(context.Context, int64, int64, int64, bool, []*ParentMessage) (bool, error)
	decorate      func(context.Context, int64, int64, []*ParentMessage)
}

func (p *nativeConversationProbe) AppendMessage(ctx context.Context, row *ParentMessage) error {
	return p.appendMessage(ctx, row)
}
func (p *nativeConversationProbe) MarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staff bool, rows []*ParentMessage) (bool, error) {
	return p.markRead(ctx, tenantID, threadID, accountID, staff, rows)
}
func (p *nativeConversationProbe) DecorateReadReceipts(ctx context.Context, threadID, accountID int64, rows []*ParentMessage) {
	p.decorate(ctx, threadID, accountID, rows)
}
func (*nativeConversationProbe) Broadcast(int64, int64, int64, int64)     {}
func (*nativeConversationProbe) BroadcastRead(int64, int64, int64, int64) {}

func TestParentNativeConversationRetainsMutationOnOwnerErrorAndJSONNumbers(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cause := errors.New("original owner refusal")
	created := time.Date(2026, 10, 17, 14, 20, 0, 0, time.UTC)
	source := &nativeConversationProbe{appendMessage: func(got context.Context, row *ParentMessage) error {
		assert.Same(t, ctx, got)
		assert.Equal(t, json.Number("9007199254740993"), row.Payload["large"])
		nested, ok := row.Payload["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, json.Number("9223372036854775807"), nested["value"])
		row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = 41, 43, created, created.Add(time.Minute)
		row.Body = "normalized before refusal"
		row.Payload["normalized"] = true
		return cause
	}}
	core := BindParentConversationCore(source.AppendMessage, source.MarkReadToNewest, source.DecorateReadReceipts, source.Broadcast, source.BroadcastRead)
	value := &communication.PortalParentMessage{ThreadID: 47, StudentID: 53, Body: "original", Payload: map[string]json.RawMessage{"large": json.RawMessage(`9007199254740993`), "nested": json.RawMessage(`{"value":9223372036854775807}`)}}
	err := core.PortalAppendMessage(ctx, value)
	require.ErrorIs(t, err, cause)
	assert.Equal(t, int64(41), value.ID)
	assert.Equal(t, int64(43), value.TenantID)
	assert.Equal(t, created, value.CreatedAt)
	assert.Equal(t, created.Add(time.Minute), value.UpdatedAt)
	assert.Equal(t, "normalized before refusal", value.Body)
	assert.Equal(t, json.RawMessage(`9007199254740993`), value.Payload["large"])
	assert.Equal(t, json.RawMessage(`true`), value.Payload["normalized"])
}

func TestParentNativeReadReceiptsConsumeCursorFactsWithoutChangingPayload(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	payload := map[string]json.RawMessage{"opaque": json.RawMessage(`not-json`)}
	source := &nativeConversationProbe{decorate: func(_ context.Context, threadID, accountID int64, rows []*ParentMessage) {
		assert.Equal(t, int64(17), threadID)
		assert.Equal(t, int64(19), accountID)
		require.Len(t, rows, 1)
		assert.Equal(t, int64(23), rows[0].ID)
		assert.Equal(t, created, rows[0].CreatedAt)
		assert.Equal(t, "request_created", rows[0].EventType)
		assert.Equal(t, "guardian", rows[0].EventActorKind)
		rows[0].ReadByStaff = true
		rows[0].ReadByGuardian = true
	}}
	core := BindParentConversationCore(source.AppendMessage, source.MarkReadToNewest, source.DecorateReadReceipts, source.Broadcast, source.BroadcastRead)
	row := &communication.PortalParentMessage{ID: 23, CreatedAt: created, SenderKind: "system", Kind: "event", EventType: "request_created", EventActorKind: "guardian", Payload: payload}
	core.PortalDecorateReadReceipts(context.Background(), 17, 19, []*communication.PortalParentMessage{row})
	assert.True(t, row.ReadByStaff)
	assert.True(t, row.ReadByGuardian)
	assert.Equal(t, payload, row.Payload)
	assert.Equal(t, json.RawMessage(`not-json`), row.Payload["opaque"])
}
