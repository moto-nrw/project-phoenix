package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	repo          platformModels.EmailOutboxRepository
	registry      *TemplateRegistry
	mailer        email.Mailer
	maxAttempts   int
	logger        *slog.Logger
	db            *bun.DB
	tenantRuntime *tenant.UnitOfWork
}

// SetTenantRuntime wires the transaction runtime used by this cross-tenant worker.
func (w *OutboxWorker) SetTenantRuntime(runtime tenant.UnitOfWork) {
	w.tenantRuntime = &runtime
}

func (w *OutboxWorker) withTenantRuntime(ctx context.Context) context.Context {
	if w.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *w.tenantRuntime)
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
//  3. Re-lock the row (LockSending) and Mailer.Send the rendered
//     message inside one phoenix_admin transaction, so a concurrent
//     cancellation (deletion of the outbox row) cannot slip between
//     claim and send.
//  4. Mark the row sent / retry / failed depending on outcome and
//     attempt count.
//
// Errors during step 1 are logged and returned. Errors per row are
// caught and surfaced as MarkRetry/MarkFailed; one bad row never
// stalls the rest of the batch.
func (w *OutboxWorker) RunOnce(ctx context.Context, batchSize int) (int, error) {
	ctx = w.withTenantRuntime(ctx)
	if w.repo == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing repo)")
	}
	if w.db == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing db)")
	}
	if w.registry == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing registry)")
	}
	if w.mailer == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing mailer)")
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

// Backlog returns the durable platform-wide pending queue depth.
func (w *OutboxWorker) Backlog(ctx context.Context) (int, error) {
	ctx = w.withTenantRuntime(ctx)
	if w.repo == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing repo)")
	}
	if w.db == nil {
		return 0, fmt.Errorf("outbox worker not wired (missing db)")
	}
	var backlog int
	err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		var countErr error
		backlog, countErr = w.repo.CountPending(adminCtx)
		return countErr
	})
	if err != nil {
		return 0, fmt.Errorf("count pending outbox: %w", err)
	}
	return backlog, nil
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
		// A cancelled render is a decision, not a fault: the row may never be sent,
		// so retrying it would only delay the same answer. Retire it now.
		if errors.Is(renderErr, ErrRenderCancelled) {
			w.cancelRow(ctx, row, renderErr.Error())
			return
		}
		w.recordFailure(ctx, row, fmt.Sprintf("render: %v", renderErr))
		return
	}

	// Send and MarkSent share one admin transaction that first re-locks
	// the claimed row (FOR UPDATE). Features cancel queued emails by
	// deleting their outbox rows (e.g. enrollment deletion wipes the rows
	// for a deleted request), and that delete races the committed claim:
	// without the lock the worker could still send a message built from
	// data that is already gone. With it, a concurrent delete either
	// committed first (the probe finds nothing, the send is skipped) or
	// blocks until this transaction commits.
	var cancelled bool
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
		if sendErr := w.mailer.Send(*msg); sendErr != nil {
			sendFailure = fmt.Sprintf("send: %v", sendErr)
			return nil
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
		w.logger.Info("outbox: send skipped, row cancelled after claim",
			slog.Int64("outbox_id", row.ID),
			slog.Int64("tenant_id", row.GetTenantID()),
			slog.String("kind", row.Kind))
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

// cancelRow retires a row its renderer refused to build. It shares the terminal
// 'failed' state with a permanent delivery failure — the same state
// CancelPendingByRelatedEntity uses to retract queued mail — but logs at Info,
// because nothing here is broken.
func (w *OutboxWorker) cancelRow(ctx context.Context, row *platformModels.EmailOutbox, reason string) {
	attempts := row.Attempts + 1
	if err := tenant.WithAdminTx(ctx, w.db, func(adminCtx context.Context, _ bun.Tx) error {
		return w.repo.MarkFailed(adminCtx, row.ID, attempts, reason)
	}); err != nil {
		w.logger.Error("outbox: mark cancelled failed",
			slog.Int64("outbox_id", row.ID),
			slog.String("error", err.Error()))
		return
	}
	w.logger.Info("outbox: send cancelled by renderer",
		slog.Int64("outbox_id", row.ID),
		slog.Int64("tenant_id", row.GetTenantID()),
		slog.String("kind", row.Kind),
		slog.String("reason", reason))
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
