package legacy_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// newPickupBaselineService binds native records in the fixture's legacy booking mode.
func newPickupBaselineService(records compose.PickupBaselineRecords, links careplan.ApprovedBookingReader) careplan.PickupBaselineReader {
	baselines, err := compose.NewPickupBaselines(records, links, func(context.Context) (bool, error) { return false, nil })
	if err != nil {
		panic(err)
	}
	return baselines
}
