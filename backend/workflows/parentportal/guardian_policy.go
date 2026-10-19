package parentportal

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type GuardianRolePolicy interface {
	IsFullRole(string) bool
	PermissionGranted(any) bool
	DefaultRole(string, bool, bool, bool) (string, []string)
}
type GuardianPolicyCallbacks struct {
	RelationshipTypeValid func(string) bool
	NormalizeRelationship func(*GuardianRelationship) error
	PermissionGrantedJSON func(json.RawMessage, string) bool
	MissingProfileError   func() error
}
type GuardianPolicy interface{ care.GuardianPolicy }

func NewGuardianPolicy(roles GuardianRolePolicy, callbacks GuardianPolicyCallbacks) GuardianPolicy {
	return guardianPolicy{roles, callbacks}
}

type guardianPolicy struct {
	roles     GuardianRolePolicy
	callbacks GuardianPolicyCallbacks
}

func (p guardianPolicy) IsFullRole(value string) bool     { return p.roles.IsFullRole(value) }
func (p guardianPolicy) PermissionGranted(value any) bool { return p.roles.PermissionGranted(value) }
func (p guardianPolicy) DefaultRole(relationship string, primary, emergency, pickup bool) (string, []string) {
	return p.roles.DefaultRole(relationship, primary, emergency, pickup)
}
func (p guardianPolicy) RelationshipTypeValid(value string) bool {
	return p.callbacks.RelationshipTypeValid(value)
}
func (p guardianPolicy) NormalizeRelationship(value *care.GuardianRelationship) error {
	return p.callbacks.NormalizeRelationship((*GuardianRelationship)(value))
}
func (p guardianPolicy) PermissionGrantedJSON(value json.RawMessage, permission string) bool {
	return p.callbacks.PermissionGrantedJSON(value, permission)
}
func (p guardianPolicy) MissingProfileError() error { return p.callbacks.MissingProfileError() }
