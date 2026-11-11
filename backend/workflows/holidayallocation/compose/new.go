// Package compose builds the holiday allocation workflow (ADR 0206) and binds
// it to the joining Träger transaction of Tenant Runtime.
package compose

import (
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/holidayallocation"
)

// New builds the workflow over the owner commands the root binds.
func New(billing holidayallocation.Billing, periods holidayallocation.Periods, places holidayallocation.Places, allocation holidayallocation.Allocation) *holidayallocation.Workflow {
	return holidayallocation.New(holidayallocation.Ports{Billing: billing, Periods: periods, Places: places, Allocation: allocation, Within: tenant.WithinTraeger})
}
