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

// ListPendingAccountWideWipeAccountIDs returns accounts whose tenant request
// recorded a pending cross-school wipe that the after-commit step may have
// missed.
func (r *AuthEventRepository) ListPendingAccountWideWipeAccountIDs(ctx context.Context, since time.Time) ([]int64, error) {
	var ids []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr("DISTINCT account_id").
		TableExpr("audit.auth_events").
		Where("event_type = ?", audit.EventTypeTokenRevoked).
		Where("created_at >= ?", since).
		Where(`metadata @> ?`, `{"pending_account_wide_wipe":true}`).
		Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
