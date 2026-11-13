package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	carePlanPorts "github.com/moto-ogs/project-phoenix/modules/careplan/ports"

	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
)

func NewPickupApprovalExceptions(records carePlanPorts.PickupApprovalRecords) (ports.PickupApprovalExceptions, error) {
	if records == nil {
		return nil, errors.New("pickup approval exceptions: records are required")
	}
	return application.NewPickupApprovalExceptions(records, postgres.IsPickupExceptionConflict), nil
}
