package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
)

const (
	// emailProviderRetention is how long the provider keeps a mail's events.
	emailProviderRetention = 28 * 24 * time.Hour
	// emailWebhookGrace leaves a young mail to the webhook.
	emailWebhookGrace = 30 * time.Minute
	// emailRecheckInterval rotates through the open mails.
	emailRecheckInterval = time.Hour
	// emailEventSilence is how long an accepted mail may go without any event.
	emailEventSilence   = 24 * time.Hour
	emailReconcileBatch = 100
)

// The alarms of a reconciliation. Lettermint gives up on a webhook after 12
// attempts in about 14 hours and disables it after 72 hours of failures; what
// happened in that time reaches moto only through this job.
var (
	ErrEmailWebhookDisabled = errors.New("email webhook is disabled at the provider")
	ErrEmailEventsMissed    = errors.New("the provider reported events the webhook never delivered")
	ErrEmailEventsSilent    = errors.New("accepted mails stayed without any provider event")
)

// EmailReconciliation compares open mails with the provider's own record.
type EmailReconciliation struct {
	store interface {
		ports.EmailReconciliationStore
		ports.EmailDeliveryEventStore
	}
	api      ports.EmailProviderAPI
	fallback bool
	logger   *slog.Logger
	observe  ports.Observer
	now      func() time.Time
}

// NewEmailReconciliation composes the job. Without a provider API (plain SMTP)
// it has nothing to compare and does nothing.
func NewEmailReconciliation(store interface {
	ports.EmailReconciliationStore
	ports.EmailDeliveryEventStore
}, api ports.EmailProviderAPI, fallback bool, logger *slog.Logger, observe ports.Observer) *EmailReconciliation {
	return &EmailReconciliation{store: store, api: api, fallback: fallback, logger: logger, observe: observe, now: time.Now}
}

// NewEmailEvents composes the webhook, the reconciliation and the lifting of a
// block for this deployment's provider. The webhook exists in every deployment
// and answers 503 without a secret; the rest does nothing on plain SMTP.
func NewEmailEvents(config mail.ProviderConfig, store *WorkerCommands, observe ports.Observer) (*EmailWebhooks, *EmailReconciliation) {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "email_events")
	var api ports.EmailProviderAPI
	if config.UsesLettermint() {
		api = newLettermintClient(lettermintClientConfig{TeamToken: config.TeamToken, WebhookID: config.WebhookID})
	}
	fallback := config.UsesLettermint()
	webhooks := NewEmailWebhooks(store, []ports.EmailWebhookProvider{newLettermintWebhook(config.WebhookSecrets)}, fallback, logger, observe)
	return webhooks, NewEmailReconciliation(store, api, fallback, logger, observe)
}

var (
	_ delivery.EmailWebhooks     = (*EmailWebhooks)(nil)
	_ delivery.EmailSuppressions = (*EmailReconciliation)(nil)
)

// reconcileRun counts what one run found.
type reconcileRun struct {
	checked, missed, silent int
	failures                []error
}

// ReconcileEmailDelivery brings the open mails up to the provider's record and returns the alarms.
// Every alarm is an error, so it reaches the job's failure reporting.
func (r *EmailReconciliation) ReconcileEmailDelivery(ctx context.Context) error {
	if r == nil || r.api == nil {
		return nil
	}
	now := r.now()
	run := &reconcileRun{}
	r.checkWebhook(ctx, run)
	mails, err := r.store.OpenProviderMails(ctx, domain.ProviderMailQuery{
		SentAfter: now.Add(-emailProviderRetention), SentBefore: now.Add(-emailWebhookGrace),
		CheckedBefore: now.Add(-emailRecheckInterval), Limit: emailReconcileBatch,
	})
	if err != nil {
		return errors.Join(append(run.failures, err)...)
	}
	for _, mail := range mails {
		if !r.reconcileMail(ctx, mail, now, run) {
			break
		}
	}
	r.observe(domain.Observation{Operation: "email_reconciliation", Count: run.checked, Err: errors.Join(run.failures...)})
	return r.alarms(run)
}

