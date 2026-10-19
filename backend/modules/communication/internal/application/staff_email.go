package application

import (
	"context"
	"fmt"
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

	// emailPayloadBody / emailPayloadAckRequired are the Elternbrief additions
	// (#2384). A letter is the ONE case where the body travels by e-mail: the
	// school has decided this content may leave the portal, and a parent who
	// never opens the app must still be able to read it. Both are optional, so
	// outbox rows queued before this change still render exactly as before.
	emailPayloadBody        = "body"
	emailPayloadAckRequired = "ack_required"

	// emailPayloadHasAttachment says that the announcement carries files
	// (#2890). It carries a flag, never the file: an attachment stays in the
	// portal, where the audience check decides who may open it. A mail
	// attachment would reach every address the school entered — including, for
	// an Elternbrief to "alle Bezugspersonen", people with no portal access at
	// all — and would thereby undo exactly the control "nur mit Portalzugang"
	// exists to provide.
	emailPayloadHasAttachment = "has_attachment"

	defaultEmailKicker = "Elternmitteilung"
	letterEmailKicker  = "Elternbrief"
	// declarationEmailKicker marks an Erklärung (#3430); its mail carries
	// only title and link, like every Mitteilung.
	declarationEmailKicker = "Einverständnis"
)

// EmailConfig is the static config for the announcement e-mail renderer.
type EmailConfig MailRendererConfig

// NewAnnouncementRenderer renders a parent_announcement outbox row into an
// e-mail. Registered in the email template registry (kind
// platform.EmailKindParentAnnouncement) and drained by the outbox worker.
func NewAnnouncementRenderer(cfg EmailConfig) MailRenderer {
	return NewMailRenderer(func(_ context.Context, row *MailIntent) (*MailMessage, error) {
		recipient := row.Payload.String(emailPayloadRecipient)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		title := row.Payload.String(emailPayloadTitle)
		schoolName := row.Payload.String(emailPayloadSchoolName)
		first := row.Payload.String(emailPayloadFirstName)
		last := row.Payload.String(emailPayloadLastName)
		portalURL := row.Payload.String(emailPayloadPortalURL)
		logoURL := row.Payload.String(emailPayloadLogoURL)
		motoLogoURL := row.Payload.String(emailPayloadMotoLogoURL)

		kicker := row.Payload.String(emailPayloadKicker)
		if kicker == "" {
			kicker = defaultEmailKicker
		}
		intro := row.Payload.String(emailPayloadIntro)
		body := row.Payload.String(emailPayloadBody)
		ackRequired := row.Payload.Bool(emailPayloadAckRequired)

		subject := title
		if schoolName != "" {
			subject = fmt.Sprintf("%s: %s", schoolName, title)
		}

		content := NewMailContent(map[string]string{
			"Title":             title,
			"Intro":             intro,
			"BrandKicker":       kicker,
			"SchoolName":        schoolName,
			"GuardianFirstName": first,
			"GuardianLastName":  last,
			"PortalURL":         portalURL,
			"LogoURL":           logoURL,
			"MotoLogoURL":       motoLogoURL,
		}, map[string]bool{})
		// The body and the acknowledgement notice are added ONLY for an
		// Elternbrief. A Mitteilung mail stays title + link, and the rendered
		// content must not even carry the keys — the mail is the least trusted
		// channel, and that rule is pinned by TestAnnouncementRenderer_TitleAndLinkOnly.
		if body != "" {
			content.Strings["Body"] = body
		}
		if ackRequired {
			content.Flags["AckRequired"] = true
		}
		if hasAttachment := row.Payload.Bool(emailPayloadHasAttachment); hasAttachment {
			content.Flags["HasAttachment"] = true
		}

		return &MailMessage{
			From:     cfg.DefaultFrom,
			To:       NewMailAddress("", recipient),
			Subject:  subject,
			Template: "announcement-published.html",
			Content:  content,
		}, nil
	})
}
