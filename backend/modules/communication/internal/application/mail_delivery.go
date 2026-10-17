package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// MailEnqueueInput is the complete application-owned producer value.
type MailEnqueueInput struct {
	Kind              string
	Payload           communication.MailPayload
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
	DeliverAfter      time.Time
}

func (input MailEnqueueInput) Public() communication.MailEnqueueInput {
	return communication.MailEnqueueInput(input)
}

type MailEnqueuer interface {
	EnqueueMail(context.Context, communication.MailEnqueueInput) error
}
type MailOutbox interface {
	Enqueue(context.Context, communication.MailEnqueueInput) (*communication.MailEnqueued, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}

// JSONString and JSONBool encode only infallible primitive producer values.
func JSONString(value string) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }
func JSONBool(value bool) json.RawMessage     { encoded, _ := json.Marshal(value); return encoded }

// MailIntent and MailMessage belong to the application. Nested values are its
// owner's public contracts; Root never needs the private application types.
type MailIntent struct {
	ID                int64
	TenantID          int64
	Kind              string
	RelatedEntityType string
	RelatedEntityID   int64
	Payload           communication.MailPayloadReads
	Attempts          int
}
type MailMessage struct {
	From     communication.MailAddress
	To       communication.MailAddress
	ReplyTo  communication.MailAddress
	Subject  string
	Template string
	Content  *communication.MailContent
}
type MailRendererConfig struct{ DefaultFrom communication.MailAddress }
type MailRenderer func(context.Context, *communication.MailIntent) (*communication.MailMessage, error)

// NewMailRenderer binds the native application callback with exact pointer,
// context, nil and partial-result/error semantics.
func NewMailRenderer(render func(context.Context, *MailIntent) (*MailMessage, error)) MailRenderer {
	if render == nil {
		return nil
	}
	return func(ctx context.Context, input *communication.MailIntent) (*communication.MailMessage, error) {
		message, err := render(ctx, (*MailIntent)(input))
		return (*communication.MailMessage)(message), err
	}
}
func NewMailAddress(name, address string) communication.MailAddress {
	return communication.MailAddress{Name: name, Address: address}
}
func NewMailContent(strings map[string]string, flags map[string]bool) *communication.MailContent {
	return &communication.MailContent{Strings: strings, Flags: flags}
}

const (
	MailKindParentMessage        = communication.MailKindParentMessage
	MailRelatedTypeParentMessage = communication.MailRelatedTypeParentMessage
	MailKindParentAnnouncement   = communication.MailKindParentAnnouncement
)

// NewMailReceiptIndex allocates the application's address-deduplication index.
func NewMailReceiptIndex() map[string]*communication.MailEnqueued {
	return make(map[string]*communication.MailEnqueued)
}
