package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

// WorkerRecords is Delivery's durable worker state. Each transition commits
// separately: claiming and fencing must commit before a provider can send.
type WorkerRecords interface {
	ports.Store
	ports.RetentionStore
	targetedEmailStore
	targetedPushStore
	DeleteExpiredSystemEmails(context.Context, time.Time, int) (int, error)
}

// WorkerCommands owns the cross-school worker units of work. Row adapters
// receive existing transactions and cannot start or commit one themselves.
type WorkerCommands struct {
	WorkerRecords
	withinAdmin func(context.Context, func(context.Context) error) error
}

func NewWorkerCommands(records WorkerRecords, withinAdmin func(context.Context, func(context.Context) error) error) *WorkerCommands {
	if records == nil || withinAdmin == nil {
		panic("delivery: worker records and unit of work are required")
	}
	return &WorkerCommands{WorkerRecords: records, withinAdmin: withinAdmin}
}

func (c *WorkerCommands) ClaimPush(ctx context.Context, tenantID, id int64, now, expires time.Time) (result0 domain.Intent, result1 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, result1, err = c.WorkerRecords.ClaimPush(txCtx, tenantID, id, now, expires)
		return err
	})
	return
}

func (c *WorkerCommands) Claim(ctx context.Context, transport domain.Transport, limit int, now, leaseExpiresAt time.Time) (result0 []domain.Intent, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.Claim(txCtx, transport, limit, now, leaseExpiresAt)
		return err
	})
	return
}

func (c *WorkerCommands) DeadLetterExpiredDispatches(ctx context.Context, transport domain.Transport, now time.Time) (result0 int64, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.DeadLetterExpiredDispatches(txCtx, transport, now)
		return err
	})
	return
}

func (c *WorkerCommands) StartDelivery(ctx context.Context, transport domain.Transport, id int64, token string, leaseExpiresAt time.Time) (result0 string, result1 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, result1, err = c.WorkerRecords.StartDelivery(txCtx, transport, id, token, leaseExpiresAt)
		return err
	})
	return
}

func (c *WorkerCommands) FinalizeSent(ctx context.Context, transport domain.Transport, id int64, token string, providerResult json.RawMessage, sentAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.FinalizeSent(txCtx, transport, id, token, providerResult, sentAt)
		return err
	})
	return
}

func (c *WorkerCommands) FinalizeCancelled(ctx context.Context, transport domain.Transport, id int64, token, reason string, cancelledAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.FinalizeCancelled(txCtx, transport, id, token, reason, cancelledAt)
		return err
	})
	return
}

func (c *WorkerCommands) FinalizeDeferred(ctx context.Context, transport domain.Transport, id int64, token, reason string, nextRetryAt time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.FinalizeDeferred(txCtx, transport, id, token, reason, nextRetryAt)
		return err
	})
	return
}

func (c *WorkerCommands) FinalizeFailure(ctx context.Context, transport domain.Transport, id int64, token string, attempts int, lastError string, nextRetryAt time.Time, maxAttempts int) (result0 domain.FinalizeResult, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.FinalizeFailure(txCtx, transport, id, token, attempts, lastError, nextRetryAt, maxAttempts)
		return err
	})
	return
}

func (c *WorkerCommands) Backlog(ctx context.Context) (result0 int, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.Backlog(txCtx)
		return err
	})
	return
}

func (c *WorkerCommands) OldestPendingAge(ctx context.Context, now time.Time) (result0 time.Duration, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.OldestPendingAge(txCtx, now)
		return err
	})
	return
}

func (c *WorkerCommands) ClaimEmail(ctx context.Context, tenantID, id int64, now, expires time.Time) (result0 domain.Intent, result1 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, result1, err = c.WorkerRecords.ClaimEmail(txCtx, tenantID, id, now, expires)
		return err
	})
	return
}

func (c *WorkerCommands) RecordEmailRetry(ctx context.Context, id int64, token string, attempts int, now time.Time) (result0 bool, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.RecordEmailRetry(txCtx, id, token, attempts, now)
		return err
	})
	return
}

func (c *WorkerCommands) DeleteExpiredSystemEmails(ctx context.Context, before time.Time, limit int) (result0 int, err error) {
	err = c.withinAdmin(ctx, func(txCtx context.Context) error {
		result0, err = c.WorkerRecords.DeleteExpiredSystemEmails(txCtx, before, limit)
		return err
	})
	return
}
