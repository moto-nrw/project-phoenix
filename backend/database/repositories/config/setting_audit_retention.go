package config

import (
	"context"
	"fmt"
	"time"
)

// SettingAuditRetention deletes expired setting changes of one school
// (#4044). The tenant role holds DELETE on config.setting_audit; the explicit
// tenant filter keeps a runtime without row-level security to the school as
// well.
type SettingAuditRetention struct {
	runtime Runtime
}

// NewSettingAuditRetention builds the retention over the caller's runtime.
func NewSettingAuditRetention(runtime Runtime) *SettingAuditRetention {
	return &SettingAuditRetention{runtime: runtime}
}

// DeleteChangedBefore removes the school's setting changes made before the
// instant and reports how many went.
func (r *SettingAuditRetention) DeleteChangedBefore(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	if tenantID <= 0 {
		return 0, fmt.Errorf("delete expired setting changes: tenant is required")
	}
	result, err := r.runtime.DB(ctx).NewDelete().
		TableExpr(tableSettingAudit).
		Where("tenant_id = ?", tenantID).
		Where("changed_at < ?", before).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired setting changes: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired setting changes: %w", err)
	}
	return int(rows), nil
}
