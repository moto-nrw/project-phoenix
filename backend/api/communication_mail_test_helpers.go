//go:build testhooks

package api

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	mail "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

type communicationCapturingOutbox func(context.Context, communication.MailEnqueueInput) error

func (send communicationCapturingOutbox) EnqueueMail(ctx context.Context, input communication.MailEnqueueInput) error {
	return send(ctx, input)
}

// BindCommunicationCapturingOutbox keeps the existing actual capture fixture
// behind the producer's native port; no new delivery or consent fake is added.
func BindCommunicationCapturingOutbox(source interface {
	EnqueueOutbox(context.Context, deliveryCompose.MailEnqueueRequest) error
}) interface {
	EnqueueMail(context.Context, communication.MailEnqueueInput) error
} {
	if source == nil {
		return nil
	}
	return communicationCapturingOutbox(func(ctx context.Context, input communication.MailEnqueueInput) error {
		encoded, err := json.Marshal(input.Payload)
		if err != nil {
			return err
		}
		var payload map[string]any
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return err
		}
		return source.EnqueueOutbox(ctx, deliveryCompose.MailEnqueueRequest{Kind: input.Kind, Payload: payload, RelatedEntityType: input.RelatedEntityType, RelatedEntityID: input.RelatedEntityID, IdempotencyKey: input.IdempotencyKey, DeliverAfter: input.DeliverAfter})
	})
}

// RenderCommunicationMailFixture exercises the production Root binding so the
// fixture still observes the actual final transport message and content map.
func RenderCommunicationMailFixture(ctx context.Context, render communication.MailRenderer, kind string, payload map[string]any) (*mail.Message, error) {
	return BindCommunicationMailRenderer(render)(ctx, &deliveryCompose.MailIntent{Kind: kind, Payload: payload})
}
