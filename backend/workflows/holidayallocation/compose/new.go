// Package compose binds the holiday allocation workflow to its Träger unit.
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/holidayallocation"
)

// New wires the owner commands into one joining Träger transaction.
func New(allocation enrollment.HolidayAllocation, periods holidayallocation.Periods, billing holidayallocation.Billing) *holidayallocation.Workflow {
	return holidayallocation.New(allocation, periods, billing, tenant.WithinTraeger)
}
