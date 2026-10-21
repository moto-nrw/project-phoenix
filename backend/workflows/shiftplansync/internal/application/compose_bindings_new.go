package application

import (
	context "context"
	slices "slices"

	workforce "github.com/moto-nrw/project-phoenix/modules/workforce"
)

// missingNames lists the absent dependencies in a stable order.
func MissingNames(present map[string]bool) []string {
	missing := make([]string, 0, len(present))
	for name, ok := range present {
		if !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

// Shifts binds the workflow's Dienstplan port to the Workforce owner: the
// cancellation is promoted from its planning capability, the listings and the
// sick stamp from its shift row query and command, and the write lock is the
// per-staff advisory lock the module's own writes take.
type Shifts struct {
	workforce.StaffShiftPlanning
	workforce.ShiftQuery
	workforce.StaffShiftCommand
	Lock func(context.Context, int64) error
}

func (s Shifts) LockStaffShifts(ctx context.Context, staffID int64) error {
	return s.Lock(ctx, staffID)
}
