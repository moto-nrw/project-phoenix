package api

// The operator provisioning suites pin the identity adapter the root binds
// provisioning over (#3253).

// ProvisioningIdentityForTest is the operator provisioning's identity adapter.
type ProvisioningIdentityForTest = provisioningIdentity

// NewProvisioningIdentityForTest is the identity adapter over the role reads
// of roles.
func NewProvisioningIdentityForTest(roles provisioningRoles) ProvisioningIdentityForTest {
	return provisioningIdentity{roles: roles}
}
