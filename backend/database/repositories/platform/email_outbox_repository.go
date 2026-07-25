package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
func NewEmailOutboxRepository(db *bun.DB) platform.EmailOutboxCleanupRepository {
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
		On("CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING").
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

func (r *EmailOutboxRepository) DeleteByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) (int64, error) {
	result, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Where(`"email_outbox".related_entity_type = ?`, relatedType).
		Where(`"email_outbox".related_entity_id = ?`, relatedID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete related email outbox rows: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
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

// LockSending locks a claimed row FOR UPDATE and reports whether it still
// exists with status='sending'. Runs under phoenix_admin inside the worker's
// send transaction — see the interface doc for the cancellation contract.
func (r *EmailOutboxRepository) LockSending(ctx context.Context, id int64) (bool, error) {
	var claimed int
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT 1
		FROM platform.email_outbox
		WHERE id = ? AND status = ?
		FOR UPDATE
	`, id, platform.EmailOutboxStatusSending).Scan(ctx, &claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to lock sending email outbox row: %w", err)
	}
	return true, nil
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

// CancelPendingByRelatedEntity marks every still-pending outbox row for a
// feature's related entity as 'failed' with the given reason, so the worker
// (which only claims 'pending' rows) never sends them. Used when the triggering
// entity is retracted before the async send (e.g. a published parent
// announcement is unpublished or deleted). Rows already claimed by the worker
// ('sending') or terminal ('sent'/'failed') are deliberately left untouched —
// they may already be in flight. Tenant-scoped; runs under the caller's tenant
// tx (phoenix_tenant holds UPDATE on the table). Returns rows affected.
func (r *EmailOutboxRepository) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("status = ?", platform.EmailOutboxStatusFailed).
		Set("last_error = ?", reason).
		Where(`"email_outbox".related_entity_type = ?`, relatedType).
		Where(`"email_outbox".related_entity_id = ?`, relatedID).
		Where(`"email_outbox".status = ?`, platform.EmailOutboxStatusPending).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to cancel pending email rows: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// FindByMessageID resolves the outbox row a provider delivery event refers to.
// Runs under phoenix_admin (cross-tenant); returns (nil, nil) when nothing
// matches, because an event about a message we never sent is a normal outcome
// on a shared provider account and must not be treated as an error.
func (r *EmailOutboxRepository) FindByMessageID(ctx context.Context, messageID string) (*platform.EmailOutbox, error) {
	return r.findByIdentifier(ctx, "message_id", messageID)
}

// FindByProviderMessageID is the same lookup keyed on the transport's own id.
func (r *EmailOutboxRepository) FindByProviderMessageID(ctx context.Context, providerMessageID string) (*platform.EmailOutbox, error) {
	return r.findByIdentifier(ctx, "provider_message_id", providerMessageID)
}

func (r *EmailOutboxRepository) findByIdentifier(ctx context.Context, column, value string) (*platform.EmailOutbox, error) {
	if value == "" {
		return nil, nil
	}
	row := new(platform.EmailOutbox)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(tableExprAlias).
		Where(`"email_outbox".`+column+` = ?`, value).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find email outbox row by %s: %w", column, err)
	}
	return row, nil
}

// SetDispatchIdentifiers stores the correlation identifiers for a message the
// worker is about to hand to the transport.
func (r *EmailOutboxRepository) SetDispatchIdentifiers(ctx context.Context, id int64, messageID string, providerMessageID *string) error {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("message_id = ?", messageID).
		Where(`"email_outbox".id = ?`, id)
	if providerMessageID != nil && *providerMessageID != "" {
		q = q.Set("provider_message_id = ?", *providerMessageID)
	}
	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set dispatch identifiers: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("email outbox row %d not found", id)
	}
	return nil
}

// ApplyDeliveryStatus advances the row's delivery status only when the
// incoming event outranks what is stored. The guard in the WHERE clause is the
// concurrency control: duplicate and out-of-order provider deliveries are
// absorbed here, not by any ordering guarantee from the provider.
//
//	apply ⟺ newRank > storedRank ∨ (newRank == storedRank ∧ newOccurredAt > storedAt)
func (r *EmailOutboxRepository) ApplyDeliveryStatus(ctx context.Context, id int64, t platform.DeliveryTransition) (bool, error) {
	detail := sql.NullString{String: t.Detail, Valid: strings.TrimSpace(t.Detail) != ""}
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.EmailOutbox)(nil)).
		ModelTableExpr(tableExprAlias).
		Set("delivery_status = ?", t.Status).
		Set("delivery_status_rank = ?", t.Rank).
		Set("delivery_status_at = ?", t.OccurredAt).
		Set("delivery_detail = ?", detail).
		Set("updated_at = NOW()").
		Where(`"email_outbox".id = ?`, id).
		Where(`(? > "email_outbox".delivery_status_rank
			OR (? = "email_outbox".delivery_status_rank
				AND ("email_outbox".delivery_status_at IS NULL
					OR ? > "email_outbox".delivery_status_at)))`,
			t.Rank, t.Rank, t.OccurredAt).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to apply delivery status: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
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
