package api

import (
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
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

// ParentDeclarationPolicy binds the workflow port to Communication's value policy.
func ParentDeclarationPolicy() parentportal.DeclarationPolicy {
	return BindParentDeclarationPolicy(parentStore.NewPortalDeclarationPolicy())
}

// NewParentGuardianPolicy binds canonical role, relationship and permission
// policies without leaking the directory model into the workflow.
func NewParentGuardianPolicy() parentportal.GuardianPolicy {
	return parentportal.NewGuardianPolicy(ParentGuardianPolicy{}, parentportal.GuardianPolicyCallbacks{
		RelationshipTypeValid: peopledirectory.GuardianRelationshipTypeValid,
		NormalizeRelationship: func(value *parentportal.GuardianRelationship) error {
			row := directoryGuardianRelationship(value)
			err := peopledirectory.NormalizeGuardianRelationship(row)
			value.RelationshipType, value.GuardianRole, value.Permissions = row.RelationshipType, row.GuardianRole, row.Permissions
			return err
		},
		PermissionGrantedJSON: parentGuardianPermissionJSON,
		MissingProfileError: func() error {
			return parentportal.WithGuardianProfileMissingCause(peopledirectory.ErrGuardianProfileNotFound)
		},
	})
}

// parentGuardianPermissionJSON reads a stored permissions object with
// Security Runtime's fail-closed rule.
var parentGuardianPermissionJSON = securityruntime.GuardianPermissionGrantedJSON
