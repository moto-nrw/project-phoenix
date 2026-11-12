package application

import (
	"context"
	"encoding/json"

	"github.com/moto-ogs/project-phoenix/modules/communication"
)

var ErrMessageSendConflict = communication.ErrMessageSendConflict

func MessageSendID(ctx context.Context) string { return communication.MessageSendID(ctx) }

// HasMessageAttachments reports whether the send names uploaded files, so a
// message without text is still a message (#6685).
func HasMessageAttachments(ctx context.Context) bool { return communication.HasMessageAttachments(ctx) }

// MessagePreview is the text a conversation list shows for a message.
func MessagePreview(body string, attachments json.RawMessage) string {
	return communication.MessagePreview(body, attachments)
}