func (r *EmailReconciliation) checkWebhook(ctx context.Context, run *reconcileRun) {
	enabled, err := r.api.WebhookEnabled(ctx)
	switch {
	case err != nil:
		run.failures = append(run.failures, fmt.Errorf("read the webhook state: %w", err))
	case !enabled:
		run.failures = append(run.failures, ErrEmailWebhookDisabled)
	}
}

// reconcileMail reports whether the run goes on. The rate limit ends it for now;
// the next run continues with the same mails, since they stay unmarked.
func (r *EmailReconciliation) reconcileMail(ctx context.Context, mail domain.ProviderMail, now time.Time, run *reconcileRun) bool {
	events, err := r.api.MessageEvents(ctx, mail.ProviderMessageID)
	switch {
	case errors.Is(err, domain.ErrProviderRateLimited):
		return false
	case errors.Is(err, domain.ErrProviderNotFound):
		events = nil
	case err != nil:
		run.failures = append(run.failures, fmt.Errorf("read the events of mail %d: %w", mail.ID, err))
		return ctx.Err() == nil
	}
	state, err := r.recordEvents(ctx, mail, events, run)
	if err != nil {
		run.failures = append(run.failures, fmt.Errorf("record the events of mail %d: %w", mail.ID, err))
		return ctx.Err() == nil
	}
	if state == "" && now.Sub(mail.SentAt) > emailEventSilence {
		run.silent++
	}
	if err := r.store.MarkEmailDeliveryChecked(ctx, mail.ID, now); err != nil {
		run.failures = append(run.failures, fmt.Errorf("mark mail %d checked: %w", mail.ID, err))
	}
	run.checked++
	return ctx.Err() == nil
}

// recordEvents returns the mail's state after the provider's events. A state the
// webhook should have brought counts as missed.
func (r *EmailReconciliation) recordEvents(ctx context.Context, mail domain.ProviderMail, events []domain.EmailDeliveryEvent, run *reconcileRun) (domain.EmailDeliveryState, error) {
	state := mail.State
	for _, event := range events {
		event.Reroute = event.Reroute && r.fallback
		result, err := RecordEmailDeliveryEvent(ctx, r.store, event)
		if err != nil {
			return state, err
		}
		if result.Rerouted || (result.State != "" && result.State != state) {
			run.missed++
			state = event.State
		}
	}
	return state, nil
}

func (r *EmailReconciliation) alarms(run *reconcileRun) error {
	failures := run.failures
	if run.missed > 0 {
		failures = append(failures, fmt.Errorf("%w: %d", ErrEmailEventsMissed, run.missed))
	}
	if run.silent > 0 {
		failures = append(failures, fmt.Errorf("%w: %d", ErrEmailEventsSilent, run.silent))
	}
	if err := errors.Join(failures...); err != nil {
		r.logger.Error("email delivery alarm",
			slog.Int("checked", run.checked),
			slog.Int("missed", run.missed),
			slog.Int("silent", run.silent),
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

// LiftEmailSuppression frees a corrected address: first at the provider, whose
// hard bounce block applies to the whole team and never expires, then on the
// school's list. A complaint's block is never lifted here; it needs a review by
// the provider's support.
func (r *EmailReconciliation) LiftEmailSuppression(ctx context.Context, tenantID int64, address string) error {
	address = strings.TrimSpace(address)
	reason, listed, err := r.store.EmailSuppressionReason(ctx, tenantID, address)
	if err != nil {
		return err
	}
	if listed && reason == domain.EmailDeliveryComplained {
		return deliveryContract.ErrEmailSuppressionNeedsReview
	}
	if r.api != nil {
		if err := r.api.LiftSuppression(ctx, address); err != nil {
			if errors.Is(err, domain.ErrSuppressionNeedsReview) {
				return deliveryContract.ErrEmailSuppressionNeedsReview
			}
			return fmt.Errorf("lift the provider's block: %w", err)
		}
	}
	if err := r.store.DeleteEmailSuppression(ctx, tenantID, address); err != nil {
		if errors.Is(err, domain.ErrSuppressionNeedsReview) {
			return deliveryContract.ErrEmailSuppressionNeedsReview
		}
		return err
	}
	return nil
}
