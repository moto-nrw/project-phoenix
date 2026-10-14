package services

// The suites that compose the factory through the typed root are external
// tests: the root imports this package. These names hand them the internal
// adapters they pin.

// ProvisioningIdentityForTest is the operator provisioning's identity adapter.
type ProvisioningIdentityForTest = provisioningIdentity

// NewProvisioningIdentityForTest is the identity adapter over the role reads
// of roles.
func NewProvisioningIdentityForTest(roles provisioningRoles) ProvisioningIdentityForTest {
	return provisioningIdentity{roles: roles}
}
