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
	ErrEmailDelayOverdue    = errors.New("delayed mails passed the delay limit without a final event")
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
// and answers 503 without a secret. On plain SMTP nothing is reconciled, and
// lifting a block only clears the school's list.
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
	checked, missed, silent, overdue int
	failures                         []error
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
		run.failures = append(run.failures, fmt.Errorf("read the open mails: %w", err))
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

// reconcileMail reports whether the run goes on. The rate limit ends it and is
// the run's failure; the next run continues with the same mails, since they stay
// unmarked.
func (r *EmailReconciliation) reconcileMail(ctx context.Context, mail domain.ProviderMail, now time.Time, run *reconcileRun) bool {
	events, err := r.api.MessageEvents(ctx, mail.ProviderMessageID)
	unknown := errors.Is(err, domain.ErrProviderNotFound)
	switch {
	case errors.Is(err, domain.ErrProviderRateLimited):
		run.failures = append(run.failures, fmt.Errorf("read the events of mail %d: %w", mail.ID, err))
		return false
	case unknown:
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
	if err := r.store.MarkEmailDeliveryChecked(ctx, mail.ID, now); err != nil {
		run.failures = append(run.failures, fmt.Errorf("mark mail %d checked: %w", mail.ID, err))
		return ctx.Err() == nil
	}
	run.checked++
	if state == "" && turnedSilent(mail, now) {
		r.reportSilence(mail, unknown, run)
	}
	if state == domain.EmailDeliveryDelayed && turnedOverdue(mail, now) {
		run.overdue++
	}
	return ctx.Err() == nil
}

// reportSilence separates a mail the provider does not know from a mail it knows
// without an outcome: only the second speaks of missing events.
func (r *EmailReconciliation) reportSilence(mail domain.ProviderMail, unknown bool, run *reconcileRun) {
	if !unknown {
		run.silent++
		return
	}
	r.logger.Warn("the provider does not know an accepted mail", slog.Int64("mail_id", mail.ID))
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

// turnedSilent reports whether the mail passed the silence limit since it was
// last asked about. An open mail is asked about for up to 28 days, and it counts
// only the first time: the check that follows is already past the limit.
func turnedSilent(mail domain.ProviderMail, now time.Time) bool {
	if now.Sub(mail.SentAt) <= emailEventSilence {
		return false
	}
	return mail.CheckedAt == nil || mail.CheckedAt.Sub(mail.SentAt) <= emailEventSilence
}

// turnedOverdue reports whether the mail's delay passed the limit since it was
// last asked about, so one delay raises one alarm.
func turnedOverdue(mail domain.ProviderMail, now time.Time) bool {
	if mail.StateAt == nil || now.Sub(*mail.StateAt) <= deliveryContract.EmailDelayLimit {
		return false
	}
	return mail.CheckedAt == nil || mail.CheckedAt.Sub(*mail.StateAt) <= deliveryContract.EmailDelayLimit
}

func (r *EmailReconciliation) alarms(run *reconcileRun) error {
	failures := run.failures
	if run.missed > 0 {
		failures = append(failures, fmt.Errorf("%w: %d", ErrEmailEventsMissed, run.missed))
	}
	if run.silent > 0 {
		failures = append(failures, fmt.Errorf("%w: %d", ErrEmailEventsSilent, run.silent))
	}
	if run.overdue > 0 {
		failures = append(failures, fmt.Errorf("%w: %d", ErrEmailDelayOverdue, run.overdue))
	}
	if err := errors.Join(failures...); err != nil {
		r.logger.Error("email delivery alarm",
			slog.Int("checked", run.checked),
			slog.Int("missed", run.missed),
			slog.Int("silent", run.silent),
			slog.Int("overdue", run.overdue),
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

// The provider is asked about one address at a time. A list asks once per distinct
// address and only for mails that ran into a block, within these limits; whatever
// they leave out counts as unreadable and is not offered.
const (
	providerBlockReads  = 10
	providerBlockBudget = 3 * time.Second
)

// ProviderEmailBlocks reads the provider's current block of the addresses. Without
// a provider no such block exists. It answers for every address and has no error:
// an address it could not read is EmailProviderBlockUnreadable. The caller only asks
// about addresses its own mail ran into, so this says nothing about anyone else's.
func (r *EmailReconciliation) ProviderEmailBlocks(ctx context.Context, addresses []string) map[string]deliveryContract.EmailProviderBlock {
	blocks := make(map[string]deliveryContract.EmailProviderBlock, len(addresses))
	ctx, cancel := context.WithTimeout(ctx, providerBlockBudget)
	defer cancel()
	reads, stop := 0, false
	for _, address := range addresses {
		key := deliveryContract.NormalizedEmailAddress(address)
		if _, known := blocks[key]; known || key == "" {
			continue
		}
		if r.api == nil {
			blocks[key] = deliveryContract.EmailProviderBlockNone
			continue
		}
		blocks[key] = deliveryContract.EmailProviderBlockUnreadable
		if stop || reads >= providerBlockReads {
			continue
		}
		reads++
		active, err := r.api.SuppressionActive(ctx, key)
		switch {
		case errors.Is(err, domain.ErrProviderNotFound):
			blocks[key] = deliveryContract.EmailProviderBlockNone
		case err != nil:
			stop = errors.Is(err, domain.ErrProviderRateLimited) || ctx.Err() != nil
			r.logger.Warn("the provider's block could not be read", slog.String("error", err.Error()))
		case active:
			blocks[key] = deliveryContract.EmailProviderBlockActive
		default:
			blocks[key] = deliveryContract.EmailProviderBlockNone
		}
	}
	return blocks
}

// LiftEmailSuppression frees an address someone at the school checked: first at
// the provider, whose hard bounce block applies to the whole team and never
// expires, then on the school's list. Any error means the address stays blocked.
//
// Only an entry on the school's own list opens this path. The list holds the
// school's own bounces and complaints, so a school cannot lift, or even learn of, a
// block another school caused, whatever mail it sent itself. An answer of the
// provider that names a complaint stays generic: that complaint may be another
// school's.
func (r *EmailReconciliation) LiftEmailSuppression(ctx context.Context, tenantID int64, address string) error {
	address = strings.ToLower(strings.TrimSpace(address))
	reason, listed, err := r.store.EmailSuppressionReason(ctx, tenantID, address)
	if err != nil {
		return err
	}
	if !listed {
		return deliveryContract.ErrEmailSuppressionNotFound
	}
	if reason == domain.EmailSuppressionComplained {
		return deliveryContract.ErrEmailSuppressionNeedsReview
	}
	if r.api != nil {
		if err := r.api.LiftSuppression(ctx, address); err != nil {
			if errors.Is(err, domain.ErrSuppressionNeedsReview) {
				return deliveryContract.ErrEmailSuppressionStaysAtProvider
			}
			return liftFailure(err)
		}
	}
	return liftFailure(r.store.DeleteEmailSuppression(ctx, tenantID, address))
}

func liftFailure(err error) error {
	switch {
	case errors.Is(err, domain.ErrSuppressionNeedsReview):
		return deliveryContract.ErrEmailSuppressionNeedsReview
	case errors.Is(err, domain.ErrSuppressionStaysAtProvider):
		return deliveryContract.ErrEmailSuppressionStaysAtProvider
	case err != nil:
		return fmt.Errorf("lift the block: %w", err)
	}
	return nil
}
