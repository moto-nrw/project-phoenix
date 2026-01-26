package audit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Table constants for setting changes
const (
	tableSettingChanges      = "audit.setting_changes"
	tableSettingChangesAlias = `audit.setting_changes AS "setting_change"`
)

// SettingChangeRepository implements audit.SettingChangeRepository
type SettingChangeRepository struct {
	db *bun.DB
}

// NewSettingChangeRepository creates a new SettingChangeRepository
func NewSettingChangeRepository(db *bun.DB) audit.SettingChangeRepository {
	return &SettingChangeRepository{db: db}
}

// Create inserts a new setting change record
func (r *SettingChangeRepository) Create(ctx context.Context, change *audit.SettingChange) error {
	if err := change.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(change).
		ModelTableExpr(tableSettingChanges).
		Returning("id").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create setting change",
			Err: err,
		}
	}

	return nil
}

// FindByID retrieves a setting change by ID
func (r *SettingChangeRepository) FindByID(ctx context.Context, id int64) (*audit.SettingChange, error) {
	change := new(audit.SettingChange)
	err := r.db.NewSelect().
		Model(change).
		ModelTableExpr(tableSettingChangesAlias).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find setting change by id",
			Err: err,
		}
	}

	return change, nil
}

// FindByScope retrieves setting changes for a specific scope
func (r *SettingChangeRepository) FindByScope(
	ctx context.Context,
	scopeType string,
	scopeID *int64,
	limit int,
) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias).
		Where("scope_type = ?", scopeType)

	if scopeID == nil {
		query = query.Where("scope_id IS NULL")
	} else {
		query = query.Where("scope_id = ?", *scopeID)
	}

	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find setting changes by scope",
			Err: err,
		}
	}

	return changes, nil
}

// FindByKey retrieves setting changes for a specific key
func (r *SettingChangeRepository) FindByKey(ctx context.Context, settingKey string, limit int) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias).
		Where("setting_key = ?", settingKey).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find setting changes by key",
			Err: err,
		}
	}

	return changes, nil
}

// FindByKeyAndScope retrieves setting changes for a specific key and scope
func (r *SettingChangeRepository) FindByKeyAndScope(
	ctx context.Context,
	settingKey string,
	scopeType string,
	scopeID *int64,
	limit int,
) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias).
		Where("setting_key = ?", settingKey).
		Where("scope_type = ?", scopeType)

	if scopeID == nil {
		query = query.Where("scope_id IS NULL")
	} else {
		query = query.Where("scope_id = ?", *scopeID)
	}

	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find setting changes by key and scope",
			Err: err,
		}
	}

	return changes, nil
}

// FindByAccount retrieves setting changes made by a specific account
func (r *SettingChangeRepository) FindByAccount(ctx context.Context, accountID int64, limit int) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias).
		Where("account_id = ?", accountID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find setting changes by account",
			Err: err,
		}
	}

	return changes, nil
}

// FindRecent retrieves recent setting changes
func (r *SettingChangeRepository) FindRecent(ctx context.Context, since time.Time, limit int) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias).
		Where("created_at >= ?", since).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find recent setting changes",
			Err: err,
		}
	}

	return changes, nil
}

// List retrieves setting changes with optional filters
func (r *SettingChangeRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.SettingChange, error) {
	var changes []*audit.SettingChange
	query := r.db.NewSelect().
		Model(&changes).
		ModelTableExpr(tableSettingChangesAlias)

	// Apply filters
	if key, ok := filters["setting_key"].(string); ok && key != "" {
		query = query.Where("setting_key = ?", key)
	}

	if scopeType, ok := filters["scope_type"].(string); ok && scopeType != "" {
		query = query.Where("scope_type = ?", scopeType)
	}

	if scopeID, ok := filters["scope_id"].(int64); ok && scopeID > 0 {
		query = query.Where("scope_id = ?", scopeID)
	}

	if changeType, ok := filters["change_type"].(string); ok && changeType != "" {
		query = query.Where("change_type = ?", changeType)
	}

	if accountID, ok := filters["account_id"].(int64); ok && accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}

	if since, ok := filters["since"].(time.Time); ok && !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}

	if until, ok := filters["until"].(time.Time); ok && !until.IsZero() {
		query = query.Where("created_at <= ?", until)
	}

	// Default ordering
	query = query.Order("created_at DESC")

	// Apply limit
	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list setting changes",
			Err: err,
		}
	}

	return changes, nil
}

// CleanupOldChanges removes setting changes older than the specified duration
func (r *SettingChangeRepository) CleanupOldChanges(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.NewDelete().
		Model((*audit.SettingChange)(nil)).
		ModelTableExpr(tableSettingChanges).
		Where("created_at < ?", cutoff).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "cleanup old setting changes",
			Err: err,
		}
	}

	count, _ := result.RowsAffected()
	return int(count), nil
}
