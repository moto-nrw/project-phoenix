package compose

import (
	context "context"
	time "time"

	settings "github.com/moto-nrw/project-phoenix/modules/settings"
	configRepo "github.com/moto-nrw/project-phoenix/modules/settings/internal/adapters/configpostgres"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

type retainedAuditRetention struct{ Now func() time.Time }

func (r retainedAuditRetention) DeleteExpiredSettingChanges(ctx context.Context) (int, int, error) {
	return configRepo.DeleteComposedSettingAudit(ctx, r.Now, settings.SettingAuditRetentionYears, tenant.TenantFromContext, tenant.TransactionFromContext)
}
