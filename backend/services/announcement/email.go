package announcement

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Outbox payload keys for the parent_announcement e-mail.
const (
	emailPayloadRecipient  = "recipient_email"
	emailPayloadFirstName  = "first_name"
	emailPayloadLastName   = "last_name"
	emailPayloadTitle      = "title"
	emailPayloadBody       = "body"
	emailPayloadSchoolName = "school_name"
	emailPayloadPortalURL  = "portal_url"
)

// EmailConfig is the static config for the announcement e-mail renderer.
type EmailConfig struct {
	DefaultFrom email.Email
}

// NewAnnouncementRenderer renders a parent_announcement outbox row into an
// e-mail. Registered in the email template registry (kind
// platform.EmailKindParentAnnouncement) and drained by the outbox worker.
func NewAnnouncementRenderer(cfg EmailConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[emailPayloadRecipient].(string)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		title, _ := row.Payload[emailPayloadTitle].(string)
		body, _ := row.Payload[emailPayloadBody].(string)
		schoolName, _ := row.Payload[emailPayloadSchoolName].(string)
		first, _ := row.Payload[emailPayloadFirstName].(string)
		last, _ := row.Payload[emailPayloadLastName].(string)
		portalURL, _ := row.Payload[emailPayloadPortalURL].(string)

		subject := title
		if schoolName != "" {
			subject = fmt.Sprintf("%s: %s", schoolName, title)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "announcement-published.html",
			Content: map[string]any{
				"Title":             title,
				"Body":              body,
				"SchoolName":        schoolName,
				"GuardianFirstName": first,
				"GuardianLastName":  last,
				"PortalURL":         portalURL,
			},
		}, nil
	}
}
