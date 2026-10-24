package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	mail "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
)

// BindCommunicationMailRenderer binds native values to the same registered mail
// transport. Payload reads retain the original values, including wrong types.
func BindCommunicationMailRenderer(render communication.MailRenderer) func(context.Context, *deliveryCompose.MailIntent) (*mail.Message, error) {
	if render == nil {
		return nil
	}
	return func(ctx context.Context, row *deliveryCompose.MailIntent) (*mail.Message, error) {
		message, err := render(ctx, communicationMailIntent(row))
		if message == nil {
			return nil, err
		}
		return &mail.Message{
			From: mail.Email(message.From), To: mail.Email(message.To), ReplyTo: mail.Email(message.ReplyTo),
			Subject: message.Subject, Template: message.Template, Content: communicationMailContent(message.Content),
		}, err
	}
}

func communicationMailIntent(row *deliveryCompose.MailIntent) *communication.MailIntent {
	if row == nil {
		return nil
	}
	return &communication.MailIntent{
		ID: row.ID, TenantID: row.TenantID, Kind: row.Kind, RelatedEntityType: row.RelatedEntityType,
		RelatedEntityID: row.RelatedEntityID, Attempts: row.Attempts,
		Payload: communication.NewMailPayloadReads(
			func(key string) string { value, _ := row.Payload[key].(string); return value },
			func(key string) bool { value, _ := row.Payload[key].(bool); return value },
		),
	}
}

func communicationMailContent(content *communication.MailContent) any {
	if content == nil {
		return nil
	}
	values := make(map[string]any, len(content.Strings)+len(content.Flags))
	for key, value := range content.Strings {
		values[key] = value
	}
	for key, value := range content.Flags {
		values[key] = value
	}
	return values
}

// Enqueue binds the same captured outbox pointer to native producer values.
func (o OutboxEnqueuer) Enqueue(ctx context.Context, input communication.MailEnqueueInput) (*communication.MailEnqueued, error) {
	id, err := o.Outbox.EnqueueJSON(ctx, input.Kind, input.Payload, input.RelatedEntityType, input.RelatedEntityID, input.IdempotencyKey, input.DeliverAfter)
	if err != nil {
		return nil, err
	}
	return &communication.MailEnqueued{ID: id}, nil
}

func (o OutboxEnqueuer) EnqueueMail(ctx context.Context, input communication.MailEnqueueInput) error {
	_, err := o.Enqueue(ctx, input)
	return err
}

func (o OutboxEnqueuer) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	return o.Outbox.CancelPendingByRelatedEntity(ctx, relatedType, relatedID, reason)
}
