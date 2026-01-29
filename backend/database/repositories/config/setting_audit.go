package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingAuditRepository implements config.SettingAuditRepository
type SettingAuditRepository struct {
	db *bun.DB
}

// NewSettingAuditRepository creates a new setting audit repository
func NewSettingAuditRepository(db *bun.DB) *SettingAuditRepository {
	return &SettingAuditRepository{db: db}
}

// Create inserts a new audit entry
func (r *SettingAuditRepository) Create(ctx context.Context, entry *config.SettingAuditEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(entry).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Returning("*").
		Exec(ctx)
	return err
}

// FindByDefinitionID retrieves audit entries for a definition
func (r *SettingAuditRepository) FindByDefinitionID(ctx context.Context, defID int64, limit int) ([]*config.SettingAuditEntry, error) {
	var entries []*config.SettingAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Where(`"setting_audit_entry".definition_id = ?`, defID).
		Order("changed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindBySettingKey retrieves audit entries for a setting key
func (r *SettingAuditRepository) FindBySettingKey(ctx context.Context, key string, limit int) ([]*config.SettingAuditEntry, error) {
	var entries []*config.SettingAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Where(`"setting_audit_entry".setting_key = ?`, key).
		Order("changed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindByScope retrieves audit entries for a specific scope
func (r *SettingAuditRepository) FindByScope(ctx context.Context, scopeType config.Scope, scopeID *int64, limit int) ([]*config.SettingAuditEntry, error) {
	var entries []*config.SettingAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Where(`"setting_audit_entry".scope_type = ?`, scopeType).
		Order("changed_at DESC")

	if scopeID == nil {
		query = query.Where(`"setting_audit_entry".scope_id IS NULL`)
	} else {
		query = query.Where(`"setting_audit_entry".scope_id = ?`, *scopeID)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindByAccountID retrieves audit entries by who made the change
func (r *SettingAuditRepository) FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*config.SettingAuditEntry, error) {
	var entries []*config.SettingAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Where(`"setting_audit_entry".changed_by_account_id = ?`, accountID).
		Order("changed_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindRecent retrieves the most recent audit entries
func (r *SettingAuditRepository) FindRecent(ctx context.Context, limit int) ([]*config.SettingAuditEntry, error) {
	var entries []*config.SettingAuditEntry
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Order("changed_at DESC")

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

// CountByDefinitionID returns the count of audit entries for a definition
func (r *SettingAuditRepository) CountByDefinitionID(ctx context.Context, defID int64) (int64, error) {
	count, err := r.db.NewSelect().
		Model((*config.SettingAuditEntry)(nil)).
		ModelTableExpr(`config.setting_audit_log AS "setting_audit_entry"`).
		Where(`"setting_audit_entry".definition_id = ?`, defID).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}
