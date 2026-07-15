package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ScheduleTenantSettingsBroadcast queues a tenant_settings_changed SSE event
// to fire after the OUTERMOST tenant tx commits. Cross-origin tabs (e.g.
// operator changing a tenant setting from operator.<domain>) only receive
// change notifications via SSE; same-origin tabs are covered by the
// BroadcastChannel ping the frontend fires. Source carries the setting key so
// the receiving tab can scope future selective invalidations and so log
// review can attribute writes. Shared by the tenant and operator settings
// resources so both writers fan out the same event shape.
func ScheduleTenantSettingsBroadcast(ctx context.Context, broadcaster realtime.Broadcaster, tenantID int64, key string) {
	if broadcaster == nil || tenantID == 0 {
		return
	}
	tenant.RegisterAfterCommit(ctx, func() {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{
			Source: &key,
		})
		_ = broadcaster.BroadcastToTenant(tenantID, event)
	})
}
