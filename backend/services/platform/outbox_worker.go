package platform

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// defaultBackoff is the exponential schedule applied to retryable
// failures. Once attempts >= MaxAttempts the row goes to 'failed'.
// Indexed by attempts (0 = first retry after the initial failure).
var defaultBackoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

// pickBackoff returns the delay for the given attempts count, capping
// at the largest configured value when attempts exceeds the schedule
// length.
func pickBackoff(attempts int) time.Duration {
	if attempts <= 0 {
		return defaultBackoff[0]
	}
	if attempts >= len(defaultBackoff) {
		return defaultBackoff[len(defaultBackoff)-1]
	}
	return defaultBackoff[attempts]
}

// OutboxWorker drains platform.email_outbox. Lives at the platform
// level because it iterates rows across all tenants and dispatches
// emails using a process-global mailer. The worker calls Renderer
// implementations (registered in TemplateRegistry) to turn each row
// into an email.Message.
//
// Concurrency: ClaimDuePending uses FOR UPDATE SKIP LOCKED, so
// running multiple workers in parallel is safe. We currently run one
// per process — see scheduler.go for the polling loop.
type OutboxWorker struct {
	repo            platformModels.EmailOutboxRepository
	registry        *TemplateRegistry
	mailer          email.Mailer
	maxAttempts     int
	messageIDDomain string
	logger          *slog.Logger
	db              *bun.DB
}

// OutboxWorkerConfig is the dep-injection bundle for NewOutboxWorker.
// MaxAttempts is the per-row retry budget; rows that fail more than
// this go to status='failed'. DB is needed so the worker can wrap
// pickup in a phoenix_admin transaction (cross-tenant) and per-row
// renders in the matching tenant transaction.
type OutboxWorkerConfig struct {
	Repo            platformModels.EmailOutboxRepository
	Registry        *TemplateRegistry
	Mailer          email.Mailer
	MaxAttempts     int
	MessageIDDomain string
	Logger          *slog.Logger
	DB              *bun.DB
}

// NewOutboxWorker constructs a worker. A nil logger falls back to
// slog.Default(). MaxAttempts <= 0 falls back to len(defaultBackoff).
func NewOutboxWorker(cfg OutboxWorkerConfig) *OutboxWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = len(defaultBackoff)
	}
	return &OutboxWorker{
		repo:            cfg.Repo,
		registry:        cfg.Registry,
		mailer:          cfg.Mailer,
		maxAttempts:     maxAttempts,
		messageIDDomain: strings.TrimSpace(cfg.MessageIDDomain),
		logger:          logger,
		db:              cfg.DB,
	}
}

// SetMaxAttempts allows the scheduler to push a fresh value from the
// `enrollment.outbox_max_attempts` setting on each tick without
// rebuilding the worker.
func (w *OutboxWorker) SetMaxAttempts(n int) {
	if n > 0 {
		w.maxAttempts = n
	}
}

