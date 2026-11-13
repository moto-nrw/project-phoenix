package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	organizationCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
)

var _ organizationCompose.ProvisioningIdentity = owners.ProvisioningIdentity{}

// --- the provisioning capability's identity error contract ------------------
