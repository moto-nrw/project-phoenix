package application

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// recordedMailRequest is the decoded observation of the native enqueue input.
// Existing fixture assertions continue to inspect the values persisted as JSON.
type recordedMailRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
	DeliverAfter      time.Time
}

func recordMailRequest(input communication.MailEnqueueInput) recordedMailRequest {
	encoded, err := json.Marshal(input.Payload)
	if err != nil {
		panic(err)
	}
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		panic(err)
	}
	return recordedMailRequest{Kind: input.Kind, Payload: payload, RelatedEntityType: input.RelatedEntityType, RelatedEntityID: input.RelatedEntityID, IdempotencyKey: input.IdempotencyKey, DeliverAfter: input.DeliverAfter}
}

type recordedMailIntent struct {
	ID                int64
	TenantID          int64
	Kind              string
	RelatedEntityType string
	RelatedEntityID   int64
	Payload           map[string]any
	Attempts          int
}
type renderedAnnouncementMail struct {
	From     communication.MailAddress
	To       communication.MailAddress
	ReplyTo  communication.MailAddress
	Subject  string
	Template string
	Content  any
}

// announcementRendererFixture runs the real native renderer and observes the
// same string/boolean template values as the final transport projection.
func announcementRendererFixture(cfg EmailConfig) func(context.Context, *recordedMailIntent) (*renderedAnnouncementMail, error) {
	render := NewAnnouncementRenderer(cfg)
	return func(ctx context.Context, row *recordedMailIntent) (*renderedAnnouncementMail, error) {
		input := &communication.MailIntent{
			ID: row.ID, TenantID: row.TenantID, Kind: row.Kind, RelatedEntityType: row.RelatedEntityType, RelatedEntityID: row.RelatedEntityID, Attempts: row.Attempts,
			Payload: communication.NewMailPayloadReads(func(key string) string { v, _ := row.Payload[key].(string); return v }, func(key string) bool { v, _ := row.Payload[key].(bool); return v }),
		}
		message, err := render(ctx, input)
		if message == nil {
			return nil, err
		}
		var content any
		if message.Content != nil {
			values := make(map[string]any)
			for key, value := range message.Content.Strings {
				values[key] = value
			}
			for key, value := range message.Content.Flags {
				values[key] = value
			}
			content = values
		}
		return &renderedAnnouncementMail{From: message.From, To: message.To, ReplyTo: message.ReplyTo, Subject: message.Subject, Template: message.Template, Content: content}, err
	}
}