// RunOnce drains up to `batchSize` rows. Returns the number of rows
// the worker attempted to process (sent + retried + failed).
//
// The flow:
//  1. ClaimDuePending under phoenix_admin (cross-tenant SELECT,
//     bypasses RLS on email_outbox).
//  2. For each claimed row, switch into the row's tenant context and
//     call the registered Renderer.
//  3. Reserve any API-provider ID, then persist it with the generated
//     Message-ID in a short phoenix_admin transaction. This commit must happen
//     before transport submission so an immediate webhook can correlate.
//  4. Re-lock the row (LockSending) and Mailer.Send the rendered message
//     inside a second phoenix_admin transaction, so a concurrent cancellation
//     cannot slip between the final probe and send.
//  5. Mark the row sent / retry / failed depending on outcome and
//     attempt count.
//
// Errors during step 1 are logged and returned. Errors per row are
// caught and surfaced as MarkRetry/MarkFailed; one bad row never
// stalls the rest of the batch.
func (w *OutboxWorker) RunOnce(ctx context.Context, batchSize int) (int, error) {
	if w.repo == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing repo)")
	}
	if w.registry == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing registry)")
	}
	if w.mailer == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing mailer)")
	}
	if w.db == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing db)")
	}
	if w.messageIDDomain == "" {
		return 0, fmt.Errorf("outbox worker not wired (missing message id domain)")
	}

	now := time.Now()
	var rows []*platformModels.EmailOutbox

	// Phase 1: claim due pending rows under phoenix_admin so the
	// cross-tenant SELECT isn't filtered by RLS.
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		var claimErr error
		rows, claimErr = w.repo.ClaimDuePending(adminCtx, batchSize, now)
		return claimErr
	}); err != nil {
		return 0, fmt.Errorf("claim due pending: %w", err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	processed := 0
	for _, row := range rows {
		w.processRow(ctx, row)
		processed++
	}
	return processed, nil
}

// processRow handles one claimed row. Per-row failures are logged
// here, never bubbled up. MarkSent/MarkRetry/MarkFailed run under
// phoenix_admin for the same RLS-bypass reason as the pickup query.
func (w *OutboxWorker) processRow(ctx context.Context, row *platformModels.EmailOutbox) {
	renderer, err := w.registry.Lookup(row.Kind)
	if err != nil {
		// Unknown kind — terminal. No backoff, just fail immediately.
		w.markFailedAdmin(ctx, row, fmt.Sprintf("no renderer registered for kind %q", row.Kind))
		return
	}

	tenantCtx := tenant.WithTenantID(ctx, row.GetTenantID())
	msg, renderErr := renderer.Render(tenantCtx, row)
	if renderErr != nil {
		w.recordFailure(ctx, row, fmt.Sprintf("render: %v", renderErr))
		return
	}

	// Correlation identifier for provider delivery events. Minted per send
	// attempt (the column is UNIQUE, so a retry after a partial failure gets a
	// fresh value and cannot collide). The local part carries tenant and
	// outbox id as a last-resort fallback when a relay rewrites the header —
	// the ingest path re-verifies both against the stored row before trusting
	// it, because this string travels through hostile territory.
	correlationToken := uuid.Must(uuid.NewV4()).String()
	messageID := fmt.Sprintf("ob.%d.%d.%s@%s", row.GetTenantID(), row.ID, correlationToken, w.messageIDDomain)
	msg.MessageID = messageID
	preparedSend, prepareErr := w.prepareSend(*msg)
	if prepareErr != nil {
		w.recordFailure(ctx, row, fmt.Sprintf("prepare send: %v", prepareErr))
		return
	}
	dispatchAttempt := &platformModels.EmailDispatchAttempt{
		OutboxID:          row.ID,
		AttemptNumber:     row.Attempts + 1,
		CorrelationToken:  correlationToken,
		MessageID:         messageID,
		ProviderMessageID: preparedSend.providerMessageID,
	}
	dispatchAttempt.SetTenantID(row.GetTenantID())

	var cancelled bool
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		stillClaimed, lockErr := w.repo.LockSending(adminCtx, row.ID)
		if lockErr != nil {
			return lockErr
		}
		if !stillClaimed {
			cancelled = true
			return nil
		}
		if createErr := w.repo.CreateDispatchAttempt(adminCtx, dispatchAttempt); createErr != nil {
			return createErr
		}
		return w.repo.SetDispatchIdentifiers(adminCtx, row.ID, messageID, preparedSend.providerMessageID)
	}); err != nil {
		w.recordFailure(ctx, row, fmt.Sprintf("persist dispatch identifiers: %v", err))
		return
	}
	if cancelled {
		w.logCancelled(row)
		return
	}

	// Send and MarkSent share a second admin transaction that re-locks the
	// claimed row. Features cancel queued emails by deleting their outbox rows;
	// the delete either commits before this probe (send is skipped) or blocks
	// until this transaction commits. The Message-ID remains visible because
	// its transaction above already committed.
	var (
		sendFailure string
		submitted   bool
		sentAt      time.Time
	)
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		stillClaimed, lockErr := w.repo.LockSending(adminCtx, row.ID)
		if lockErr != nil {
			return lockErr
		}
		if !stillClaimed {
			cancelled = true
			return nil
		}
		if sendErr := preparedSend.send(); sendErr != nil {
			sendFailure = fmt.Sprintf("send: %v", sendErr)
			return nil
		}
		submitted = true
		sentAt = time.Now()
		return w.repo.MarkSent(adminCtx, row.ID, sentAt)
	}); err != nil {
		if !submitted {
			w.logger.Error("outbox: send transaction failed",
				slog.Int64("outbox_id", row.ID),
				slog.String("kind", row.Kind),
				slog.String("error", err.Error()))
			return
		}
		if !w.recoverSubmitted(ctx, row, sentAt, err) {
			return
		}
	}
	if cancelled {
		w.logCancelled(row)
		return
	}
	if sendFailure != "" {
		w.recordFailure(ctx, row, sendFailure)
		return
	}
	w.logger.Info("outbox: email sent",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("tenant_id", row.GetTenantID()),
		slog.String("kind", row.Kind))
}

