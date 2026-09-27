package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
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
	// guardianPayloadExistingAccount marks the variant for an address that
	// already owns an account: invitation_url is then the parents portal
	// login, and the mail asks for the existing credentials (#3320).
	guardianPayloadExistingAccount = "existing_account"
	// The welcome mail (#3534) carries the help article and the Elterninfo
	// instead of an accept link.
	guardianPayloadHelpURL           = "help_url"
	guardianPayloadParentInfoURL     = "parent_info_url"
	guardianPayloadPrecedingOutboxID = "preceding_outbox_id"
)

// GuardianInvitationRendererConfig is the closure-state for the
// renderer. Captures the process-static defaults (default from address)
// so the renderer doesn't have to reach back into the service.
type GuardianInvitationRendererConfig struct {
	DefaultFrom          email.Email
	PrecedingEmailStatus func(context.Context, int64) (sent bool, terminal bool, err error)
}

// NewGuardianInvitationRenderer returns a function that turns the payload of
// a queued guardian_invitation e-mail into an email.Message. The root
// registers it with the Delivery renderer registry at startup.
func NewGuardianInvitationRenderer(cfg GuardianInvitationRendererConfig) func(context.Context, map[string]any) (*email.Message, error) {
	return func(_ context.Context, payload map[string]any) (*email.Message, error) {
		recipient, _ := payload[guardianPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("guardian invitation payload missing recipient_email")
		}
		invitationURL, _ := payload[guardianPayloadInvitationURL].(string)
		if invitationURL == "" {
			return nil, fmt.Errorf("guardian invitation payload missing invitation_url")
		}

		firstName, _ := payload[guardianPayloadFirstName].(string)
		lastName, _ := payload[guardianPayloadLastName].(string)
		logoURL, _ := payload[guardianPayloadLogoURL].(string)
		schoolName, _ := payload[guardianPayloadSchoolName].(string)
		expiryHours, _ := payloadIntField(payload, guardianPayloadExpiryHours)
		if expiryHours <= 0 {
			expiryHours = 48
		}

		existingAccount, _ := payload[guardianPayloadExistingAccount].(bool)

		subject := "Einladung zum Eltern-Portal"
		if existingAccount {
			subject = "Ihr Zugang zum Eltern-Portal"
		}
		if schoolName != "" {
			subject = fmt.Sprintf("%s – %s", subject, schoolName)
		}

		msg := &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "guardian-invitation.html",
			Content: map[string]any{
				"InvitationURL":   invitationURL,
				"FirstName":       firstName,
				"LastName":        lastName,
				"ExpiryHours":     expiryHours,
				"LogoURL":         logoURL,
				"SchoolName":      schoolName,
				"ExistingAccount": existingAccount,
			},
		}
		return msg, nil
	}
}

// NewGuardianWelcomeRenderer returns the renderer of the queued welcome
// mail (#3534): a greeting, the help article for parents and the Elterninfo.
func NewGuardianWelcomeRenderer(cfg GuardianInvitationRendererConfig) func(context.Context, map[string]any) (*email.Message, error) {
	return func(ctx context.Context, payload map[string]any) (*email.Message, error) {
		precedingOutboxID, ok := payloadInt64Field(payload, guardianPayloadPrecedingOutboxID)
		if !ok || precedingOutboxID <= 0 {
			return nil, fmt.Errorf("guardian welcome payload missing %s", guardianPayloadPrecedingOutboxID)
		}
		if cfg.PrecedingEmailStatus == nil {
			return nil, fmt.Errorf("guardian welcome preceding email status is not configured")
		}
		sent, terminal, err := cfg.PrecedingEmailStatus(ctx, precedingOutboxID)
		if err != nil {
			return nil, fmt.Errorf("look up guardian access email delivery: %w", err)
		}
		if terminal {
			return nil, fmt.Errorf("%w: guardian access email cannot be delivered", emailoutbox.ErrRenderCancelled)
		}
		if !sent {
			return nil, fmt.Errorf("%w: guardian access email has not been accepted", emailoutbox.ErrRenderDeferred)
		}
		recipient, _ := payload[guardianPayloadRecipientEmail].(string)
		if recipient == "" {
			return nil, fmt.Errorf("guardian welcome payload missing recipient_email")
		}
		helpURL, _ := payload[guardianPayloadHelpURL].(string)
		if helpURL == "" {
			return nil, fmt.Errorf("guardian welcome payload missing help_url")
		}
		parentInfoURL, _ := payload[guardianPayloadParentInfoURL].(string)
		firstName, _ := payload[guardianPayloadFirstName].(string)
		lastName, _ := payload[guardianPayloadLastName].(string)
		logoURL, _ := payload[guardianPayloadLogoURL].(string)
		schoolName, _ := payload[guardianPayloadSchoolName].(string)

		subject := "Willkommen bei moto"
		if schoolName != "" {
			subject = fmt.Sprintf("%s – %s", subject, schoolName)
		}
		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "guardian-welcome.html",
			Content: map[string]any{
				"HelpURL":       helpURL,
				"ParentInfoURL": parentInfoURL,
				"FirstName":     firstName,
				"LastName":      lastName,
				"LogoURL":       logoURL,
				"SchoolName":    schoolName,
			},
		}, nil
	}
}

func payloadInt64Field(payload map[string]any, key string) (int64, bool) {
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(x), x == float64(int64(x))
	default:
		return 0, false
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
