package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// ActionAuditRepository implements config.ActionAuditRepository
type ActionAuditRepository struct {
	db *bun.DB
}

// NewActionAuditRepository creates a new action audit repository
func NewActionAuditRepository(db *bun.DB) *ActionAuditRepository {
	return &ActionAuditRepository{db: db}
}

// Create inserts a new audit entry
func (r *ActionAuditRepository) Create(ctx context.Context, entry *config.ActionAuditEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(entry).
		ModelTableExpr(`config.action_audit_log AS "action_audit_entry"`).
		Returning("*").
		Exec(ctx)
	return err
}

// FindByActionKey retrieves audit entries for a specific action
func (r *ActionAuditRepository) FindByActionKey(ctx context.Context, key string, limit int) ([]*config.ActionAuditEntry, error) {
	var entries []*config.ActionAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.action_audit_log AS "action_audit_entry"`).
		Where(`"action_audit_entry".action_key = ?`, key).
		Order("executed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindRecent retrieves the most recent audit entries across all actions
func (r *ActionAuditRepository) FindRecent(ctx context.Context, limit int) ([]*config.ActionAuditEntry, error) {
	var entries []*config.ActionAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.action_audit_log AS "action_audit_entry"`).
		Order("executed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(100) // Default limit
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindByAccountID retrieves audit entries by who executed the action
func (r *ActionAuditRepository) FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*config.ActionAuditEntry, error) {
	var entries []*config.ActionAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.action_audit_log AS "action_audit_entry"`).
		Where(`"action_audit_entry".executed_by_account_id = ?`, accountID).
		Order("executed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// CountByActionKey returns the count of executions for an action
func (r *ActionAuditRepository) CountByActionKey(ctx context.Context, key string) (int64, error) {
	count, err := r.db.NewSelect().
		Model((*config.ActionAuditEntry)(nil)).
		ModelTableExpr(`config.action_audit_log AS "action_audit_entry"`).
		Where(`"action_audit_entry".action_key = ?`, key).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}