func (w *OutboxWorker) logCancelled(row *platformModels.EmailOutbox) {
	w.logger.Info("outbox: send skipped, row cancelled after claim",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("tenant_id", row.GetTenantID()),
		slog.String("kind", row.Kind))
}

type preparedOutboxSend struct {
	providerMessageID *string
	send              func() error
}

// prepareSend reserves an API provider's correlation ID before submission.
// Plain SMTP has no provider ID and only needs the stamped RFC Message-ID.
func (w *OutboxWorker) prepareSend(msg email.Message) (*preparedOutboxSend, error) {
	if reporter, ok := w.mailer.(email.MessageIDReporter); ok {
		providerID, err := reporter.PrepareProviderMessageID(msg)
		if err != nil {
			return nil, err
		}
		providerID = strings.TrimSpace(providerID)
		if providerID == "" {
			return nil, fmt.Errorf("provider message id is empty")
		}
		return &preparedOutboxSend{
			providerMessageID: &providerID,
			send: func() error {
				return reporter.SendWithID(msg, providerID)
			},
		}, nil
	}
	return &preparedOutboxSend{send: func() error { return w.mailer.Send(msg) }}, nil
}

// recoverSubmitted completes the terminal state without calling the transport
// again. Once Send has succeeded, retrying the mail would risk a duplicate.
func (w *OutboxWorker) recoverSubmitted(ctx context.Context, row *platformModels.EmailOutbox, sentAt time.Time, cause error) bool {
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		return w.repo.MarkSent(adminCtx, row.ID, sentAt)
	}); err != nil {
		w.logger.Error("outbox: recover submitted email failed",
			slog.Int64("outbox_id", row.ID),
			slog.String("kind", row.Kind),
			slog.String("transaction_error", cause.Error()),
			slog.String("recovery_error", err.Error()))
		return false
	}
	w.logger.Warn("outbox: recovered submitted email status",
		slog.Int64("outbox_id", row.ID),
		slog.String("kind", row.Kind),
		slog.String("transaction_error", cause.Error()))
	return true
}

// recordFailure increments attempts and decides retry vs terminal.
func (w *OutboxWorker) recordFailure(ctx context.Context, row *platformModels.EmailOutbox, errMsg string) {
	attempts := row.Attempts + 1
	if attempts >= w.maxAttempts {
		w.markFailedAdmin(ctx, row, errMsg)
		return
	}
	nextRetry := time.Now().Add(pickBackoff(attempts))
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		return w.repo.MarkRetry(adminCtx, row.ID, attempts, errMsg, nextRetry)
	}); err != nil {
		w.logger.Error("outbox: mark retry failed",
			slog.Int64("outbox_id", row.ID),
			slog.String("error", err.Error()))
		return
	}
	w.logger.Warn("outbox: delivery retry scheduled",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("tenant_id", row.GetTenantID()),
		slog.String("kind", row.Kind),
		slog.Int("attempts", attempts),
		slog.Time("next_retry_at", nextRetry),
		slog.String("error", errMsg))
}

func (w *OutboxWorker) markFailedAdmin(ctx context.Context, row *platformModels.EmailOutbox, errMsg string) {
	attempts := row.Attempts + 1
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		return w.repo.MarkFailed(adminCtx, row.ID, attempts, errMsg)
	}); err != nil {
		w.logger.Error("outbox: mark failed failed",
			slog.Int64("outbox_id", row.ID),
			slog.String("error", err.Error()))
		return
	}
	w.logger.Error("outbox: delivery permanently failed",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("tenant_id", row.GetTenantID()),
		slog.String("kind", row.Kind),
		slog.Int("attempts", attempts),
		slog.String("error", errMsg))
}
