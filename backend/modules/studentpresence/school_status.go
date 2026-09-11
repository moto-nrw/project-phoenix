package studentpresence

import (
	"context"
	"time"
)

type SchoolStatus struct {
	StudentID                            int64
	CheckedInBy                          int64
	CheckedOutBy                         *int64
	Date, Status                         string
	CheckInTime, CheckOutTime, YardSince *time.Time
}

// ListSchoolStatuses returns one status per requested student. Checkout wins
// over yard presence; a student without a stay has not_checked_in status.
func (m *Module) ListSchoolStatuses(ctx context.Context, ids []int64, date string) ([]SchoolStatus, error) {
	return m.engine.ListSchoolStatuses(ctx, ids, date)
}
