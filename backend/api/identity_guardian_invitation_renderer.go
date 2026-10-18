package api

import (
	"context"
	"fmt"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
		mail, recipient, err := identityaccess.GuardianInvitationMail(payload)
		if err != nil {
			return nil, err
		}
		return identityMessage(cfg.DefaultFrom, email.NewEmail("", recipient), mail), nil
	}
}

// NewGuardianWelcomeRenderer returns the renderer of the queued welcome
// mail (#3534): a greeting, the help article for parents and the Elterninfo.
func NewGuardianWelcomeRenderer(cfg GuardianInvitationRendererConfig) func(context.Context, map[string]any) (*email.Message, error) {
	return func(ctx context.Context, payload map[string]any) (*email.Message, error) {
		if err := guardianWelcomeReleased(ctx, cfg, payload); err != nil {
			return nil, err
		}
		mail, recipient, err := identityaccess.GuardianWelcomeMail(payload)
		if err != nil {
			return nil, err
		}
		return identityMessage(cfg.DefaultFrom, email.NewEmail("", recipient), mail), nil
	}
}

// guardianWelcomeReleased asks the owner whether the welcome may render now,
// given the delivery state of the mail that granted the access.
func guardianWelcomeReleased(ctx context.Context, cfg GuardianInvitationRendererConfig, payload map[string]any) error {
	precedingOutboxID, err := identityaccess.GuardianWelcomePrecedingOutboxID(payload)
	if err != nil {
		return err
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

// identityMessage addresses an Identity & Access mail.
func identityMessage(from, to email.Email, mail identityaccess.Mail) *email.Message {
	return &email.Message{From: from, To: to, Subject: mail.Subject, Template: mail.Template, Content: mail.Content}
}
