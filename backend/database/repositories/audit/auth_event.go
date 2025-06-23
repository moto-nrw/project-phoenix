package audit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/audit"
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

	_, err := r.db.NewInsert().
		Model(event).
		ModelTableExpr("audit.auth_events").
		Exec(ctx)

	return err
}

// FindByID finds an auth event by ID
func (r *AuthEventRepository) FindByID(ctx context.Context, id interface{}) (*audit.AuthEvent, error) {
	var event audit.AuthEvent
	err := r.db.NewSelect().
		Model(&event).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".id = ?`, id).
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

	query := r.db.NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, accountID).
		Order(`"auth_event".created_at DESC`)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// FindByEventType returns auth events of a specific type since a given time
func (r *AuthEventRepository) FindByEventType(ctx context.Context, eventType string, since time.Time) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	err := r.db.NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".event_type = ?`, eventType).
		Where(`"auth_event".created_at >= ?`, since).
		Order(`"auth_event".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return events, nil
}

// FindFailedAttempts returns failed auth attempts for an account within a time window
func (r *AuthEventRepository) FindFailedAttempts(ctx context.Context, accountID int64, since time.Time) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	err := r.db.NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, accountID).
		Where(`"auth_event".success = ?`, false).
		Where(`"auth_event".created_at >= ?`, since).
		Order(`"auth_event".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return events, nil
}

// CountFailedAttempts counts failed auth attempts for an account within a time window
func (r *AuthEventRepository) CountFailedAttempts(ctx context.Context, accountID int64, since time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*audit.AuthEvent)(nil)).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, accountID).
		Where(`"auth_event".success = ?`, false).
		Where(`"auth_event".created_at >= ?`, since).
		Count(ctx)

	return count, err
}

// CleanupOldEvents removes auth events older than the specified duration
func (r *AuthEventRepository) CleanupOldEvents(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-olderThan)

	result, err := r.db.NewDelete().
		Model((*audit.AuthEvent)(nil)).
		ModelTableExpr("audit.auth_events").
		Where("created_at < ?", cutoffTime).
		Exec(ctx)

	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// List returns all auth events matching the given filters
func (r *AuthEventRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	query := r.db.NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Order(`"auth_event".created_at DESC`)

	// Apply filters
	for key, value := range filters {
		switch key {
		case "account_id":
			query = query.Where(`"auth_event".account_id = ?`, value)
		case "event_type":
			query = query.Where(`"auth_event".event_type = ?`, value)
		case "success":
			query = query.Where(`"auth_event".success = ?`, value)
		case "since":
			if t, ok := value.(time.Time); ok {
				query = query.Where(`"auth_event".created_at >= ?`, t)
			}
		case "until":
			if t, ok := value.(time.Time); ok {
				query = query.Where(`"auth_event".created_at <= ?`, t)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}