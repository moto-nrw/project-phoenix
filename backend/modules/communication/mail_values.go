package communication

import (
	"context"
	"encoding/json"
	"time"
)

// MailPayload retains the complete encoded write object without a float decode.
type MailPayload map[string]json.RawMessage

// MailEnqueueInput is the producer's complete durable mail request.
type MailEnqueueInput struct {
	Kind              string
	Payload           MailPayload
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
	DeliverAfter      time.Time
}
type MailEnqueued struct{ ID int64 }

// MailPayloadReads preserves the renderer's exact typed reads over a claimed
// payload. Root binds the existing values without serializing them again.
type MailPayloadReads interface {
	String(string) string
	Bool(string) bool
}
type mailPayloadReads struct {
	stringValue func(string) string
	boolValue   func(string) bool
}

func NewMailPayloadReads(strings func(string) string, flags func(string) bool) MailPayloadReads {
	return mailPayloadReads{stringValue: strings, boolValue: flags}
}
func (read mailPayloadReads) String(key string) string { return read.stringValue(key) }
func (read mailPayloadReads) Bool(key string) bool     { return read.boolValue(key) }

// MailIntent retains all claimed metadata and the read-only payload capability.
type MailIntent struct {
	ID                int64
	TenantID          int64
	Kind              string
	RelatedEntityType string
	RelatedEntityID   int64
	Payload           MailPayloadReads
	Attempts          int
}
type MailAddress struct {
	Name    string
	Address string
}

// MailContent represents the exact string and boolean values of these templates.
// A nil content pointer and a non-nil empty object remain distinct.
type MailContent struct {
	Strings map[string]string
	Flags   map[string]bool
}
type MailMessage struct {
	From     MailAddress
	To       MailAddress
	ReplyTo  MailAddress
	Subject  string
	Template string
	Content  *MailContent
}
type MailRendererConfig struct{ DefaultFrom MailAddress }
type MailRenderer func(context.Context, *MailIntent) (*MailMessage, error)

// Existing stored mail protocol values, independent of delivery implementation.
const (
	MailKindParentMessage        = "parent_message"
	MailRelatedTypeParentMessage = "parent_message"
	MailKindParentAnnouncement   = "parent_announcement"
)
