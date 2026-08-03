package announcement

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Outbox payload keys for the parent_announcement e-mail. The announcement
// BODY is deliberately not part of the payload: the mail is only a pointer
// (title + portal link) — the content lives in the parent portal.
const (
	emailPayloadRecipient  = "recipient_email"
	emailPayloadFirstName  = "first_name"
	emailPayloadLastName   = "last_name"
	emailPayloadTitle      = "title"
	emailPayloadSchoolName = "school_name"
	emailPayloadPortalURL  = "portal_url"
	// emailPayloadLogoURL / emailPayloadMotoLogoURL carry the same header/footer
	// branding the enrollment mails use, so the parent-announcement mail renders
	// the school logo and the moto logo instead of the plain fallbacks.
	emailPayloadLogoURL     = "logo_url"
	emailPayloadMotoLogoURL = "moto_logo_url"
	// emailPayloadKicker / emailPayloadIntro let a poll (Umfrage) and its reminder
	// reuse this renderer and template while saying what they are. Both are
	// optional: a row without them renders exactly like the pre-poll mail, so
	// outbox rows queued before this change still deliver correctly.
	emailPayloadKicker = "brand_kicker"
	emailPayloadIntro  = "intro"

	defaultEmailKicker = "Elternmitteilung"
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
		schoolName, _ := row.Payload[emailPayloadSchoolName].(string)
		first, _ := row.Payload[emailPayloadFirstName].(string)
		last, _ := row.Payload[emailPayloadLastName].(string)
		portalURL, _ := row.Payload[emailPayloadPortalURL].(string)
		logoURL, _ := row.Payload[emailPayloadLogoURL].(string)
		motoLogoURL, _ := row.Payload[emailPayloadMotoLogoURL].(string)

		kicker, _ := row.Payload[emailPayloadKicker].(string)
		if kicker == "" {
			kicker = defaultEmailKicker
		}
		intro, _ := row.Payload[emailPayloadIntro].(string)

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
				"Intro":             intro,
				"BrandKicker":       kicker,
				"SchoolName":        schoolName,
				"GuardianFirstName": first,
				"GuardianLastName":  last,
				"PortalURL":         portalURL,
				"LogoURL":           logoURL,
				"MotoLogoURL":       motoLogoURL,
			},
		}, nil
	}
}
