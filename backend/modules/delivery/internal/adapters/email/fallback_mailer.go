package email

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	mailcontract "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
)

// fallbackPrimaryTimeout leaves the fallback time inside the worker's provider
// timeout when the provider hangs instead of answering.
const fallbackPrimaryTimeout = 12 * time.Second

// fallbackMailer gives security mail a second way out. Lettermint can be down
// or its IPs blocklisted (23.06.2026, about 11 hours at Spamhaus), and someone
// is waiting for a sign-in code. All other mail stays on the provider, so it
// keeps its retries and never leaves through the small mailbox.
type fallbackMailer struct {
	primary        mailcontract.ReceiptMailer
	fallback       mailcontract.ReceiptMailer
	logger         *slog.Logger
	primaryTimeout time.Duration
}

func newFallbackMailer(primary, fallback mailcontract.ReceiptMailer, logger *slog.Logger) *fallbackMailer {
	return &fallbackMailer{primary: primary, fallback: fallback, logger: logger, primaryTimeout: fallbackPrimaryTimeout}
}

func (m *fallbackMailer) Send(message mailcontract.Message) error {
	return m.SendContext(context.Background(), message)
}

func (m *fallbackMailer) SendContext(ctx context.Context, message mailcontract.Message) error {
	_, err := m.SendWithReceipt(ctx, message)
	return err
}

// SendWithReceipt tries the provider first unless it already gave up on the
// mail. An acceptance whose answer got lost can send a security mail twice,
// which is the better failure for someone waiting to sign in.
func (m *fallbackMailer) SendWithReceipt(ctx context.Context, message mailcontract.Message) (mailcontract.Receipt, error) {
	if !message.Security {
		return m.primary.SendWithReceipt(ctx, message)
	}
	if message.UseFallback {
		return m.sendOverFallback(ctx, message, "provider_reported_failure")
	}
	attemptCtx, cancel := context.WithTimeout(ctx, m.primaryTimeout)
	defer cancel()
	receipt, err := m.primary.SendWithReceipt(attemptCtx, message)
	if err == nil || ctx.Err() != nil {
		return receipt, err
	}
	receipt, fallbackErr := m.sendOverFallback(ctx, message, "provider_send_failed")
	if fallbackErr != nil {
		return mailcontract.Receipt{}, fmt.Errorf("security fallback: %w (provider: %v)", fallbackErr, err)
	}
	return receipt, nil
}

func (m *fallbackMailer) sendOverFallback(ctx context.Context, message mailcontract.Message, reason string) (mailcontract.Receipt, error) {
	m.logger.Warn("security mail sent over fallback transport",
		slog.String("template", message.Template),
		slog.String("reason", reason))
	receipt, err := m.fallback.SendWithReceipt(ctx, message)
	receipt.UsedFallback = err == nil
	return receipt, err
}
