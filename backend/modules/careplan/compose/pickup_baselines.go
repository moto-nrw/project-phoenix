package compose

import (
	"context"
	"errors"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
)

func NewPickupBaselines(records carePlanPorts.PickupBaselineRecords, links careplan.ApprovedBookingReader, authoritative func(context.Context) (bool, error)) (careplan.PickupBaselineReader, error) {
	if records == nil || links == nil || authoritative == nil {
		return nil, errors.New("pickup baselines: records, approved bookings, and booking mode are required")
	}
	return application.NewPickupBaselines(records, links, authoritative), nil
}
