package emailoutbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	email "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
)

// IntentTransactions binds a render or delivery record to the intent's scope.
type IntentTransactions interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinSystem(context.Context, func(context.Context) error) error
}

// Transport renders mail snapshots, sends them, and records actual attempts.
type Transport struct {
	Mailer       email.Mailer
	MailIdentity email.ReplyToResolver
	Tenants      IntentTransactions
	Logger       *slog.Logger
}

func (p Transport) Send(ctx context.Context, intent *Intent, renderer Renderer, recipient email.Email) (email.Receipt, error) {
	var message *email.Message
	err := p.withinIntent(ctx, intent, func(tenantCtx context.Context) error {
		var renderErr error
		message, renderErr = renderer.Render(tenantCtx, intent)
		if renderErr == nil {
			p.applyReplyTo(tenantCtx, intent, message)
		}
		return renderErr
	})
	if err != nil {
		return email.Receipt{}, err
	}
	if message != nil {
		message.DeliveryToken = intent.DeliveryToken
		message.Security = intent.Security
		message.UseFallback = intent.UseFallback
	}
	receipt, sendErr := sendRenderedEmail(ctx, p.Mailer, recipient, message)
	if errors.Is(sendErr, email.ErrThrottled) && time.Since(intent.CreatedAt) < ThrottleWindow {
		sendErr = fmt.Errorf("%w: %w", ErrSendThrottled, sendErr)
	}
	if reporter, ok := renderer.(interface {
		RecordDelivery(context.Context, *Intent, error) error
	}); ok {
		// A completed transport attempt must remain observable when its deadline
		// or the caller's cancellation ended the sending context.
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := p.withinIntent(recordCtx, intent, func(recordCtx context.Context) error {
			return reporter.RecordDelivery(recordCtx, intent, sendErr)
		}); err != nil {
			p.Logger.Error("record email delivery failed",
				slog.Int64("intent_id", intent.ID),
				slog.String("error", err.Error()))
		}
	}
	if sendErr != nil {
		return email.Receipt{}, fmt.Errorf("delivery provider: send email: %w", sendErr)
	}
	return receipt, nil
}

func sendRenderedEmail(ctx context.Context, mailer email.Mailer, recipient email.Email, message *email.Message) (email.Receipt, error) {
	if message == nil {
		return email.Receipt{}, errors.New("renderer returned no email message")
	}
	message.To = email.NewEmail(recipient.Name, recipient.Address)
	if receiptMailer, ok := mailer.(email.ReceiptMailer); ok {
		return receiptMailer.SendWithReceipt(ctx, *message)
	}
	contextMailer, ok := mailer.(email.ContextMailer)
	if ok {
		return email.Receipt{}, contextMailer.SendContext(ctx, *message)
	}
	if err := ctx.Err(); err != nil {
		return email.Receipt{}, err
	}
	return email.Receipt{}, mailer.Send(*message)
}

func (p *Transport) applyReplyTo(ctx context.Context, intent *Intent, message *email.Message) {
	if intent.TenantID == 0 || p.MailIdentity == nil || message == nil || message.ReplyTo.Address != "" {
		return
	}
	identity, err := p.MailIdentity.ResolveReplyTo(ctx, intent.TenantID)
	if err != nil {
		p.Logger.Warn("delivery: reply-to lookup failed",
			slog.Int64("intent_id", intent.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if !identity.IsZero() {
		message.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	}
}

// A persisted NULL school denotes system identity mail, not an unscoped
// school operation. Tenant steps call the required-school API directly.
func (p Transport) withinIntent(ctx context.Context, intent *Intent, fn func(context.Context) error) error {
	if intent.TenantID == 0 {
		return p.Tenants.WithinSystem(ctx, fn)
	}
	return p.Tenants.WithinTenant(ctx, intent.TenantID, fn)
}
