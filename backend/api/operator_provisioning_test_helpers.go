//go:build testhooks

package api

import owners "github.com/moto-nrw/project-phoenix/roots/owners"

// The operator provisioning suites pin the identity adapter the root binds
// provisioning over (#3253).

// ProvisioningIdentityForTest is the operator provisioning's identity adapter.
type ProvisioningIdentityForTest = owners.ProvisioningIdentity

// NewProvisioningIdentityForTest is the identity adapter over the role reads
// of roles.
func NewProvisioningIdentityForTest(roles owners.ProvisioningRoles) ProvisioningIdentityForTest {
	return owners.ProvisioningIdentity{Roles: roles}
}
