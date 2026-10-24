package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// staffLookup is the caller's staff profile, as every request decision port
// reads it.
type staffLookup interface {
	CurrentStaffID(context.Context) (staffID int64, found bool, err error)
}

// actingStaffID is the staff member a decision writes plans in the name of.
// Only staff decide parent requests: a caller without a staff profile is
// refused, whatever its permissions say.
func actingStaffID(ctx context.Context, staff staffLookup, purpose string) (int64, error) {
	staffID, found, err := staff.CurrentStaffID(ctx)
	if err != nil {
		return 0, fmt.Errorf("schedule: resolve acting staff%s: %w", purpose, err)
	}
	if !found {
		return 0, carerequests.ErrCareRequestForbidden
	}
	return staffID, nil
}

// pickupRequestStaff is the purpose suffix of a pickup request decision.
const pickupRequestStaff = " for pickup request"
