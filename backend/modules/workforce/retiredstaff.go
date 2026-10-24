package workforce

import (
	"context"
	"time"
)

// RetiredStaffRecords erases the Workforce records of a staff member who
// left before the cutoff (#3963): time tracking, shifts, substitutions,
// balances, vacation and allowance accounts, work schedules, master,
// financial and employment data, qualifications and documents. Document
// files go through the staff document file cleanup. It runs on the tenant
// transaction of the school in context.
type RetiredStaffRecords interface {
	// EraseRetiredStaffRecords reports the deleted rows per record kind. A
	// staff member who has not left before retiredBefore is refused.
	EraseRetiredStaffRecords(ctx context.Context, staffID int64, retiredBefore time.Time) (map[string]int64, error)
}
