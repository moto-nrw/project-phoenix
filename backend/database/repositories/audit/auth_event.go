package audit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// SQL clause constants
const (
	whereAccountIDEquals = "account_id = ?"
	whereCreatedAtGTE    = "created_at >= ?"
	whereSuccessEquals   = "success = ?"
)

// AuthEventRepository implements the audit.AuthEventRepository interface
type AuthEventRepository struct {
	db *bun.DB
}

// NewAuthEventRepository creates a new auth event repository
func NewAuthEventRepository(db *bun.DB) *AuthEventRepository {
	return &AuthEventRepository{
		db: db,
	}
}

// Create creates a new auth event
func (r *AuthEventRepository) Create(ctx context.Context, event *audit.AuthEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, event)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr("audit.auth_events").
		Exec(ctx)

	return err
}

// FindByID finds an auth event by ID
func (r *AuthEventRepository) FindByID(ctx context.Context, id interface{}) (*audit.AuthEvent, error) {
	var event audit.AuthEvent
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&event).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("auth event not found")
		}
		return nil, err
	}

	return &event, nil
}

// FindByAccountID returns auth events for a specific account
func (r *AuthEventRepository) FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(whereAccountIDEquals, accountID).
		Order(orderByCreatedAtDesc)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// List returns all auth events matching the given filters
func (r *AuthEventRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Order(orderByCreatedAtDesc)

	// Apply filters
	for key, value := range filters {
		switch key {
		case "account_id":
			query = query.Where(whereAccountIDEquals, value)
		case "event_type":
			query = query.Where("event_type = ?", value)
		case "success":
			query = query.Where(whereSuccessEquals, value)
		case "since":
			if t, ok := value.(time.Time); ok {
				query = query.Where(whereCreatedAtGTE, t)
			}
		case "until":
			if t, ok := value.(time.Time); ok {
				query = query.Where("created_at <= ?", t)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// ExistsByAccountEventAndMetadata reports whether the account already has an
// event of eventType whose metadata matches every given key/value.
func (r *AuthEventRepository) ExistsByAccountEventAndMetadata(ctx context.Context, accountID int64, eventType string, metadata map[string]string) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*audit.AuthEvent)(nil)).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".`+whereAccountIDEquals, accountID).
		Where(`"auth_event".event_type = ?`, eventType)
	for key, value := range metadata {
		query = query.Where(`"auth_event".metadata->>? = ?`, key, value)
	}
	return query.Exists(ctx)
}

// ListPendingAccountWideWipes returns the newest pending wipe per account.
// A zero since value lists every still-pending wipe so a failed after-commit
// revoke is retried for as long as the flag remains.
func (r *AuthEventRepository) ListPendingAccountWideWipes(ctx context.Context, since time.Time) ([]audit.PendingAccountWideWipe, error) {
	var rows []audit.PendingAccountWideWipe
	query := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr("DISTINCT ON (account_id) account_id").
		ColumnExpr("COALESCE(metadata->>'reason', '') AS reason").
		ColumnExpr("created_at").
		TableExpr("audit.auth_events").
		Where("event_type = ?", audit.EventTypeTokenRevoked).
		Where(`metadata @> ?`, `{"pending_account_wide_wipe":true}`)
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	err := query.
		OrderExpr("account_id ASC, created_at DESC").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ClaimPendingAccountWideWipes clears pending wipe flags for the account and
// returns the rows that were still pending. Zero rows means another writer
// already claimed or cleared them.
func (r *AuthEventRepository) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]audit.PendingAccountWideWipe, error) {
	var rows []audit.PendingAccountWideWipe
	err := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr("audit.auth_events").
		Set(`metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{pending_account_wide_wipe}', 'false')`).
		Where("account_id = ?", accountID).
		Where("event_type = ?", audit.EventTypeTokenRevoked).
		Where(`metadata @> ?`, `{"pending_account_wide_wipe":true}`).
		Returning("account_id, COALESCE(metadata->>'reason', '') AS reason, created_at").
		Scan(ctx, &rows)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rows, nil
}

// MarkAccountWideWipeCompleted clears pending wipe flags so a later
// reactivation is not selected for another account-wide delete.
func (r *AuthEventRepository) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	_, err := r.ClaimPendingAccountWideWipes(ctx, accountID)
	return err
}
