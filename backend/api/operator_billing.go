package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
)

var _ organizationCompose.BillingCounts = owners.BillingCounts{}
