package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const tableExprAlias = `platform.email_outbox AS "email_outbox"`

// EmailOutboxRepository implements platform.EmailOutboxRepository with bun.
type EmailOutboxRepository struct {
	db *bun.DB
}

// NewEmailOutboxRepository wires a new repository.
func NewEmailOutboxRepository(db *bun.DB) platform.EmailOutboxRepository {
	return &EmailOutboxRepository{db: db}
}

// Create inserts a new outbox row. Tenant ID is auto-populated from the
// transaction context if not already set.
func (r *EmailOutboxRepository) Create(ctx context.Context, row *platform.EmailOutbox) error {
	if err := row.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	base.EnsureTenantID(ctx, row)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(tableExprAlias).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create email outbox row: %w", err)
	}
	return nil
}

// FindByID retrieves a single row by primary key. Tenant-scoped.
func (r *EmailOutboxRepository) FindByID(ctx context.Context, id int64) (*platform.EmailOutbox, error) {
	row := new(platform.EmailOutbox)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(tableExprAlias).
		Where(`"email_outbox".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("email outbox row %d not found", id)
		}
		return nil, fmt.Errorf("failed to find email outbox row: %w", err)
	}
	return row, nil
}

// ClaimDuePending atomically reserves up to `limit` rows. The CTE pattern
// + FOR UPDATE SKIP LOCKED is the standard Postgres recipe for at-least-
// once worker queues. Callers must run as phoenix_admin so the
// cross-tenant SELECT isn't filtered by the RLS policy.
//
// We deliberately don't filter by tenant — the worker is platform-level.
// Each returned row carries its own tenant_id and the worker uses it to
// switch into the correct tenant transaction for the rendering step.
func (r *EmailOutboxRepository) ClaimDuePending(ctx context.Context, limit int, now time.Time) ([]*platform.EmailOutbox, error) {
	if limit <= 0 {
		limit = 25
	}

	var rows []*platform.EmailOutbox
	query := `
		WITH due AS (
			SELECT id, next_retry_at
			FROM platform.email_outbox
			WHERE status = 'pending'
			  AND next_retry_at <= ?
			ORDER BY next_retry_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT ?
		), updated AS (
			UPDATE platform.email_outbox AS o
			SET status = 'sending'
			FROM due
			WHERE o.id = due.id
			RETURNING o.*
		)
		SELECT updated.*
		FROM updated
		ORDER BY updated.next_retry_at, updated.id
	`
	// Use base.GetDB so the query runs inside the caller's tx (which
	// holds SET LOCAL ROLE phoenix_admin). Bypassing it would grab a
	// fresh phoenix_auth connection from the pool and fail with
	// "permission denied" — phoenix_auth has no direct grants on
	// platform.email_outbox.
	err := base.GetDB(ctx, r.db).NewRaw(query, now, limit).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to claim due pending rows: %w", err)
	}
	return rows, nil
}

// MarkSent transitions a claimed row to 'sent'.
func (r *EmailOutboxRepository) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("status = ?", platform.EmailOutboxStatusSent).
		Set("sent_at = ?", sentAt).
		Set("last_error = NULL").
		Where(`"email_outbox".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark email sent: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("email outbox row %d not found", id)
	}
	return nil
}

// MarkRetry pushes a claimed row back to pending with delayed next_retry_at.
func (r *EmailOutboxRepository) MarkRetry(ctx context.Context, id int64, attempts int, lastErr string, nextRetryAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("status = ?", platform.EmailOutboxStatusPending).
		Set("attempts = ?", attempts).
		Set("last_error = ?", lastErr).
		Set("next_retry_at = ?", nextRetryAt).
		Where(`"email_outbox".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark retry: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("email outbox row %d not found", id)
	}
	return nil
}

// MarkFailed transitions a claimed row to 'failed' (terminal).
func (r *EmailOutboxRepository) MarkFailed(ctx context.Context, id int64, attempts int, lastErr string) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("status = ?", platform.EmailOutboxStatusFailed).
		Set("attempts = ?", attempts).
		Set("last_error = ?", lastErr).
		Where(`"email_outbox".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("email outbox row %d not found", id)
	}
	return nil
}

// FindByRelatedEntity returns all outbox rows linked to a feature's
// related entity. Tenant-scoped.
func (r *EmailOutboxRepository) FindByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) ([]*platform.EmailOutbox, error) {
	var rows []*platform.EmailOutbox
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAlias).
		Where(`"email_outbox".related_entity_type = ?`, relatedType).
		Where(`"email_outbox".related_entity_id = ?`, relatedID).
		OrderExpr(`"email_outbox".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find by related entity: %w", err)
	}
	return rows, nil
}
