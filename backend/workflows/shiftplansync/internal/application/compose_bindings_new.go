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

// shifts binds the workflow's Dienstplan port to the Workforce owner: the
// cancellation to its planning capability, the listings and the sick stamp to
// its row capability, and the write lock to the per-staff advisory lock the
// module's own writes take.
type Shifts struct {
	Planning workforce.StaffShiftPlanning
	Rows     workforce.Capability
	Lock     func(context.Context, int64) error
}

func (s Shifts) ListStaffShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]workforce.StaffShift, error) {
	return s.Rows.ListStaffShifts(ctx, filter)
}
func (s Shifts) ApplyCancellation(ctx context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
	return s.Planning.ApplyCancellation(ctx, input)
}
func (s Shifts) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error) {
	return s.Rows.SetStaffShiftSickAbsence(ctx, shiftID, absenceID)
}
func (s Shifts) LockStaffShifts(ctx context.Context, staffID int64) error {
	return s.Lock(ctx, staffID)
}
