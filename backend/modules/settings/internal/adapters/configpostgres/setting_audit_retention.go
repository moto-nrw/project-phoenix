package config

import (
	"context"
	"fmt"
)

// deleteExpiredSettingAuditSQL calls the fixed retention function of #6844:
// the tenant role holds no DELETE on config.setting_audit, and the function
// deletes only the calling school's changes older than three years.
const deleteExpiredSettingAuditSQL = `SELECT config.delete_expired_setting_audit(?)`

// SettingAuditRetention deletes expired setting changes of one school
// (#4044).
type SettingAuditRetention struct {
	runtime Runtime
}

// NewSettingAuditRetention builds the retention over the caller's runtime.
func NewSettingAuditRetention(runtime Runtime) *SettingAuditRetention {
	return &SettingAuditRetention{runtime: runtime}
}

// deleteExpired removes the school's setting changes older than the fixed
// window and reports how many went.
func (r *SettingAuditRetention) deleteExpired(ctx context.Context, tenantID int64) (int, error) {
	if tenantID <= 0 {
		return 0, fmt.Errorf("delete expired setting changes: tenant is required")
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(deleteExpiredSettingAuditSQL, tenantID).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("delete expired setting changes: %w", err)
	}
	return int(deleted), nil
}
