package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Payload keys used by the guardian-invitation outbox row. The service
// fills these on Enqueue; the renderer reads them on dispatch.
const (
	guardianPayloadRecipientEmail = "recipient_email"
	guardianPayloadFirstName      = "first_name"
	guardianPayloadLastName       = "last_name"
	guardianPayloadInvitationURL  = "invitation_url"
	guardianPayloadLogoURL        = "logo_url"
	guardianPayloadSchoolName     = "school_name"
	guardianPayloadExpiryHours    = "expiry_hours"
)

// GuardianInvitationRendererConfig is the closure-state for the
// renderer. Captures the process-static defaults (default from address)
// so the renderer doesn't have to reach back into the service.
type GuardianInvitationRendererConfig struct {
	DefaultFrom email.Email
}

// NewGuardianInvitationRenderer returns a Renderer-compatible function
// that turns a guardian_invitation outbox row into an email.Message.
// Register the result with platform.TemplateRegistry at startup.
func NewGuardianInvitationRenderer(cfg GuardianInvitationRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[guardianPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("guardian invitation payload missing recipient_email")
		}
		invitationURL, _ := row.Payload[guardianPayloadInvitationURL].(string)
		if invitationURL == "" {
			return nil, fmt.Errorf("guardian invitation payload missing invitation_url")
		}

		firstName, _ := row.Payload[guardianPayloadFirstName].(string)
		lastName, _ := row.Payload[guardianPayloadLastName].(string)
		logoURL, _ := row.Payload[guardianPayloadLogoURL].(string)
		schoolName, _ := row.Payload[guardianPayloadSchoolName].(string)
		expiryHours, _ := payloadIntField(row.Payload, guardianPayloadExpiryHours)
		if expiryHours <= 0 {
			expiryHours = 48
		}

		subject := "Einladung zum Eltern-Portal"
		if schoolName != "" {
			subject = fmt.Sprintf("Einladung zum Eltern-Portal – %s", schoolName)
		}

		msg := &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "guardian-invitation.html",
			Content: map[string]any{
				"InvitationURL": invitationURL,
				"FirstName":     firstName,
				"LastName":      lastName,
				"ExpiryHours":   expiryHours,
				"LogoURL":       logoURL,
				"SchoolName":    schoolName,
			},
		}
		return msg, nil
	}
}

// payloadIntField extracts an integer from a JSON payload that may
// arrive as float64 (Postgres jsonb roundtrip) or int. Returns ok=false
// when the field is missing or not numeric.
func payloadIntField(payload map[string]any, key string) (int, bool) {
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}
