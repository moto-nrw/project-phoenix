package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeConversationProbe struct {
	appendMessage func(context.Context, *models.ParentMessage) error
	markRead      func(context.Context, int64, int64, int64, bool, []*models.ParentMessage) (bool, error)
	decorate      func(context.Context, int64, int64, []*models.ParentMessage)
}

func (p *nativeConversationProbe) AppendMessage(ctx context.Context, row *models.ParentMessage) error {
	return p.appendMessage(ctx, row)
}
func (p *nativeConversationProbe) MarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staff bool, rows []*models.ParentMessage) (bool, error) {
	return p.markRead(ctx, tenantID, threadID, accountID, staff, rows)
}
func (p *nativeConversationProbe) DecorateReadReceipts(ctx context.Context, threadID, accountID int64, rows []*models.ParentMessage) {
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
	source := &nativeConversationProbe{appendMessage: func(got context.Context, row *models.ParentMessage) error {
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
	core := parentstore.NewPortalConversationCore(source)
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
	source := &nativeConversationProbe{decorate: func(_ context.Context, threadID, accountID int64, rows []*models.ParentMessage) {
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
	core := parentstore.NewPortalConversationCore(source)
	row := &communication.PortalParentMessage{ID: 23, CreatedAt: created, SenderKind: "system", Kind: "event", EventType: "request_created", EventActorKind: "guardian", Payload: payload}
	core.PortalDecorateReadReceipts(context.Background(), 17, 19, []*communication.PortalParentMessage{row})
	assert.True(t, row.ReadByStaff)
	assert.True(t, row.ReadByGuardian)
	assert.Equal(t, payload, row.Payload)
	assert.Equal(t, json.RawMessage(`not-json`), row.Payload["opaque"])
}

func TestParentNativeDeclarationPolicyRecomputesCanonicalCurrentValues(t *testing.T) {
	t.Parallel()
	policy := parentstore.NewPortalDeclarationPolicy()
	version := communication.PortalDeclarationVersion{TenantID: 13, AnnouncementID: 17, Title: "Title", Body: "Original", Kind: "consent", Attachments: []communication.PortalDeclarationAttachmentDigest{{AttachmentID: 29, Filename: "second", SizeBytes: 31}, {AttachmentID: 23, Filename: "first", SizeBytes: 37}}}
	original := models.DeclarationVersion{Title: version.Title, Body: version.Body, Kind: version.Kind, Attachments: []models.DeclarationAttachmentDigest{{AttachmentID: 29, Filename: "second", SizeBytes: 31}, {AttachmentID: 23, Filename: "first", SizeBytes: 37}}}
	before := policy.PortalCanonicalContent(version)
	assert.Equal(t, original.CanonicalContent(), before)
	version.Body = "Changed after reading"
	assert.NotEqual(t, before, policy.PortalCanonicalContent(version))
	assert.Equal(t, int64(29), version.Attachments[0].AttachmentID, "canonical sorting must not mutate the caller's order")
	guardian := int64(41)
	record := communication.PortalDeclarationSubmission{TenantID: 13, AnnouncementID: 17, VersionID: 19, StudentID: 43, GuardianProfileID: &guardian, SignerName: "Guardian", GuardianRole: "parent", Action: "agreed", Method: "simple_electronic", PasswordConfirmed: true, ContentHash: "content", SubmittedAt: time.Date(2026, 10, 17, 10, 11, 12, 123456000, time.FixedZone("offset", 7200))}
	legacy := models.DeclarationSubmission{TenantID: record.TenantID, AnnouncementID: record.AnnouncementID, VersionID: record.VersionID, StudentID: record.StudentID, GuardianProfileID: record.GuardianProfileID, SignerName: record.SignerName, GuardianRole: record.GuardianRole, Action: record.Action, Method: record.Method, PasswordConfirmed: record.PasswordConfirmed, ContentHash: record.ContentHash, SubmittedAt: record.SubmittedAt}
	before = policy.PortalCanonicalRecord(record)
	assert.Equal(t, legacy.CanonicalRecord(), before)
	record.GuardianRole = "changed"
	assert.NotEqual(t, before, policy.PortalCanonicalRecord(record))
}
