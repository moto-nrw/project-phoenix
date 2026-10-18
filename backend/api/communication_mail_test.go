package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	mail "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unexpectedMailMarshal func() ([]byte, error)

func (value unexpectedMailMarshal) MarshalJSON() ([]byte, error) { return value() }

func TestCommunicationMailBindingPreservesMetadataTypedReadsAndPartialMessage(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("rendering refused after partial message")
	marshals := 0
	source := &deliveryCompose.MailIntent{
		ID: 13, TenantID: 17, Kind: "kind", RelatedEntityType: "entity", RelatedEntityID: 19, Attempts: 3,
		Payload: map[string]any{"recipient": " parent@example.test ", "flag": true, "large": json.Number("9007199254740993"), "wrong": unexpectedMailMarshal(func() ([]byte, error) { marshals++; return []byte(`"must not become a string"`), nil })},
	}
	native := communication.MailRenderer(func(actual context.Context, input *communication.MailIntent) (*communication.MailMessage, error) {
		assert.Same(t, ctx, actual)
		assert.Equal(t, source.ID, input.ID)
		assert.Equal(t, source.TenantID, input.TenantID)
		assert.Equal(t, "kind", input.Kind)
		assert.Equal(t, "entity", input.RelatedEntityType)
		assert.Equal(t, source.RelatedEntityID, input.RelatedEntityID)
		assert.Equal(t, 3, input.Attempts)
		assert.Equal(t, " parent@example.test ", input.Payload.String("recipient"))
		assert.True(t, input.Payload.Bool("flag"))
		assert.False(t, input.Payload.Bool("recipient"))
		assert.Empty(t, input.Payload.String("large"))
		assert.Empty(t, input.Payload.String("wrong"))
		source.Payload["recipient"] = "updated@example.test"
		assert.Equal(t, "updated@example.test", input.Payload.String("recipient"), "reads retain the original payload, not a re-encoded snapshot")
		return &communication.MailMessage{
			From: communication.MailAddress{Name: "sender", Address: "from@example.test"}, To: communication.MailAddress{Name: "guardian", Address: "to@example.test"}, ReplyTo: communication.MailAddress{Name: "school", Address: "reply@example.test"},
			Subject: "subject", Template: "template", Content: &communication.MailContent{Strings: map[string]string{"Body": "", "Title": "title"}, Flags: map[string]bool{"AckRequired": false}},
		}, failure
	})
	message, err := BindCommunicationMailRenderer(native)(ctx, source)
	assert.Same(t, failure, err)
	require.NotNil(t, message)
	assert.Equal(t, &mail.Message{From: mail.Email{Name: "sender", Address: "from@example.test"}, To: mail.Email{Name: "guardian", Address: "to@example.test"}, ReplyTo: mail.Email{Name: "school", Address: "reply@example.test"}, Subject: "subject", Template: "template", Content: map[string]any{"Body": "", "Title": "title", "AckRequired": false}}, message)
	assert.Zero(t, marshals, "typed reads must not invoke JSON serialization")
	assert.Equal(t, json.Number("9007199254740993"), source.Payload["large"])
	assert.Nil(t, BindCommunicationMailRenderer(nil))
}

func TestCommunicationMailBindingPreservesNilInputsResultsAndEmptyContent(t *testing.T) {
	t.Parallel()
	failure := errors.New("no message")
	render := BindCommunicationMailRenderer(func(_ context.Context, input *communication.MailIntent) (*communication.MailMessage, error) {
		assert.Nil(t, input)
		return nil, failure
	})
	message, err := render(context.Background(), nil)
	assert.Nil(t, message)
	assert.Same(t, failure, err)
	for _, content := range []*communication.MailContent{nil, {}} {
		native := communication.MailRenderer(func(_ context.Context, input *communication.MailIntent) (*communication.MailMessage, error) {
			assert.Empty(t, input.Payload.String("missing"))
			assert.False(t, input.Payload.Bool("missing"))
			return &communication.MailMessage{Content: content}, nil
		})
		for _, payload := range []map[string]any{nil, {}} {
			message, err := BindCommunicationMailRenderer(native)(context.Background(), &deliveryCompose.MailIntent{Payload: payload})
			require.NoError(t, err)
			if content == nil {
				assert.Nil(t, message.Content)
			} else {
				assert.Equal(t, map[string]any{}, message.Content)
			}
		}
	}
}

func TestCommunicationAnnouncementAuditPreservesOwnerBindingAndEncodingFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("ledger rejected entry")
	calls := 0
	var recorded *auditModels.OperatorAuditEntry
	audit := NewCommunicationAnnouncementAudit(func(actual context.Context, row *auditModels.OperatorAuditEntry) error {
		assert.Same(t, ctx, actual)
		calls++
		recorded = row
		return failure
	})
	input := communicationCompose.AuditEntry{OperatorID: 13, Action: "publish", ResourceID: 17, RequestIP: net.IPv4(127, 0, 0, 1)}
	assert.Same(t, failure, audit.AppendAnnouncementAudit(ctx, input))
	assert.Equal(t, input.OperatorID, recorded.OperatorID)
	assert.Equal(t, "publish", recorded.Action)
	assert.Equal(t, "announcement", recorded.ResourceType)
	require.NotNil(t, recorded.ResourceID)
	assert.Equal(t, input.ResourceID, *recorded.ResourceID)
	assert.Equal(t, input.RequestIP, recorded.RequestIP)
	assert.Nil(t, recorded.Changes)
	input.Changes = map[string]any{}
	assert.Same(t, failure, audit.AppendAnnouncementAudit(ctx, input))
	assert.Equal(t, json.RawMessage(`{}`), recorded.Changes)
	input.Changes = map[string]any{"large": json.Number("9007199254740993")}
	assert.Same(t, failure, audit.AppendAnnouncementAudit(ctx, input))
	assert.Equal(t, json.RawMessage(`{"large":9007199254740993}`), recorded.Changes)
	before := calls
	input.Changes = map[string]any{"invalid": make(chan struct{})}
	err := audit.AppendAnnouncementAudit(ctx, input)
	var unsupported *json.UnsupportedTypeError
	assert.ErrorAs(t, err, &unsupported)
	assert.Equal(t, before, calls, "encoding failure must not reach the ledger")
}
