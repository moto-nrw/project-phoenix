package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

const (
	tableSettingAudit = "config.setting_audit"
)

// SettingAuditRepository implements config.SettingAuditRepository.
type SettingAuditRepository struct {
	runtime Runtime
}

// NewSettingAuditRepository creates a new SettingAuditRepository.
func NewSettingAuditRepository(runtime Runtime) config.SettingAuditRepository {
	return &SettingAuditRepository{runtime: runtime}
}

// Create appends a new audit entry.
func (r *SettingAuditRepository) Create(ctx context.Context, entry *config.SettingAuditEntry) error {
	if entry == nil {
		return fmt.Errorf("audit entry cannot be nil")
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	_, err := r.runtime.DB(ctx).NewInsert().
		Model(entry).
		ModelTableExpr(tableSettingAudit).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("create setting audit entry: %w", err)
	}
	return nil
}
