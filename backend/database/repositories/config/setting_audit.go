package config

import (
	"context"
	"fmt"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

const (
	tableSettingAudit = "config.setting_audit"
)

// SettingAuditRepository implements config.SettingAuditRepository.
type SettingAuditRepository struct {
	db *bun.DB
}

// NewSettingAuditRepository creates a new SettingAuditRepository.
func NewSettingAuditRepository(db *bun.DB) config.SettingAuditRepository {
	return &SettingAuditRepository{db: db}
}

// Create appends a new audit entry.
func (r *SettingAuditRepository) Create(ctx context.Context, entry *config.SettingAuditEntry) error {
	if entry == nil {
		return fmt.Errorf("audit entry cannot be nil")
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	_, err := repoBase.GetDB(ctx, r.db).NewInsert().
		Model(entry).
		ModelTableExpr(tableSettingAudit).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("create setting audit entry: %w", err)
	}
	return nil
}
