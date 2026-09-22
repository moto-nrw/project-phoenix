package presence

import "context"

type AttendanceStaff interface {
	LockStaffExists(context.Context, int64) (bool, error)
	ExistingStaffIDs(context.Context, []int64) (map[int64]struct{}, error)
	StaffTenantID(context.Context, int64) (*int64, error)
}
