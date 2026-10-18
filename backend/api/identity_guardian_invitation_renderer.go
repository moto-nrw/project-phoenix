package api

import (
	"context"
	"fmt"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
		queued := storedGuardianMail(payload)
		mail, err := identityaccess.GuardianInvitationMail(queued)
		if err != nil {
			return nil, err
		}
		return identityMessage(cfg.DefaultFrom, email.NewEmail("", queued.Recipient), mail), nil
	}
}

// NewGuardianWelcomeRenderer returns the renderer of the queued welcome
// mail (#3534): a greeting, the help article for parents and the Elterninfo.
func NewGuardianWelcomeRenderer(cfg GuardianInvitationRendererConfig) func(context.Context, map[string]any) (*email.Message, error) {
	return func(ctx context.Context, payload map[string]any) (*email.Message, error) {
		if err := guardianWelcomeReleased(ctx, cfg, payload); err != nil {
			return nil, err
		}
		queued := storedGuardianMail(payload)
		mail, err := identityaccess.GuardianWelcomeMail(queued)
		if err != nil {
			return nil, err
		}
		return identityMessage(cfg.DefaultFrom, email.NewEmail("", queued.Recipient), mail), nil
	}
}

// guardianWelcomeReleased asks the owner whether the welcome may render now,
// given the delivery state of the mail that granted the access.
func guardianWelcomeReleased(ctx context.Context, cfg GuardianInvitationRendererConfig, payload map[string]any) error {
	precedingOutboxID, ok := payloadInt64Field(payload, guardianPayloadPrecedingOutboxID)
	if !ok || precedingOutboxID <= 0 {
		return fmt.Errorf("guardian welcome payload missing %s", guardianPayloadPrecedingOutboxID)
	}
	if cfg.PrecedingEmailStatus == nil {
		return fmt.Errorf("guardian welcome preceding email status is not configured")
	}
	sent, terminal, err := cfg.PrecedingEmailStatus(ctx, precedingOutboxID)
	if err != nil {
		return fmt.Errorf("look up guardian access email delivery: %w", err)
	}
	switch identityaccess.GuardianWelcomeRelease(sent, terminal) {
	case identityaccess.MailCancelled:
		return fmt.Errorf("%w: guardian access email cannot be delivered", deliveryCompose.ErrRenderCancelled)
	case identityaccess.MailDeferred:
		return fmt.Errorf("%w: guardian access email has not been accepted", deliveryCompose.ErrRenderDeferred)
	default:
		return nil
	}
}

// storedGuardianMail reads a guardian mail back from its payload; a field of
// the wrong type reads as absent.
func storedGuardianMail(payload map[string]any) identityaccess.GuardianMail {
	text := func(key string) string {
		value, _ := payload[key].(string)
		return value
	}
	expiryHours, _ := payloadIntField(payload, guardianPayloadExpiryHours)
	existingAccount, _ := payload[guardianPayloadExistingAccount].(bool)
	precedingOutboxID, _ := payloadInt64Field(payload, guardianPayloadPrecedingOutboxID)
	return identityaccess.GuardianMail{
		Recipient: text(guardianPayloadRecipientEmail), FirstName: text(guardianPayloadFirstName),
		LastName: text(guardianPayloadLastName), InvitationURL: text(guardianPayloadInvitationURL),
		LogoURL: text(guardianPayloadLogoURL), SchoolName: text(guardianPayloadSchoolName),
		ExpiryHours: expiryHours, ExistingAccount: existingAccount,
		HelpURL: text(guardianPayloadHelpURL), ParentInfoURL: text(guardianPayloadParentInfoURL),
		PrecedingOutboxID: precedingOutboxID,
	}
}

// identityMessage addresses an Identity & Access mail.
func identityMessage(from, to email.Email, mail identityaccess.Mail) *email.Message {
	return &email.Message{From: from, To: to, Subject: mail.Subject, Template: mail.Template, Content: mail.Content}
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
