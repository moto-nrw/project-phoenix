package platform

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Outbox payload keys for the delivery-failure notice. One row per recipient,
// matching how every other outbox mail addresses exactly one person.
const (
	DeliveryNoticePayloadRecipient      = "recipient_email"
	DeliveryNoticePayloadFailedTo       = "failed_to"
	DeliveryNoticePayloadFailedStatus   = "failed_status"
	DeliveryNoticePayloadFailedKind     = "failed_kind"
	DeliveryNoticePayloadReason         = "reason"
	DeliveryNoticePayloadReasonCode     = "reason_code"
	DeliveryNoticePayloadOccurredAtText = "occurred_at_text"
)

// EmailConfig is the static config for the delivery notice renderer.
type EmailConfig struct {
	DefaultFrom email.Email
}

// deliveryStatusLabels are the German words the notice uses. Kept next to the
// renderer rather than in the model so the model holds no presentation.
var deliveryStatusLabels = map[string]string{
	platformModels.EmailDeliveryStatusBounced:    "endgültig nicht zustellbar",
	platformModels.EmailDeliveryStatusComplained: "vom Empfänger als Spam gemeldet",
	platformModels.EmailDeliveryStatusSuppressed: "vom Mailanbieter blockiert",
}

var deliveryKindLabels = map[string]string{
	platformModels.EmailKindGuardianInvitation: "Einladung zum Eltern-Portal",
	platformModels.EmailKindParentAnnouncement: "Elternmitteilung",
}

// NewDeliveryFailureRenderer renders the ops notice for a terminally failed
// email. Registered under EmailKindDeliveryFailureNotice.
func NewDeliveryFailureRenderer(cfg EmailConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[DeliveryNoticePayloadRecipient].(string)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		failedTo, _ := row.Payload[DeliveryNoticePayloadFailedTo].(string)
		failedStatus, _ := row.Payload[DeliveryNoticePayloadFailedStatus].(string)
		failedKind, _ := row.Payload[DeliveryNoticePayloadFailedKind].(string)
		reason, _ := row.Payload[DeliveryNoticePayloadReason].(string)
		reasonCode, _ := row.Payload[DeliveryNoticePayloadReasonCode].(string)
		occurredAt, _ := row.Payload[DeliveryNoticePayloadOccurredAtText].(string)

		statusLabel := deliveryStatusLabels[failedStatus]
		if statusLabel == "" {
			statusLabel = "nicht zustellbar"
		}
		kindLabel := deliveryKindLabels[failedKind]
		if kindLabel == "" {
			kindLabel = "E-Mail"
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  "E-Mail konnte nicht zugestellt werden",
			Template: "delivery-failure-notice.html",
			Content: map[string]any{
				"FailedTo":    failedTo,
				"StatusLabel": statusLabel,
				"KindLabel":   kindLabel,
				"Reason":      reason,
				"ReasonCode":  reasonCode,
				"OccurredAt":  occurredAt,
			},
		}, nil
	}
}
