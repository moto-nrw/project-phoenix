package calendar

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// EmailConfig is the static config for the appointment e-mail renderer.
type EmailConfig struct {
	DefaultFrom email.Email
}

// NewAppointmentRenderer renders any appointment outbox row (published /
// updated / cancelled / reminder) into an e-mail. One renderer handles all four
// kinds — they share a template and differ only in the intro copy + subject.
// Registered in the email template registry for each appointment kind and
// drained by the outbox worker.
func NewAppointmentRenderer(cfg EmailConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[apptPayloadRecipient].(string)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		title, _ := row.Payload[apptPayloadTitle].(string)
		when, _ := row.Payload[apptPayloadWhen].(string)
		location, _ := row.Payload[apptPayloadLocation].(string)
		schoolName, _ := row.Payload[apptPayloadSchoolName].(string)
		first, _ := row.Payload[apptPayloadFirstName].(string)
		last, _ := row.Payload[apptPayloadLastName].(string)
		portalURL, _ := row.Payload[apptPayloadPortalURL].(string)
		logoURL, _ := row.Payload[apptPayloadLogoURL].(string)
		motoLogoURL, _ := row.Payload[apptPayloadMotoLogoURL].(string)

		kicker, subjectVerb, intro := appointmentCopy(row.Kind)
		subject := fmt.Sprintf("%s: %s", subjectVerb, title)
		if schoolName != "" {
			subject = fmt.Sprintf("%s – %s: %s", schoolName, subjectVerb, title)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "appointment-notification.html",
			Content: map[string]any{
				"Title":             title,
				"BrandKicker":       kicker,
				"IntroText":         intro,
				"WhenText":          when,
				"Location":          location,
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

// appointmentCopy returns the German header kicker, subject verb and intro
// sentence for each appointment e-mail kind.
func appointmentCopy(kind string) (kicker, subjectVerb, intro string) {
	switch kind {
	case platformModels.EmailKindAppointmentUpdated:
		return "Termin geändert", "Termin geändert", "ein Termin wurde aktualisiert."
	case platformModels.EmailKindAppointmentCancelled:
		return "Termin abgesagt", "Termin abgesagt", "ein Termin wurde abgesagt."
	case platformModels.EmailKindAppointmentReminder:
		return "Terminerinnerung", "Erinnerung", "wir möchten Sie an einen bevorstehenden Termin erinnern."
	default:
		return "Neuer Termin", "Neuer Termin", "es gibt einen neuen Termin für Sie."
	}
}
