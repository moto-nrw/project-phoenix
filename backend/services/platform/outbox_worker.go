package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	repo        platformModels.EmailOutboxRepository
	registry    *TemplateRegistry
	mailer      email.Mailer
	maxAttempts int
	logger      *slog.Logger
	db          *bun.DB
}

// OutboxWorkerConfig is the dep-injection bundle for NewOutboxWorker.
// MaxAttempts is the per-row retry budget; rows that fail more than
// this go to status='failed'. DB is needed so the worker can wrap
// pickup in a phoenix_admin transaction (cross-tenant) and per-row
// renders in the matching tenant transaction.
type OutboxWorkerConfig struct {
	Repo        platformModels.EmailOutboxRepository
	Registry    *TemplateRegistry
	Mailer      email.Mailer
	MaxAttempts int
	Logger      *slog.Logger
	DB          *bun.DB
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
		repo:        cfg.Repo,
		registry:    cfg.Registry,
		mailer:      cfg.Mailer,
		maxAttempts: maxAttempts,
		logger:      logger,
		db:          cfg.DB,
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
//  3. Persist the generated Message-ID in a short phoenix_admin transaction.
//     This commit must happen before transport submission so an immediate
//     delivery webhook can correlate the message.
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
	messageID := fmt.Sprintf("ob.%d.%d.%s@%s", row.GetTenantID(), row.ID, uuid.Must(uuid.NewV4()).String(), w.messageIDDomain())
	msg.MessageID = messageID

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
		return w.repo.SetDispatchIdentifiers(adminCtx, row.ID, messageID, nil)
	}); err != nil {
		w.recordFailure(ctx, row, fmt.Sprintf("persist message id: %v", err))
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
	var sendFailure string
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		stillClaimed, lockErr := w.repo.LockSending(adminCtx, row.ID)
		if lockErr != nil {
			return lockErr
		}
		if !stillClaimed {
			cancelled = true
			return nil
		}
		providerMessageID, sendErr := w.send(*msg)
		if sendErr != nil {
			sendFailure = fmt.Sprintf("send: %v", sendErr)
			return nil
		}
		if providerMessageID != nil {
			if idErr := w.repo.SetDispatchIdentifiers(adminCtx, row.ID, messageID, providerMessageID); idErr != nil {
				return idErr
			}
		}
		return w.repo.MarkSent(adminCtx, row.ID, time.Now())
	}); err != nil {
		w.logger.Error("outbox: send transaction failed",
			slog.Int64("outbox_id", row.ID),
			slog.String("kind", row.Kind),
			slog.String("error", err.Error()))
		return
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

// send dispatches the message, preferring the MessageIDReporter extension so
// providers that return their own identifier get it stored alongside ours.
// Plain SMTP mailers fall through to Send and report no provider id.
func (w *OutboxWorker) send(msg email.Message) (*string, error) {
	if reporter, ok := w.mailer.(email.MessageIDReporter); ok {
		providerID, err := reporter.SendWithID(msg)
		if err != nil {
			return nil, err
		}
		if providerID != "" {
			return &providerID, nil
		}
		return nil, nil
	}
	if err := w.mailer.Send(msg); err != nil {
		return nil, err
	}
	return nil, nil
}

// messageIDDomain is the right-hand side of the minted Message-ID. It only has
// to be stable and syntactically valid — nothing resolves it — so the sender
// address's domain is the natural choice.
func (w *OutboxWorker) messageIDDomain() string {
	if domain := strings.TrimSpace(os.Getenv("EMAIL_MESSAGE_ID_DOMAIN")); domain != "" {
		return domain
	}
	from := os.Getenv("EMAIL_FROM_ADDRESS")
	if at := strings.LastIndex(from, "@"); at >= 0 && at < len(from)-1 {
		return from[at+1:]
	}
	return "phoenix.local"
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
