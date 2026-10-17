package api

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// ParentGuardianPolicy binds the workflow to Security's canonical guardian rules.
type ParentGuardianPolicy struct{}

func (ParentGuardianPolicy) IsFullRole(role string) bool {
	return securityruntime.IsFullGuardianRole(role)
}
func (ParentGuardianPolicy) PermissionGranted(value any) bool {
	return securityruntime.GuardianPermissionGranted(value)
}
func (ParentGuardianPolicy) DefaultRole(relationship string, primary, emergency, pickup bool) (string, []string) {
	role := securityruntime.DefaultStudentGuardianRole(relationship, primary, emergency, pickup)
	return securityruntime.StudentGuardianRolePreset(role)
}

// ParentPortalLocales binds the workflow's language catalog to Delivery.
func ParentPortalLocales() devices.LocaleCatalog { return devices.LocaleCatalog{} }

type parentStudentUpdates func(int64, string) error

func (update parentStudentUpdates) StudentUpdated(tenantID int64, source string) error {
	return update(tenantID, source)
}

// BindParentStudentUpdates preserves the optional live-event trigger and error.
func BindParentStudentUpdates(broadcaster realtime.Broadcaster) interface{ StudentUpdated(int64, string) error } {
	if broadcaster == nil {
		return nil
	}
	return parentStudentUpdates(func(tenantID int64, source string) error {
		event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		return broadcaster.BroadcastToTenant(tenantID, event)
	})
}
